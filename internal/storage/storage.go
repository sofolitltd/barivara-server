package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type Client struct {
	presign *s3.PresignClient
	bucket  string
	baseURL string
}

// New builds an S3 client against Neon Object Storage, reading the
// standard AWS_* env vars (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY,
// AWS_ENDPOINT_URL_S3, AWS_REGION) injected by `neon env pull`.
func New(ctx context.Context, bucket, endpoint string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // required by Neon Object Storage
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
	})

	return &Client{
		presign: s3.NewPresignClient(s3Client),
		bucket:  bucket,
		baseURL: endpoint,
	}, nil
}

// PresignUpload returns a short-lived PUT URL plus the object's
// permanent, publicly-loadable URL (the bucket is `public_read`, so
// anonymous GET needs no signature — only the write does). Callers store
// the permanent URL directly, same as Cloudinary's old `secure_url`.
func (c *Client) PresignUpload(ctx context.Context, folder, contentType string) (uploadURL, publicURL string, err error) {
	key := fmt.Sprintf("%s/%s", folder, uuid.NewString())

	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      &c.bucket,
		Key:         &key,
		ContentType: &contentType,
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", "", fmt.Errorf("presign upload: %w", err)
	}

	return req.URL, c.ObjectURL(key), nil
}

// ObjectURL builds the permanent, path-style public URL for a stored key.
// Only valid for a `public_read` bucket (anonymous GET, no signature).
func (c *Client) ObjectURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", c.baseURL, c.bucket, key)
}
