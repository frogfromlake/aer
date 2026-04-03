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
	OTelEndpoint   string `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	MigrationsPath string `mapstructure:"MIGRATIONS_PATH"`
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
	v.SetDefault("MIGRATIONS_PATH", "/migrations")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig() // Ignore missing .env in production

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
