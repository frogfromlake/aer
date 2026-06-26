package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setRequiredEnv populates every secret Load() validates as non-empty so that
// a successful-load test can selectively unset one to exercise a single error
// path. It runs from an isolated working directory (no .env to read).
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BFF_API_KEY", "test-api-key")
	t.Setenv("CLICKHOUSE_PASSWORD", "ch-secret")
	t.Setenv("BFF_DB_USER", "bff_readonly")
	t.Setenv("BFF_DB_PASSWORD", "bff-secret")
	t.Setenv("BFF_MINIO_ACCESS_KEY", "minio-key")
	t.Setenv("BFF_MINIO_SECRET_KEY", "minio-secret")
	t.Setenv("BFF_AUTH_DB_USER", "bff_auth")
	t.Setenv("BFF_AUTH_DB_PASSWORD", "auth-secret")
}

// setProdLinkEnv supplies the production link + WebAuthn coherence values that
// SEC-036/039 require, so a prod happy-path test can boot. Tests exercising a
// single missing/invalid value clear or override one after calling this.
func setProdLinkEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BFF_PUBLIC_BASE_URL", "https://aer-project.eu")
	t.Setenv("BFF_WEBAUTHN_RP_ID", "aer-project.eu")
	t.Setenv("BFF_WEBAUTHN_RP_ORIGINS", "https://aer-project.eu")
}

// chdirEmpty moves into a fresh temp dir so Load()'s best-effort ReadInConfig
// (".env") finds no file and falls back to env + defaults only. This keeps the
// test independent of any .env the developer happens to have at the repo root.
func chdirEmpty(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}

func TestLoad_SucceedsWithRequiredEnv(t *testing.T) {
	chdirEmpty(t)
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "test-api-key" {
		t.Errorf("APIKey = %q, want test-api-key", cfg.APIKey)
	}
	if cfg.ClickHousePassword != "ch-secret" {
		t.Errorf("ClickHousePassword = %q", cfg.ClickHousePassword)
	}
	if cfg.BFFAuthDBUser != "bff_auth" {
		t.Errorf("BFFAuthDBUser = %q", cfg.BFFAuthDBUser)
	}
}

// TestLoad_AppliesDefaults verifies the SetDefault calls actually surface on
// the unmarshalled struct when the env does not override them.
func TestLoad_AppliesDefaults(t *testing.T) {
	chdirEmpty(t)
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Environment", cfg.Environment, "development"},
		{"LogLevel", cfg.LogLevel, "INFO"},
		{"BFFPort", cfg.BFFPort, "8080"},
		{"ClickHouseDB", cfg.ClickHouseDB, "aer_gold"},
		{"OTelSampleRate", cfg.OTelSampleRate, 1.0},
		{"RateLimitBurst", cfg.RateLimitBurst, 200},
		{"QueryRowLimit", cfg.QueryRowLimit, 10000},
		{"ShutdownTimeoutSeconds", cfg.ShutdownTimeoutSeconds, 65},
		{"ConfigDir", cfg.ConfigDir, "configs"},
		{"KAnonymityThreshold", cfg.KAnonymityThreshold, 10},
		{"SecureCookies", cfg.SecureCookies, true},
		{"SessionIdleSeconds", cfg.SessionIdleSeconds, 28800},
		{"SessionAbsoluteSeconds", cfg.SessionAbsoluteSeconds, 604800},
		{"Argon2MemoryKiB", cfg.Argon2MemoryKiB, 19456},
		{"Argon2Iterations", cfg.Argon2Iterations, 2},
		{"PasswordResetTTLSeconds", cfg.PasswordResetTTLSeconds, 3600},
		{"InviteTTLSeconds", cfg.InviteTTLSeconds, 259200},
		{"WebAuthnRPID", cfg.WebAuthnRPID, "localhost"},
		{"WebAuthnRPOrigins", cfg.WebAuthnRPOrigins, "https://localhost"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestLoad_EnvOverridesDefault confirms AutomaticEnv binds an env var over the
// configured default (i.e. the mapstructure tag wiring is correct).
func TestLoad_EnvOverridesDefault(t *testing.T) {
	chdirEmpty(t)
	setRequiredEnv(t)
	t.Setenv("BFF_PORT", "9191")
	t.Setenv("BFF_SECURE_COOKIES", "false")
	t.Setenv("BFF_QUERY_ROW_LIMIT", "42")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BFFPort != "9191" {
		t.Errorf("BFFPort = %q, want 9191", cfg.BFFPort)
	}
	if cfg.SecureCookies {
		t.Error("SecureCookies should be false when env sets it false")
	}
	if cfg.QueryRowLimit != 42 {
		t.Errorf("QueryRowLimit = %d, want 42", cfg.QueryRowLimit)
	}
}

// TestLoad_MissingRequiredSecret exercises every required-secret guard. For
// each, the full set is present except the one under test, which is cleared.
func TestLoad_MissingRequiredSecret(t *testing.T) {
	cases := []struct {
		name    string
		clear   string
		wantSub string
	}{
		{"api key", "BFF_API_KEY", "BFF_API_KEY must be set"},
		{"clickhouse password", "CLICKHOUSE_PASSWORD", "CLICKHOUSE_PASSWORD must be set"},
		{"db user", "BFF_DB_USER", "BFF_DB_USER must be set"},
		{"db password", "BFF_DB_PASSWORD", "BFF_DB_PASSWORD must be set"},
		{"minio access key", "BFF_MINIO_ACCESS_KEY", "BFF_MINIO_ACCESS_KEY must be set"},
		{"minio secret key", "BFF_MINIO_SECRET_KEY", "BFF_MINIO_SECRET_KEY must be set"},
		{"auth db user", "BFF_AUTH_DB_USER", "BFF_AUTH_DB_USER must be set"},
		{"auth db password", "BFF_AUTH_DB_PASSWORD", "BFF_AUTH_DB_PASSWORD must be set"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chdirEmpty(t)
			setRequiredEnv(t)
			// Clearing the env var means AutomaticEnv reads the empty default,
			// which trips the corresponding guard.
			t.Setenv(c.clear, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error when %s is empty", c.clear)
			}
			if got := err.Error(); !strings.Contains(got, c.wantSub) {
				t.Errorf("error = %q, want substring %q", got, c.wantSub)
			}
		})
	}
}

// TestLoad_EmailDisabledByDefault confirms an unset SMTP_HOST loads cleanly and
// EmailEnabled reports false (the LogSender fallback path, Phase 153).
func TestLoad_EmailDisabledByDefault(t *testing.T) {
	chdirEmpty(t)
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.EmailEnabled() {
		t.Error("EmailEnabled() = true with no SMTP_HOST, want false")
	}
	if cfg.SMTPPort != "587" {
		t.Errorf("SMTPPort default = %q, want 587", cfg.SMTPPort)
	}
}

// TestLoad_SMTPGroupAllOrNothing exercises the Phase 153 grouped validation: a
// set SMTP_HOST requires the rest of the credential group, and a complete group
// flips EmailEnabled to true.
func TestLoad_SMTPGroupAllOrNothing(t *testing.T) {
	t.Run("host set but credential missing fails", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("SMTP_HOST", "smtp-relay.brevo.com")
		t.Setenv("SMTP_USERNAME", "relay-user")
		t.Setenv("SMTP_PASSWORD", "relay-key")
		// SMTP_FROM_ADDRESS deliberately left empty.

		_, err := Load()
		if err == nil {
			t.Fatal("expected error when SMTP_HOST set but SMTP_FROM_ADDRESS empty")
		}
		if !strings.Contains(err.Error(), "SMTP_FROM_ADDRESS") {
			t.Errorf("error = %q, want it to name SMTP_FROM_ADDRESS", err.Error())
		}
	})

	t.Run("complete group enables email", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("SMTP_HOST", "smtp-relay.brevo.com")
		t.Setenv("SMTP_USERNAME", "relay-user")
		t.Setenv("SMTP_PASSWORD", "relay-key")
		t.Setenv("SMTP_FROM_ADDRESS", "noreply@aer.example")
		t.Setenv("BFF_PUBLIC_BASE_URL", "https://aer.example") // SEC-027

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !cfg.EmailEnabled() {
			t.Error("EmailEnabled() = false with a complete SMTP group, want true")
		}
	})

	t.Run("SMTP on without base URL fails (SEC-027)", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("SMTP_HOST", "smtp-relay.brevo.com")
		t.Setenv("SMTP_USERNAME", "relay-user")
		t.Setenv("SMTP_PASSWORD", "relay-key")
		t.Setenv("SMTP_FROM_ADDRESS", "noreply@aer.example")
		// BFF_PUBLIC_BASE_URL deliberately left empty.

		_, err := Load()
		if err == nil {
			t.Fatal("expected error when SMTP_HOST set but BFF_PUBLIC_BASE_URL empty")
		}
		if !strings.Contains(err.Error(), "BFF_PUBLIC_BASE_URL") {
			t.Errorf("error = %q, want it to name BFF_PUBLIC_BASE_URL", err.Error())
		}
	})
}

func TestLoad_SecureCookiesRequiredInProduction(t *testing.T) {
	t.Run("production + insecure cookies refuses to boot", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("BFF_SECURE_COOKIES", "false")

		_, err := Load()
		if err == nil {
			t.Fatal("expected Load to fail for insecure cookies in production")
		}
		if !strings.Contains(err.Error(), "BFF_SECURE_COOKIES") {
			t.Errorf("error = %q, want it to name BFF_SECURE_COOKIES", err.Error())
		}
	})

	t.Run("production + secure cookies boots", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("APP_ENV", "production")
		// BFF_SECURE_COOKIES defaults to true. A non-wildcard CORS origin is
		// required in production (SEC-010), so set one for this happy path.
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://aer.example")
		setProdLinkEnv(t) // SEC-036/039: prod also requires real base URL + RP.

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !cfg.SecureCookies {
			t.Error("SecureCookies = false, want true (the production default)")
		}
	})

	t.Run("development tolerates insecure cookies", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("BFF_SECURE_COOKIES", "false") // APP_ENV defaults to development

		if _, err := Load(); err != nil {
			t.Fatalf("development must tolerate insecure cookies: %v", err)
		}
	})
}

func TestLoad_CORSWildcardRefusedOutsideDevelopment(t *testing.T) {
	t.Run("production + wildcard CORS refuses to boot", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("APP_ENV", "production")
		// CORS_ALLOWED_ORIGINS defaults to "*".

		_, err := Load()
		if err == nil {
			t.Fatal("expected Load to fail for wildcard CORS in production")
		}
		if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
			t.Errorf("error = %q, want it to name CORS_ALLOWED_ORIGINS", err.Error())
		}
	})

	t.Run("production + explicit origin boots", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://aer.example")
		setProdLinkEnv(t) // SEC-036/039: prod also requires real base URL + RP.

		if _, err := Load(); err != nil {
			t.Fatalf("explicit origin in production must boot: %v", err)
		}
	})

	t.Run("development tolerates wildcard", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t) // APP_ENV defaults to development, CORS defaults to "*"

		if _, err := Load(); err != nil {
			t.Fatalf("development must tolerate the wildcard default: %v", err)
		}
	})
}

// TestLoad_ProductionLinkCoherence exercises the SEC-036/039 prod guards: the
// localhost/empty dev defaults must not survive into a production boot, and a
// coherent set of real values boots.
func TestLoad_ProductionLinkCoherence(t *testing.T) {
	// A correct prod base config the failure cases mutate one field of.
	base := func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://aer-project.eu")
		setProdLinkEnv(t)
	}

	cases := []struct {
		name    string
		mutate  func(t *testing.T)
		wantSub string
	}{
		{"empty base URL", func(t *testing.T) { t.Setenv("BFF_PUBLIC_BASE_URL", "") }, "BFF_PUBLIC_BASE_URL"},
		{"non-https base URL", func(t *testing.T) { t.Setenv("BFF_PUBLIC_BASE_URL", "http://aer-project.eu") }, "https://"},
		{"localhost base URL", func(t *testing.T) { t.Setenv("BFF_PUBLIC_BASE_URL", "https://localhost") }, "localhost"},
		{"localhost RP id", func(t *testing.T) { t.Setenv("BFF_WEBAUTHN_RP_ID", "localhost") }, "BFF_WEBAUTHN_RP_ID"},
		{"localhost RP origin", func(t *testing.T) { t.Setenv("BFF_WEBAUTHN_RP_ORIGINS", "https://localhost") }, "BFF_WEBAUTHN_RP_ORIGINS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base(t)
			c.mutate(t)
			_, err := Load()
			if err == nil {
				t.Fatalf("expected Load to fail for %s in production", c.name)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), c.wantSub)
			}
		})
	}

	t.Run("coherent prod values boot", func(t *testing.T) {
		base(t)
		if _, err := Load(); err != nil {
			t.Fatalf("coherent prod link config must boot: %v", err)
		}
	})

	t.Run("development tolerates localhost defaults", func(t *testing.T) {
		chdirEmpty(t)
		setRequiredEnv(t) // APP_ENV defaults to development; RP defaults to localhost
		if _, err := Load(); err != nil {
			t.Fatalf("development must tolerate localhost RP + empty base URL: %v", err)
		}
	})
}

// TestLoad_ReadsDotEnvFile confirms the best-effort .env read path is wired:
// a value present only in a .env file (not in the environment) reaches the
// struct.
func TestLoad_ReadsDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	envBody := "" +
		"BFF_API_KEY=file-api-key\n" +
		"CLICKHOUSE_PASSWORD=file-ch\n" +
		"BFF_DB_USER=file-db-user\n" +
		"BFF_DB_PASSWORD=file-db-pass\n" +
		"BFF_MINIO_ACCESS_KEY=file-minio-key\n" +
		"BFF_MINIO_SECRET_KEY=file-minio-secret\n" +
		"BFF_AUTH_DB_USER=file-auth-user\n" +
		"BFF_AUTH_DB_PASSWORD=file-auth-pass\n" +
		"BFF_PORT=7777\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "file-api-key" {
		t.Errorf("APIKey = %q, want file-api-key (from .env)", cfg.APIKey)
	}
	if cfg.BFFPort != "7777" {
		t.Errorf("BFFPort = %q, want 7777 (from .env)", cfg.BFFPort)
	}
}
