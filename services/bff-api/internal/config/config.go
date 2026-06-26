// Package config loads and validates the BFF API's environment-driven
// configuration via viper. Required secrets are validated at startup so the
// internet-facing service refuses to boot with an empty credential.
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
	// WebAuthn relying-party (Phase 134 / ADR-040). RPID is the registrable
	// domain (e.g. "localhost" or "aer.example"); RPOrigins is a comma-separated
	// list of full browser origins (e.g. "https://localhost").
	WebAuthnRPID          string `mapstructure:"BFF_WEBAUTHN_RP_ID"`
	WebAuthnRPDisplayName string `mapstructure:"BFF_WEBAUTHN_RP_DISPLAY_NAME"`
	WebAuthnRPOrigins     string `mapstructure:"BFF_WEBAUTHN_RP_ORIGINS"`

	// --- Transactional email (Phase 153 / ADR-043) ---
	//
	// Provider-agnostic SMTP submission relay (the documented default is Brevo,
	// EU/DSGVO). An empty SMTPHost falls back to notify.LogSender (links are
	// logged, not emailed) so local/dev and the `make create-admin` break-glass
	// path keep working without a relay. When SMTPHost is set, the rest of the
	// group is required (validated in Load) — a half-configured relay is a boot
	// error, never a silent half-send on an internet-facing deployment.
	SMTPHost        string `mapstructure:"SMTP_HOST"`
	SMTPPort        string `mapstructure:"SMTP_PORT"`
	SMTPUsername    string `mapstructure:"SMTP_USERNAME"`
	SMTPPassword    string `mapstructure:"SMTP_PASSWORD"`
	SMTPFromAddress string `mapstructure:"SMTP_FROM_ADDRESS"`
	SMTPFromName    string `mapstructure:"SMTP_FROM_NAME"`
}

// EmailEnabled reports whether a real transactional-email relay is configured.
// False means the BFF falls back to notify.LogSender (links logged only).
func (c *Config) EmailEnabled() bool { return c.SMTPHost != "" }

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
	v.SetDefault("BFF_WEBAUTHN_RP_ID", "localhost")
	v.SetDefault("BFF_WEBAUTHN_RP_DISPLAY_NAME", "AĒR")
	v.SetDefault("BFF_WEBAUTHN_RP_ORIGINS", "https://localhost")
	// Transactional email (Phase 153 / ADR-043). Empty host → LogSender fallback.
	v.SetDefault("SMTP_HOST", "")
	v.SetDefault("SMTP_PORT", "587")
	v.SetDefault("SMTP_USERNAME", "")
	v.SetDefault("SMTP_PASSWORD", "")
	v.SetDefault("SMTP_FROM_ADDRESS", "")
	v.SetDefault("SMTP_FROM_NAME", "AĒR")

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
	// Transactional email (Phase 153 / ADR-043): optional, but all-or-nothing.
	// A set host with a missing credential is a misconfiguration, not a reason
	// to silently fall back to logging links on an internet-facing deployment.
	if cfg.SMTPHost != "" {
		var missing []string
		if cfg.SMTPPort == "" {
			missing = append(missing, "SMTP_PORT")
		}
		if cfg.SMTPUsername == "" {
			missing = append(missing, "SMTP_USERNAME")
		}
		if cfg.SMTPPassword == "" {
			missing = append(missing, "SMTP_PASSWORD")
		}
		if cfg.SMTPFromAddress == "" {
			missing = append(missing, "SMTP_FROM_ADDRESS")
		}
		// SEC-027 — with SMTP on, invite/reset links are built by concatenating
		// BFF_PUBLIC_BASE_URL; an empty base URL emits relative links that don't
		// resolve in a mail client. Require it in the same all-or-nothing block.
		if cfg.PublicBaseURL == "" {
			missing = append(missing, "BFF_PUBLIC_BASE_URL")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("SMTP_HOST is set but %s missing", strings.Join(missing, ", "))
		}
	}

	// SEC-004 — BFF_SECURE_COOKIES=false drops the __Host- prefix and the
	// Secure flag, leaving the opaque session id replayable over a cleartext
	// hop. It is the only production-unsafe value, with no legitimate use on an
	// internet-facing deployment (Traefik is the sole TLS ingress), so refuse to
	// boot rather than silently emit insecure session cookies.
	if cfg.Environment == "production" && !cfg.SecureCookies {
		return nil, fmt.Errorf("BFF_SECURE_COOKIES must be true when APP_ENV=production")
	}

	// SEC-010 — `*` is acceptable only in development. A wildcard CORS origin in
	// any other environment is a latent footgun (a future AllowCredentials flip
	// would expose a logged-in researcher's responses cross-origin), and the
	// knob was silently ignored before. Refuse to boot so production must set
	// CORS_ALLOWED_ORIGINS to explicit origin(s) rather than run the wildcard.
	if cfg.Environment != "development" && cfg.CORSOrigins == "*" {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS must not be '*' when APP_ENV=%s; set explicit origin(s)", cfg.Environment)
	}

	// SEC-036 / SEC-039 — production link + WebAuthn coherence. The dev defaults
	// (empty BFF_PUBLIC_BASE_URL, WebAuthn RP = localhost) silently survive into
	// a prod boot and produce invite/reset links that don't resolve in a mail
	// client and a relying-party browsers reject at origin verification — i.e. a
	// box that cannot onboard even its first admin. Fail fast instead.
	if cfg.Environment == "production" {
		switch {
		case cfg.PublicBaseURL == "":
			return nil, fmt.Errorf("BFF_PUBLIC_BASE_URL must be set when APP_ENV=production")
		case !strings.HasPrefix(cfg.PublicBaseURL, "https://"):
			return nil, fmt.Errorf("BFF_PUBLIC_BASE_URL must be an https:// origin when APP_ENV=production")
		case strings.Contains(cfg.PublicBaseURL, "localhost"):
			return nil, fmt.Errorf("BFF_PUBLIC_BASE_URL must be the deployed domain, not localhost, when APP_ENV=production")
		}
		if cfg.WebAuthnRPID == "" || cfg.WebAuthnRPID == "localhost" {
			return nil, fmt.Errorf("BFF_WEBAUTHN_RP_ID must be the deployed domain (not localhost) when APP_ENV=production")
		}
		if strings.Contains(cfg.WebAuthnRPOrigins, "localhost") {
			return nil, fmt.Errorf("BFF_WEBAUTHN_RP_ORIGINS must not contain localhost when APP_ENV=production")
		}
	}

	return &cfg, nil
}
