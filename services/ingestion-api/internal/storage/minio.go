package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel/propagation"
)

type MinioClient struct {
	Client *minio.Client
}

func NewMinioClient(ctx context.Context, endpoint, accessKey, secretKey string, useSSL bool) (*MinioClient, error) {
	operation := func() (*minio.Client, error) {
		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			return nil, err
		}

		// minio.New doesn't make a network call. We use ListBuckets as a ping.
		cancelCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if _, err = client.ListBuckets(cancelCtx); err != nil {
			return nil, err
		}
		return client, nil
	}

	notify := func(err error, d time.Duration) {
		slog.Warn("MinIO not ready, retrying...", "error", err, "backoff", d)
	}

	client, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(30*time.Second),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to minio after retries: %w", err)
	}

	return &MinioClient{Client: client}, nil
}

// UploadJSON uploads a string to MinIO and injects the OTel Trace-ID into the object's metadata.
func (m *MinioClient) UploadJSON(ctx context.Context, bucketName, objectName, jsonData string) error {
	metadata := make(map[string]string)

	// MAGIC: Extract the Trace-ID from the Go context and inject it into the metadata map
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, propagation.MapCarrier(metadata))

	opts := minio.PutObjectOptions{
		ContentType:  "application/json",
		UserMetadata: metadata, // MinIO persists this and includes it in NATS events!
	}

	reader := strings.NewReader(jsonData)
	_, err := m.Client.PutObject(ctx, bucketName, objectName, reader, int64(len(jsonData)), opts)
	return err
}
