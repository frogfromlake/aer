package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel/propagation"
)

// MinioClient is a wrapper around the MinIO SDK client.
type MinioClient struct {
	Client *minio.Client
}

// NewMinioClient initializes a new connection to the S3-compatible Data Lake.
func NewMinioClient(endpoint, accessKey, secretKey string, useSSL bool) (*MinioClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize minio client: %w", err)
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