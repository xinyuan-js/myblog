package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xinyuan-js/myblog/apps/api/internal/config"
)

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type MinIO struct {
	client *minio.Client
	bucket string
}

func NewMinIO(cfg config.Config) (*MinIO, error) {
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &MinIO{client: client, bucket: cfg.MinIOBucket}, nil
}

func (m *MinIO) EnsureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		if err := m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create minio bucket: %w", err)
		}
	}
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, m.bucket)
	if err := m.client.SetBucketPolicy(ctx, m.bucket, policy); err != nil {
		return fmt.Errorf("set minio read-only bucket policy: %w", err)
	}
	return nil
}

func (m *MinIO) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:        contentType,
		ContentDisposition: "inline",
	})
	if err != nil {
		return fmt.Errorf("put minio object: %w", err)
	}
	return nil
}

func (m *MinIO) Delete(ctx context.Context, key string) error {
	if err := m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete minio object: %w", err)
	}
	return nil
}

func (m *MinIO) Ready(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("check minio readiness: %w", err)
	}
	if !exists {
		return fmt.Errorf("minio bucket %s does not exist", m.bucket)
	}
	return nil
}
