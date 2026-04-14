package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the environment variables required for the Ingestion API.
type Config struct {
	Environment    string `mapstructure:"APP_ENV"`
	LogLevel       string `mapstructure:"LOG_LEVEL"`
	IngestionPort  string `mapstructure:"INGESTION_PORT"`
	DBUrl          string `mapstructure:"DB_URL"`
	MinioEndpoint  string `mapstructure:"MINIO_ENDPOINT"`
	MinioAccessKey string `mapstructure:"MINIO_ACCESS_KEY"`
	MinioSecretKey string `mapstructure:"MINIO_SECRET_KEY"`
	MinioUseSSL    bool   `mapstructure:"MINIO_USE_SSL"`
	OTelEndpoint      string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTelSampleRate    float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
	MigrationsPath string `mapstructure:"MIGRATIONS_PATH"`
	APIKey         string `mapstructure:"INGESTION_API_KEY"`
	BronzeBucket   string `mapstructure:"INGESTION_BRONZE_BUCKET"`
	// MaxBodyBytes caps the size of the JSON body accepted by POST /api/v1/ingest.
	// A request larger than this is rejected with 413 before the decoder runs,
	// so a malicious or buggy client cannot drive the process into OOM.
	MaxBodyBytes int64 `mapstructure:"INGESTION_MAX_BODY_BYTES"`
	// ShutdownTimeoutSeconds is the grace period the HTTP server has to drain
	// in-flight requests before it is forced down. Must exceed WriteTimeout so
	// a request that is mid-flight at signal time can complete.
	ShutdownTimeoutSeconds int `mapstructure:"INGESTION_SHUTDOWN_TIMEOUT_SECONDS"`
}

// Load reads configuration from environment variables and the local .env file.
func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "INFO")
	v.SetDefault("INGESTION_PORT", "8081")
	v.SetDefault("DB_URL", "")
	v.SetDefault("MINIO_ENDPOINT", "")
	v.SetDefault("MINIO_ACCESS_KEY", "")
	v.SetDefault("MINIO_SECRET_KEY", "")
	v.SetDefault("MINIO_USE_SSL", false)
	v.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	v.SetDefault("OTEL_TRACE_SAMPLE_RATE", 1.0)
	v.SetDefault("MIGRATIONS_PATH", "/migrations")
	v.SetDefault("INGESTION_API_KEY", "")
	v.SetDefault("INGESTION_BRONZE_BUCKET", "bronze")
	v.SetDefault("INGESTION_MAX_BODY_BYTES", int64(16<<20)) // 16 MiB
	v.SetDefault("INGESTION_SHUTDOWN_TIMEOUT_SECONDS", 30)

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig() // Ignore missing .env in production

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("INGESTION_API_KEY must be set")
	}
	if cfg.DBUrl == "" {
		return nil, fmt.Errorf("DB_URL must be set (contains POSTGRES_PASSWORD)")
	}

	return &cfg, nil
}
