package storage

// S3 uploader. Keeps all S3 presigning behind a small surface so handlers don't
// touch the AWS SDK directly.
//
// Images are uploaded DIRECTLY from the mobile client to S3 using a presigned
// PUT URL — the file bytes never pass through the Go server. The server only
// signs a short-lived URL and hands back the public read URL to store.

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Uploader struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
	region  string
}

// New builds an Uploader from the already-configured S3 client. bucket/region
// are the same values main.go loads from the environment.
func New(client *s3.Client, bucket, region string) *Uploader {
	return &Uploader{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
		region:  region,
	}
}

// PresignPut returns a short-lived URL the client PUTs the raw file bytes to.
//
// We deliberately DO NOT sign the Content-Type. If it were part of the
// signature, the client's PUT header would have to byte-match it exactly or S3
// returns 403 — a brittle, hard-to-debug failure across RN upload methods. By
// leaving it unsigned, the client can send whatever Content-Type it likes and
// S3 simply records it on the object; there is nothing to mismatch. The key is
// random and single-use with a short TTL, so the looser URL is not a concern.
//
// contentType is accepted for symmetry with the handler (which validates it to
// derive the file extension) but is intentionally not bound into the signature.
func (u *Uploader) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	_ = contentType
	req, err := u.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &u.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign put object: %w", err)
	}
	return req.URL, nil
}

// PublicURL is the canonical virtual-hosted–style read URL for an object. For
// the image to actually load in the app, the bucket (or the uploads/ prefix)
// must permit public s3:GetObject via a bucket policy (see the setup notes).
func (u *Uploader) PublicURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", u.bucket, u.region, key)
}
