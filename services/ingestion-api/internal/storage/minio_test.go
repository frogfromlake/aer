package storage

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
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

	// Ensure cleanup
	defer func() {
		if err := minioContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate minio container: %v", err)
		}
	}()

	endpoint, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get minio endpoint: %v", err)
	}

	// 2. Initialize our Adapter
	client, err := NewMinioClient(endpoint, "minioadmin", "minioadmin", false)
	if err != nil {
		t.Fatalf("failed to initialize minio client: %v", err)
	}

	// 3. Prepare Test Bucket
	err = client.Client.MakeBucket(ctx, "bronze", minio.MakeBucketOptions{})
	if err != nil {
		t.Fatalf("failed to create test bucket: %v", err)
	}

	// 4. TEST: Upload JSON
	testPayload := `{"message": "Hello Testcontainers!"}`
	err = client.UploadJSON(ctx, "bronze", "test-doc.json", testPayload)
	if err != nil {
		t.Errorf("expected no error uploading json, got %v", err)
	}

	// 5. Verify object physically exists in the container
	_, err = client.Client.StatObject(ctx, "bronze", "test-doc.json", minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("expected uploaded object to exist in bucket, got error: %v", err)
	}
}
