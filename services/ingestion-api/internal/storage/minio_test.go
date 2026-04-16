package storage

import (
	"context"
	"testing"
	"time"

	"github.com/frogfromlake/aer/pkg/testutils"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestMinioStorage(t *testing.T) {
	ctx := context.Background()

	minioImage, err := testutils.GetImageFromCompose("minio")
	if err != nil {
		t.Fatalf("failed to get minio image from compose: %v", err)
	}

	// 1. Start ephemeral MinIO container
	minioContainer, err := tcminio.Run(ctx,
		minioImage,
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
	client, err := NewMinioClient(ctx, endpoint, "minioadmin", "minioadmin", false, "bronze")
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

func TestNewMinioClientChecksConfiguredBucket(t *testing.T) {
	ctx := context.Background()

	minioImage, err := testutils.GetImageFromCompose("minio")
	if err != nil {
		t.Fatalf("failed to get minio image from compose: %v", err)
	}

	minioContainer, err := tcminio.Run(ctx,
		minioImage,
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

	bootstrapClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to create bootstrap client: %v", err)
	}

	// Create a custom-named bucket (not "bronze") to prove the parameter is used
	err = bootstrapClient.MakeBucket(ctx, "custom-bronze", minio.MakeBucketOptions{})
	if err != nil {
		t.Fatalf("failed to create custom bucket: %v", err)
	}

	// Should succeed when pointing at the bucket that exists
	client, err := NewMinioClient(ctx, endpoint, "minioadmin", "minioadmin", false, "custom-bronze")
	if err != nil {
		t.Fatalf("expected success with custom bucket name, got: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Should fail when pointing at a bucket that does NOT exist.
	// Use a short context to avoid the full 30s retry loop.
	shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = NewMinioClient(shortCtx, endpoint, "minioadmin", "minioadmin", false, "nonexistent-bucket")
	if err == nil {
		t.Fatal("expected error when configured bucket does not exist, got nil")
	}
}
