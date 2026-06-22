package handlers

import (
	"database/sql"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/diegoGarciaCo/raffles/internal/realtime"
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

	// Hub is the in-memory live-draw WebSocket hub. Set in main after New().
	Hub *realtime.Hub
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
	}
}
