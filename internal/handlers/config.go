package handlers

import (
	"database/sql"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/diegoGarciaCo/raffles/internal/charity"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/diegoGarciaCo/raffles/internal/payments"
	"github.com/diegoGarciaCo/raffles/internal/realtime"
	"github.com/diegoGarciaCo/raffles/internal/storage"
)

type apiCfg struct {
	Port             string
	JWTSecret        string
	DB               *database.Queries
	RawDB            *sql.DB
	dev              bool
	S3Client         *s3.Client
	S3Bucket         string
	S3Region         string
	betterAuthSecret string

	// Storage issues presigned S3 upload URLs. Built in New() from the S3
	// client + bucket/region that main.go already passes in.
	Storage *storage.Uploader

	// Hub is the in-memory live-draw WebSocket hub. Set in main after New().
	Hub *realtime.Hub

	// Stripe. Set in main after New().
	Payments             *payments.Client
	StripePublishableKey string

	// CharityAPI (IRS EIN verification). Set in main after New().
	Charity *charity.Client
}

func New(port, JWTSecret string, db *database.Queries, dbSQL *sql.DB, dev bool, s3Client *s3.Client, s3Bucket string, s3Region string, betterAuthSecret string) *apiCfg {
	return &apiCfg{
		Port:             port,
		JWTSecret:        JWTSecret,
		DB:               db,
		RawDB:            dbSQL,
		dev:              dev,
		S3Client:         s3Client,
		S3Bucket:         s3Bucket,
		S3Region:         s3Region,
		betterAuthSecret: betterAuthSecret,
		Storage:          storage.New(s3Client, s3Bucket, s3Region),
	}
}
