package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/example/myblog/apps/api/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type minioClient interface {
	BucketExists(context.Context, string) (bool, error)
	PutObject(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (minio.UploadInfo, error)
	RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error
}

type MinIO struct {
	client minioClient
	bucket string
}

const (
	minioBucketTimeout = 5 * time.Second
	minioPutTimeout    = 30 * time.Second
	minioDeleteTimeout = 10 * time.Second
)

func NewMinIO(cfg config.Config) (*MinIO, error) {
	transport, err := newMinIOTransport(cfg.MinIOUseSSL)
	if err != nil {
		return nil, fmt.Errorf("create minio transport: %w", err)
	}
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure:    cfg.MinIOUseSSL,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &MinIO{client: client, bucket: cfg.MinIOBucket}, nil
}

func newMinIOTransport(secure bool) (*http.Transport, error) {
	transport, err := minio.DefaultTransport(secure)
	if err != nil {
		return nil, err
	}
	// MinIO is an explicitly configured private dependency. Never send its
	// signed requests or application credentials through ambient proxy
	// variables inherited from a host or container runtime.
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.MaxIdleConns = 16
	transport.MaxIdleConnsPerHost = 4
	transport.MaxConnsPerHost = 8
	transport.MaxResponseHeaderBytes = 64 << 10
	transport.ResponseHeaderTimeout = 15 * time.Second
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ExpectContinueTimeout = 2 * time.Second
	transport.IdleConnTimeout = time.Minute
	return transport, nil
}

func (m *MinIO) EnsureBucket(ctx context.Context) error {
	operationContext, cancel := context.WithTimeout(ctx, minioBucketTimeout)
	defer cancel()
	exists, err := m.client.BucketExists(operationContext, m.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("required minio bucket %s does not exist", m.bucket)
	}
	return nil
}

func (m *MinIO) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	operationContext, cancel := context.WithTimeout(ctx, minioPutTimeout)
	defer cancel()
	_, err := m.client.PutObject(operationContext, m.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:        contentType,
		ContentDisposition: "inline",
		// The application accepts at most 10 MiB, so a single PUT is both
		// sufficient and safer: interrupted uploads cannot leave multipart
		// fragments that are invisible to the media database and its cleanup.
		DisableMultipart: true,
	})
	if err != nil {
		return fmt.Errorf("put minio object: %w", err)
	}
	return nil
}

func (m *MinIO) Delete(ctx context.Context, key string) error {
	operationContext, cancel := context.WithTimeout(ctx, minioDeleteTimeout)
	defer cancel()
	if err := m.client.RemoveObject(operationContext, m.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete minio object: %w", err)
	}
	return nil
}

func (m *MinIO) Ready(ctx context.Context) error {
	operationContext, cancel := context.WithTimeout(ctx, minioBucketTimeout)
	defer cancel()
	exists, err := m.client.BucketExists(operationContext, m.bucket)
	if err != nil {
		return fmt.Errorf("check minio readiness: %w", err)
	}
	if !exists {
		return fmt.Errorf("minio bucket %s does not exist", m.bucket)
	}
	return nil
}
