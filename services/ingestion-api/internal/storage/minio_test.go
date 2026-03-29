package storage

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestMinioStorage(t *testing.T) {
	ctx := context.Background()

	// 1. Start ephemeral MinIO container
	minioContainer, err := tcminio.Run(ctx,
		"minio/minio:latest",
		tcminio.WithUsername("minioadmin"),
		tcminio.WithPassword("minioadmin"),
	)
	if err != nil {
		t.Fatalf("failed to start minio container: %v", err)
	}

	defer func() {
		if err := minioContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate minio container: %v", err)
		}
	}()

	endpoint, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get minio endpoint: %v", err)
	}

	// We need a raw client first to bootstrap the bucket,
	// because our Adapter's NewMinioClient now waits for the bucket to exist.
	bootstrapClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to create bootstrap client: %v", err)
	}

	err = bootstrapClient.MakeBucket(ctx, "bronze", minio.MakeBucketOptions{})
	if err != nil {
		t.Fatalf("failed to create bootstrap bucket: %v", err)
	}

	// 2. Now initialize our actual Adapter (this will now succeed immediately)
	client, err := NewMinioClient(ctx, endpoint, "minioadmin", "minioadmin", false)
	if err != nil {
		t.Fatalf("failed to initialize minio client: %v", err)
	}

	// 3. TEST: Upload JSON
	testPayload := `{"message": "Hello Testcontainers!"}`
	err = client.UploadJSON(ctx, "bronze", "test-doc.json", testPayload)
	if err != nil {
		t.Errorf("expected no error uploading json, got %v", err)
	}

	// 4. Verify object physically exists
	_, err = client.Client.StatObject(ctx, "bronze", "test-doc.json", minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("expected uploaded object to exist in bucket, got error: %v", err)
	}
}
