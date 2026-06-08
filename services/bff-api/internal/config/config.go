package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the environment variables required for the BFF API.
type Config struct {
	Environment         string  `mapstructure:"APP_ENV"`
	LogLevel            string  `mapstructure:"LOG_LEVEL"`
	BFFPort             string  `mapstructure:"BFF_PORT"`
	ClickHouseHost      string  `mapstructure:"CLICKHOUSE_HOST"`
	ClickHousePort      string  `mapstructure:"CLICKHOUSE_PORT"`
	ClickHouseUser      string  `mapstructure:"CLICKHOUSE_USER"`
	ClickHousePassword  string  `mapstructure:"CLICKHOUSE_PASSWORD"`
	ClickHouseDB        string  `mapstructure:"CLICKHOUSE_DB"`
	OTelEndpoint        string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTelSampleRate      float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
	CORSOrigins         string  `mapstructure:"CORS_ALLOWED_ORIGINS"`
	APIKey              string  `mapstructure:"BFF_API_KEY"`
	RateLimitRPS        float64 `mapstructure:"RATE_LIMIT_RPS"`
	RateLimitBurst      int     `mapstructure:"RATE_LIMIT_BURST"`
	QueryRowLimit       int     `mapstructure:"BFF_QUERY_ROW_LIMIT"`
	MetricsCacheTTLSecs int     `mapstructure:"BFF_METRICS_CACHE_TTL_SECONDS"`
	// ShutdownTimeoutSeconds is the grace period the HTTP server has to drain
	// in-flight requests before it is forced down. Must exceed the chi request
	// timeout so an in-flight ClickHouse query can finish during shutdown.
	ShutdownTimeoutSeconds int `mapstructure:"BFF_SHUTDOWN_TIMEOUT_SECONDS"`
	// Postgres connection for the /sources read-only path. BFF connects as
	// a dedicated `bff_readonly` role provisioned by the postgres-init-roles
	// init container; the role only holds SELECT on the `sources` table
	// (Phase 87). Leaving BFFDBUser or BFFDBPassword empty disables the
	// /sources endpoint and is only acceptable in unit tests.
	PostgresHost        string `mapstructure:"POSTGRES_HOST"`
	PostgresPort        string `mapstructure:"POSTGRES_PORT"`
	PostgresDB          string `mapstructure:"POSTGRES_DB"`
	BFFDBUser           string `mapstructure:"BFF_DB_USER"`
	BFFDBPassword       string `mapstructure:"BFF_DB_PASSWORD"`
	SourcesCacheTTLSecs int    `mapstructure:"BFF_SOURCES_CACHE_TTL_SECONDS"`
	// ConfigDir holds the directory containing the bundled BFF config files
	// (metric_provenance.yaml, content/). The container build copies them to
	// /app/configs and runs from /app, so the default `configs` resolves
	// correctly. Host-mode runs invoke the binary from the repo root and must
	// override to `services/bff-api/configs`.
	ConfigDir string `mapstructure:"BFF_CONFIG_DIR"`
	// MinIO read-only access for the L5 Evidence article-detail endpoint
	// (Phase 101). The BFF connects via a dedicated service account
	// (BFF_MINIO_ACCESS_KEY / BFF_MINIO_SECRET_KEY) that holds GetObject
	// on `silver/*` and `bronze/*` only — provisioned by `infra/minio/setup.sh`.
	MinioEndpoint  string `mapstructure:"MINIO_ENDPOINT"`
	MinioUseSSL    bool   `mapstructure:"MINIO_USE_SSL"`
	MinioAccessKey string `mapstructure:"BFF_MINIO_ACCESS_KEY"`
	MinioSecretKey string `mapstructure:"BFF_MINIO_SECRET_KEY"`
	// KAnonymityThreshold is the minimum aggregation-group size required for
	// the article-detail endpoint to return cleaned text (WP-006 §7).
	// Below the threshold, the endpoint returns 403 with a refusal payload.
	KAnonymityThreshold int `mapstructure:"BFF_K_ANONYMITY_THRESHOLD"`

	// --- Access control (Phase 134 / ADR-040) ---
	//
	// The BFF is the confidential auth authority. Auth writes go through a
	// SECOND, small Postgres pool under the dedicated `bff_auth` role
	// (DML on the auth tables only) — the analytics read path keeps using
	// `bff_readonly`. Both must be set or the service refuses to boot.
	BFFAuthDBUser     string `mapstructure:"BFF_AUTH_DB_USER"`
	BFFAuthDBPassword string `mapstructure:"BFF_AUTH_DB_PASSWORD"`
	// SecureCookies controls the `Secure` flag and the cookie name. True (the
	// default, and the only production-safe value) yields a `__Host-`-prefixed
	// Secure cookie. Set false ONLY for local http / Testcontainers, where the
	// `__Host-` prefix and Secure-over-https would otherwise drop the cookie.
	SecureCookies bool `mapstructure:"BFF_SECURE_COOKIES"`
	// Session lifetime: a sliding idle window bounded by a hard absolute cap
	// (ADR-040 "silent stateful refresh"). Seconds.
	SessionIdleSeconds     int `mapstructure:"BFF_SESSION_IDLE_SECONDS"`
	SessionAbsoluteSeconds int `mapstructure:"BFF_SESSION_ABSOLUTE_SECONDS"`
	// argon2id parameters (OWASP defaults: m=19 MiB, t=2, p=1).
	Argon2MemoryKiB   int `mapstructure:"BFF_ARGON2_MEMORY_KIB"`
	Argon2Iterations  int `mapstructure:"BFF_ARGON2_ITERATIONS"`
	Argon2Parallelism int `mapstructure:"BFF_ARGON2_PARALLELISM"`
	// Single-use token lifetimes (seconds): password reset is short, invite is
	// longer-lived to give an invitee time to act.
	PasswordResetTTLSeconds int `mapstructure:"BFF_PASSWORD_RESET_TTL_SECONDS"`
	InviteTTLSeconds        int `mapstructure:"BFF_INVITE_TTL_SECONDS"`
	// PublicBaseURL is the origin used to build invite / reset links (e.g.
	// https://aer.example). Empty yields relative links (fine for the manual
	// log sender used until SMTP lands).
	PublicBaseURL string `mapstructure:"BFF_PUBLIC_BASE_URL"`
}

// Load reads configuration from environment variables and the local .env file.
func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "INFO")
	v.SetDefault("BFF_PORT", "8080")
	v.SetDefault("CLICKHOUSE_HOST", "localhost")
	v.SetDefault("CLICKHOUSE_PORT", "9002")
	v.SetDefault("CLICKHOUSE_USER", "")
	v.SetDefault("CLICKHOUSE_PASSWORD", "")
	v.SetDefault("CLICKHOUSE_DB", "aer_gold")
	v.SetDefault("BFF_API_KEY", "")
	v.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	v.SetDefault("OTEL_TRACE_SAMPLE_RATE", 1.0)
	v.SetDefault("CORS_ALLOWED_ORIGINS", "*")
	v.SetDefault("RATE_LIMIT_RPS", 100)
	v.SetDefault("RATE_LIMIT_BURST", 200)
	v.SetDefault("BFF_QUERY_ROW_LIMIT", 10000)
	v.SetDefault("BFF_METRICS_CACHE_TTL_SECONDS", 60)
	v.SetDefault("BFF_SHUTDOWN_TIMEOUT_SECONDS", 65)
	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5432")
	v.SetDefault("POSTGRES_DB", "aer_metadata")
	v.SetDefault("BFF_DB_USER", "")
	v.SetDefault("BFF_DB_PASSWORD", "")
	v.SetDefault("BFF_SOURCES_CACHE_TTL_SECONDS", 60)
	v.SetDefault("BFF_CONFIG_DIR", "configs")
	v.SetDefault("MINIO_ENDPOINT", "localhost:9000")
	v.SetDefault("MINIO_USE_SSL", false)
	v.SetDefault("BFF_MINIO_ACCESS_KEY", "")
	v.SetDefault("BFF_MINIO_SECRET_KEY", "")
	v.SetDefault("BFF_K_ANONYMITY_THRESHOLD", 10)
	// Access control (Phase 134 / ADR-040).
	v.SetDefault("BFF_AUTH_DB_USER", "")
	v.SetDefault("BFF_AUTH_DB_PASSWORD", "")
	v.SetDefault("BFF_SECURE_COOKIES", true)
	v.SetDefault("BFF_SESSION_IDLE_SECONDS", 28800)      // 8 h sliding
	v.SetDefault("BFF_SESSION_ABSOLUTE_SECONDS", 604800) // 7 d hard cap
	v.SetDefault("BFF_ARGON2_MEMORY_KIB", 19456)         // 19 MiB (OWASP)
	v.SetDefault("BFF_ARGON2_ITERATIONS", 2)
	v.SetDefault("BFF_ARGON2_PARALLELISM", 1)
	v.SetDefault("BFF_PASSWORD_RESET_TTL_SECONDS", 3600) // 1 h
	v.SetDefault("BFF_INVITE_TTL_SECONDS", 259200)       // 72 h
	v.SetDefault("BFF_PUBLIC_BASE_URL", "")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("BFF_API_KEY must be set")
	}
	if cfg.ClickHousePassword == "" {
		return nil, fmt.Errorf("CLICKHOUSE_PASSWORD must be set")
	}
	if cfg.BFFDBUser == "" {
		return nil, fmt.Errorf("BFF_DB_USER must be set")
	}
	if cfg.BFFDBPassword == "" {
		return nil, fmt.Errorf("BFF_DB_PASSWORD must be set")
	}
	if cfg.MinioAccessKey == "" {
		return nil, fmt.Errorf("BFF_MINIO_ACCESS_KEY must be set")
	}
	if cfg.MinioSecretKey == "" {
		return nil, fmt.Errorf("BFF_MINIO_SECRET_KEY must be set")
	}
	// Phase 134 / ADR-040: the auth write role is required — the BFF is the
	// auth authority and refuses to boot without a way to persist sessions.
	if cfg.BFFAuthDBUser == "" {
		return nil, fmt.Errorf("BFF_AUTH_DB_USER must be set")
	}
	if cfg.BFFAuthDBPassword == "" {
		return nil, fmt.Errorf("BFF_AUTH_DB_PASSWORD must be set")
	}

	return &cfg, nil
}
