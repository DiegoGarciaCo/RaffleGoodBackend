package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/diegoGarciaCo/raffles/internal/charity"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/diegoGarciaCo/raffles/internal/draw"
	"github.com/diegoGarciaCo/raffles/internal/handlers"
	"github.com/diegoGarciaCo/raffles/internal/payments"
	"github.com/diegoGarciaCo/raffles/internal/realtime"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

func main() {
	// ── Env ──────────────────────────────────────────────────────────────────
	if err := godotenv.Load(); err != nil {
		logrus.Warn("no .env file found, relying on environment variables")
	}

	cfg := mustLoadEnv()

	// ── Database ───────────────────────────────────────────────────────────────
	db, err := sql.Open("postgres", cfg.dbURL)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	dbQueries := database.New(db)

	// ── S3 ───────────────────────────────────────────────────────────────────
	awsCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(cfg.s3Region))
	if err != nil {
		log.Fatalf("loading aws config: %v", err)
	}
	s3Client := s3.NewFromConfig(awsCfg)

	// ── API config ─────────────────────────────────────────────────────────────
	api := handlers.New(
		cfg.port, cfg.jwtSecret, dbQueries, db, cfg.dev,
		s3Client, cfg.s3Bucket, cfg.s3Region, cfg.betterAuthSecret,
	)

	// ── Stripe ───────────────────────────────────────────────────────────────
	api.Payments = payments.NewClient(cfg.stripeSecret, cfg.stripeWebhookSecret)
	api.StripePublishableKey = cfg.stripePublishable

	// ── CharityAPI (IRS EIN verification) ──────────────────────────────────────
	api.Charity = charity.New(cfg.charityAPIKey)

	// ── Routes + middleware ────────────────────────────────────────────────────
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	// Recovery outermost, then CORS, then logging (innermost).
	handler := handlers.Chain(
		mux,
		api.RecoveryMiddleware,
		api.CORSMiddleware,
		api.LoggerMiddleware,
	)

	srv := &http.Server{
		Addr:         ":" + cfg.port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Realtime hub + draw worker ─────────────────────────────────────────────
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	hub := realtime.NewHub()
	api.Hub = hub

	worker := draw.NewWorker(dbQueries, db, hub, 30*time.Second)
	go worker.Run(rootCtx)

	// ── Start + graceful shutdown ────────────────────────────────────────────
	go func() {
		logrus.Infof("listening on port %s", cfg.port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Fatal("server failed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logrus.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("graceful shutdown failed")
	}
	logrus.Info("stopped")
}

// ── Env loading ──────────────────────────────────────────────────────────────

type envConfig struct {
	port                string
	jwtSecret           string
	betterAuthSecret    string
	s3Bucket            string
	s3Region            string
	dbURL               string
	dev                 bool
	stripeSecret        string
	stripeWebhookSecret string
	stripePublishable   string
	charityAPIKey       string
}

func mustLoadEnv() envConfig {
	return envConfig{
		port:                mustEnv("PORT"),
		jwtSecret:           mustEnv("JWT_SECRET"),
		betterAuthSecret:    mustEnv("BETTER_AUTH_SECRET"),
		s3Bucket:            mustEnv("S3_BUCKET"),
		s3Region:            mustEnv("S3_REGION"),
		dbURL:               mustEnv("DATABASE_URL"),
		dev:                 os.Getenv("DEV") == "true",
		stripeSecret:        mustEnv("STRIPE_SECRET_KEY"),
		stripeWebhookSecret: mustEnv("STRIPE_WEBHOOK_SECRET"),
		stripePublishable:   mustEnv("STRIPE_PUBLISHABLE_KEY"),
		charityAPIKey:       mustEnv("CHARITY_API_KEY"),
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is not set", key)
	}
	return v
}
