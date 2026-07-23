package storage

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

type fakeMinIOClient struct {
	bucketExists func(context.Context, string) (bool, error)
	putObject    func(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (minio.UploadInfo, error)
	removeObject func(context.Context, string, string, minio.RemoveObjectOptions) error
}

func (f fakeMinIOClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return f.bucketExists(ctx, bucket)
}

func (f fakeMinIOClient) PutObject(
	ctx context.Context,
	bucket, key string,
	body io.Reader,
	size int64,
	options minio.PutObjectOptions,
) (minio.UploadInfo, error) {
	return f.putObject(ctx, bucket, key, body, size, options)
}

func (f fakeMinIOClient) RemoveObject(
	ctx context.Context,
	bucket, key string,
	options minio.RemoveObjectOptions,
) error {
	return f.removeObject(ctx, bucket, key, options)
}

func TestMinIOTransportDoesNotUseAmbientProxyAndBoundsConnections(t *testing.T) {
	transport, err := newMinIOTransport(false)
	if err != nil {
		t.Fatal(err)
	}
	if transport.Proxy != nil {
		t.Fatal("MinIO transport must not use environment proxy settings")
	}
	if transport.MaxConnsPerHost != 8 || transport.MaxIdleConnsPerHost != 4 {
		t.Fatalf("unexpected MinIO connection limits: %+v", transport)
	}
	if transport.MaxResponseHeaderBytes != 64<<10 {
		t.Fatalf("MaxResponseHeaderBytes = %d", transport.MaxResponseHeaderBytes)
	}
	if transport.ResponseHeaderTimeout != 15*time.Second ||
		transport.TLSHandshakeTimeout != 5*time.Second ||
		transport.ExpectContinueTimeout != 2*time.Second {
		t.Fatalf("unexpected MinIO transport timeouts: %+v", transport)
	}
}

func TestMinIOPutUsesSingleObjectAndOperationDeadline(t *testing.T) {
	called := false
	store := &MinIO{
		bucket: "blog-media",
		client: fakeMinIOClient{
			putObject: func(
				ctx context.Context,
				bucket, key string,
				body io.Reader,
				size int64,
				options minio.PutObjectOptions,
			) (minio.UploadInfo, error) {
				called = true
				assertDeadline(t, ctx, minioPutTimeout)
				if bucket != "blog-media" || key != "2026/07/image.webp" || size != 5 {
					t.Fatalf("unexpected PutObject target: bucket=%q key=%q size=%d", bucket, key, size)
				}
				if !options.DisableMultipart || options.ContentType != "image/webp" || options.ContentDisposition != "inline" {
					t.Fatalf("unsafe PutObject options: %+v", options)
				}
				value, err := io.ReadAll(body)
				if err != nil || string(value) != "image" {
					t.Fatalf("body=%q err=%v", value, err)
				}
				return minio.UploadInfo{}, nil
			},
		},
	}

	if err := store.Put(context.Background(), "2026/07/image.webp", strings.NewReader("image"), 5, "image/webp"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("PutObject was not called")
	}
}

func TestMinIOBucketAndDeleteOperationsHaveDeadlines(t *testing.T) {
	bucketChecks := 0
	store := &MinIO{
		bucket: "blog-media",
		client: fakeMinIOClient{
			bucketExists: func(ctx context.Context, bucket string) (bool, error) {
				bucketChecks++
				assertDeadline(t, ctx, minioBucketTimeout)
				if bucket != "blog-media" {
					t.Fatalf("bucket = %q", bucket)
				}
				return true, nil
			},
			removeObject: func(ctx context.Context, bucket, key string, _ minio.RemoveObjectOptions) error {
				assertDeadline(t, ctx, minioDeleteTimeout)
				if bucket != "blog-media" || key != "2026/07/image.webp" {
					t.Fatalf("unexpected RemoveObject target: bucket=%q key=%q", bucket, key)
				}
				return nil
			},
		},
	}

	if err := store.EnsureBucket(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "2026/07/image.webp"); err != nil {
		t.Fatal(err)
	}
	if bucketChecks != 2 {
		t.Fatalf("bucket checks = %d, want 2", bucketChecks)
	}
}

func assertDeadline(t *testing.T, ctx context.Context, maximum time.Duration) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("operation context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > maximum {
		t.Fatalf("deadline remaining = %s, want within (0, %s]", remaining, maximum)
	}
}
