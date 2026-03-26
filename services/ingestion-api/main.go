package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	fmt.Println("Starting AĒR Ingestion API...")

	// 1. Connect to PostgreSQL
	connStr := "postgres://aer_user:aer_password@localhost:5432/aer_metadata?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Postgres Init Error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Postgres Ping Error: %v", err)
	}
	fmt.Println("✅ PostgreSQL Connected")

	// 2. Connect to MinIO
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("aer_admin", "aer_password_123", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("❌ MinIO Init Error: %v", err)
	}

	// 3. Auto-Provision Buckets for our Medallion Architecture
	// We create 'bronze' for raw data and 'silver' for cleaned data
	buckets := []string{"bronze", "silver"}
	ctx := context.Background()

	for _, bucket := range buckets {
		exists, err := minioClient.BucketExists(ctx, bucket)
		if err != nil {
			log.Fatalf("❌ Error checking bucket %s: %v", bucket, err)
		}
		if !exists {
			fmt.Printf("📦 Creating bucket: %s...\n", bucket)
			err = minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
			if err != nil {
				log.Fatalf("❌ Failed to create bucket %s: %v", bucket, err)
			}
		} else {
			fmt.Printf("✅ Bucket '%s' already exists\n", bucket)
		}
	}

	fmt.Println("\n🚀 Data Lake is provisioned and ready!")
}
