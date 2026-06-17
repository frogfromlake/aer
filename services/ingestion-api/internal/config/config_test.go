package config

import (
	"strings"
	"testing"
)

// setRequiredSecrets sets the two values Load() validates, so the happy-path
// defaults can be asserted; the validation tests unset one of them.
func setRequiredSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("INGESTION_API_KEY", "test-key")
	t.Setenv("DB_URL", "postgres://localhost/test")
}

func TestLoad_AppliesDefaults(t *testing.T) {
	setRequiredSecrets(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Environment != "development" {
		t.Errorf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.IngestionPort != "8081" {
		t.Errorf("IngestionPort = %q, want 8081", cfg.IngestionPort)
	}
	if cfg.BronzeBucket != "bronze" {
		t.Errorf("BronzeBucket = %q, want bronze", cfg.BronzeBucket)
	}
	if cfg.MaxBodyBytes != 16<<20 {
		t.Errorf("MaxBodyBytes = %d, want %d", cfg.MaxBodyBytes, 16<<20)
	}
	if cfg.ShutdownTimeoutSeconds != 65 {
		t.Errorf("ShutdownTimeoutSeconds = %d, want 65", cfg.ShutdownTimeoutSeconds)
	}
	if cfg.OTelSampleRate != 1.0 {
		t.Errorf("OTelSampleRate = %v, want 1.0", cfg.OTelSampleRate)
	}
	if cfg.MinioUploadConcurrency != 8 {
		t.Errorf("MinioUploadConcurrency = %d, want 8", cfg.MinioUploadConcurrency)
	}
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	setRequiredSecrets(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("INGESTION_PORT", "9090")
	t.Setenv("OTEL_TRACE_SAMPLE_RATE", "0.1")
	t.Setenv("MINIO_USE_SSL", "true")
	t.Setenv("INGESTION_DB_MAX_OPEN_CONNS", "50")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Environment != "production" {
		t.Errorf("Environment = %q, want production", cfg.Environment)
	}
	if cfg.IngestionPort != "9090" {
		t.Errorf("IngestionPort = %q, want 9090", cfg.IngestionPort)
	}
	if cfg.OTelSampleRate != 0.1 {
		t.Errorf("OTelSampleRate = %v, want 0.1", cfg.OTelSampleRate)
	}
	if !cfg.MinioUseSSL {
		t.Error("MinioUseSSL = false, want true")
	}
	if cfg.DBMaxOpenConns != 50 {
		t.Errorf("DBMaxOpenConns = %d, want 50", cfg.DBMaxOpenConns)
	}
}

func TestLoad_RequiresAPIKey(t *testing.T) {
	t.Setenv("INGESTION_API_KEY", "")
	t.Setenv("DB_URL", "postgres://localhost/test")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when INGESTION_API_KEY is empty")
	}
	if !strings.Contains(err.Error(), "INGESTION_API_KEY") {
		t.Errorf("error = %v, want it to mention INGESTION_API_KEY", err)
	}
}

func TestLoad_RequiresDBUrl(t *testing.T) {
	t.Setenv("INGESTION_API_KEY", "test-key")
	t.Setenv("DB_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when DB_URL is empty")
	}
	if !strings.Contains(err.Error(), "DB_URL") {
		t.Errorf("error = %v, want it to mention DB_URL", err)
	}
}
