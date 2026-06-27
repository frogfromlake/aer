// Command server is the bff-api service — AĒR's only internet-facing backend.
// It serves the contract-first REST API (generated from openapi.yaml) behind
// session-or-API-key auth, reading ClickHouse Gold (read-only) and PostgreSQL
// (auth/analyses under the bff_auth role), with the standard HTTP timeout and
// graceful-shutdown guards.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/time/rate"

	"github.com/frogfromlake/aer/pkg/logger"
	mw "github.com/frogfromlake/aer/pkg/middleware"
	"github.com/frogfromlake/aer/pkg/telemetry"
	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/handler"
	"github.com/frogfromlake/aer/services/bff-api/internal/notify"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func main() {
	// 1. Setup Context FIRST so backoff respects interrupts
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// 3. Initialize Logger
	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR BFF API...", "environment", cfg.Environment)

	// 4. Initialize OpenTelemetry
	shutdownTracer, err := telemetry.InitProvider("aer-bff-api", cfg.OTelEndpoint, cfg.OTelSampleRate)
	if err != nil {
		slog.Error("Failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			slog.Error("Error during OpenTelemetry shutdown", "error", err)
		}
	}()

	// 5. Initialize Storage (Passing Context for Backoff)
	chAddr := cfg.ClickHouseHost + ":" + cfg.ClickHousePort
	cacheTTL := time.Duration(cfg.MetricsCacheTTLSecs) * time.Second
	chStore, err := storage.NewClickHouseStorage(
		ctx,
		chAddr,
		cfg.ClickHouseUser,
		cfg.ClickHousePassword,
		cfg.ClickHouseDB,
		cfg.QueryRowLimit,
		cacheTTL,
	)
	if err != nil {
		slog.Error("Failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	slog.Info("ClickHouse connected successfully")
	// SEC-088 — drain the ClickHouse pool on shutdown, mirroring the PG-pool
	// defers below. Close() is nil-safe and idempotent.
	defer func() {
		if err := chStore.Close(); err != nil {
			slog.Error("Error closing ClickHouse pool", "error", err)
		}
	}()

	// 6. Load static BFF config (metric provenance). Source metadata now
	// lives in Postgres (Phase 87 — no more YAML mirror).
	provenance, err := config.LoadMetricProvenance(filepath.Join(cfg.ConfigDir, "metric_provenance.yaml"))
	if err != nil {
		slog.Error("Failed to load metric_provenance.yaml", "error", err)
		os.Exit(1)
	}

	// 6b. Load the Dual-Register content catalog from the YAML files bundled
	// with the binary. Malformed files abort startup so broken content is
	// caught before any request is served.
	catalog, err := config.LoadContentCatalog(filepath.Join(cfg.ConfigDir, "content"))
	if err != nil {
		slog.Error("Failed to load content catalog", "error", err)
		os.Exit(1)
	}

	// 6d. Load the probe registry (structural probe data for the Atmosphere
	// surface). One YAML per probe; malformed files abort startup so a
	// broken probe does not silently disappear from the globe.
	probes, err := config.LoadProbeRegistry(filepath.Join(cfg.ConfigDir, "probes"))
	if err != nil {
		slog.Error("Failed to load probe registry", "error", err)
		os.Exit(1)
	}

	// 6e. Load the Language Capability Manifest (ADR-024 / Phase 118a). The
	// manifest is the SoT for valid `?language=` query-parameter values across
	// every endpoint that takes one. Malformed or absent → fatal startup so
	// the validator never silently degrades to a permissive default.
	//
	// In the runtime image the file is copied alongside the binary by the
	// Dockerfile. For local `go run` the bundled BFF configs/ directory does
	// not carry it, so we fall back to the SoT path under
	// services/analysis-worker/configs/.
	manifestPath := filepath.Join(cfg.ConfigDir, "language_capabilities.yaml")
	if _, statErr := os.Stat(manifestPath); statErr != nil {
		manifestPath = filepath.Join("..", "analysis-worker", "configs", "language_capabilities.yaml")
	}
	languageManifest, err := config.LoadLanguageManifest(manifestPath)
	if err != nil {
		slog.Error("Failed to load language capability manifest", "error", err)
		os.Exit(1)
	}

	// 6c. Open the read-only PostgreSQL pool that backs /api/v1/sources.
	// The pool is intentionally tiny: /sources is served from a TTL cache
	// so we only need enough capacity to refresh every BFF_SOURCES_CACHE_TTL
	// and absorb the occasional startup probe.
	pgDB, err := openSourcesPool(ctx, cfg)
	if err != nil {
		slog.Error("Failed to connect to Postgres for /sources", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := pgDB.Close(); err != nil {
			slog.Error("Error closing Postgres pool", "error", err)
		}
	}()
	sourcesTTL := time.Duration(cfg.SourcesCacheTTLSecs) * time.Second
	sourceStore := storage.NewSourceStore(pgDB, sourcesTTL)
	dossierStore := storage.NewDossierStore(pgDB, chStore.Conn())

	// 6f. Open the auth write pool (Phase 134 / ADR-040). A second, small
	// Postgres pool under the dedicated `bff_auth` role (DML on the auth
	// tables only) — kept separate from the read-only analytics pool so the
	// analytics path retains zero write authority.
	authPool, err := openAuthPool(ctx, cfg)
	if err != nil {
		slog.Error("Failed to connect to Postgres for auth", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := authPool.Close(); err != nil {
			slog.Error("Error closing auth Postgres pool", "error", err)
		}
	}()
	authStore := storage.NewAuthStore(authPool)
	webAuthnStore := storage.NewWebAuthnStore(authPool)
	analysesStore := storage.NewAnalysesStore(authPool)

	// WebAuthn relying-party (Phase 134 / ADR-040).
	webAuthn, err := auth.NewWebAuthn(
		cfg.WebAuthnRPID,
		cfg.WebAuthnRPDisplayName,
		strings.Split(cfg.WebAuthnRPOrigins, ","),
	)
	if err != nil {
		slog.Error("Failed to initialise WebAuthn", "error", err)
		os.Exit(1)
	}

	// Cookie name: the `__Host-` prefix requires Secure; drop it for local
	// http / Testcontainers (BFF_SECURE_COOKIES=false).
	cookieName := "__Host-aer_session"
	if !cfg.SecureCookies {
		cookieName = "aer_session"
	}
	authConfig := handler.AuthConfig{
		CookieName:      cookieName,
		SecureCookies:   cfg.SecureCookies,
		SessionIdle:     time.Duration(cfg.SessionIdleSeconds) * time.Second,
		SessionAbsolute: time.Duration(cfg.SessionAbsoluteSeconds) * time.Second,
		Argon2: auth.Argon2Params{
			MemoryKiB:   uint32(cfg.Argon2MemoryKiB),
			Iterations:  uint32(cfg.Argon2Iterations),
			Parallelism: uint8(cfg.Argon2Parallelism),
			SaltLen:     16,
			KeyLen:      32,
		},
		ResetTTL:      time.Duration(cfg.PasswordResetTTLSeconds) * time.Second,
		InviteTTL:     time.Duration(cfg.InviteTTLSeconds) * time.Second,
		PublicBaseURL: cfg.PublicBaseURL,
	}

	// Periodic expired-session cleanup (best-effort housekeeping).
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := authStore.DeleteExpiredSessions(context.Background()); err != nil {
					slog.Warn("session cleanup failed", "error", err)
				} else if n > 0 {
					slog.Info("session cleanup", "deleted", n)
				}
			}
		}
	}()

	// Phase 101: read-only Silver access for L5 Evidence article-detail.
	silverStore, err := storage.NewSilverStore(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioUseSSL)
	if err != nil {
		slog.Error("Failed to initialise Silver MinIO client", "error", err)
		os.Exit(1)
	}

	// Transactional email (Phase 153 / ADR-043). A configured SMTP relay
	// delivers invite/reset links; otherwise the LogSender keeps the
	// `make create-admin` break-glass path working by logging the links.
	var mailer notify.LinkSender = notify.LogSender{}
	if cfg.EmailEnabled() {
		mailer = notify.NewSMTPSender(notify.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFromAddress,
			FromName: cfg.SMTPFromName,
		})
		slog.Info("auth: transactional email configured", "relay", cfg.SMTPHost, "from", cfg.SMTPFromAddress)
	} else {
		slog.Warn("auth: no SMTP relay configured — invite/reset links are logged, not emailed (LogSender)")
	}

	// 7. Setup Handlers and Router
	serverLogic := handler.NewServerWithOptions(chStore, provenance, sourceStore, catalog, probes, handler.ServerOptions{
		Dossier:             dossierStore,
		Articles:            chStore,
		Silver:              silverStore,
		KAnonymityThreshold: cfg.KAnonymityThreshold,
		LanguageManifest:    languageManifest,
		Auth:                authStore,
		AuthConfig:          authConfig,
		Mailer:              mailer,
		EmailEnabled:        cfg.EmailEnabled(),
		WebAuthn:            webAuthn,
		WebAuthnBE:          webAuthnStore,
		Analyses:            analysesStore,
	})
	// Custom strict-handler error handling (SEC-015 + SEC-017): map an oversized
	// body to 413, and return generic 400/500 bodies that never leak binding
	// internals or dependency errors to the client.
	strictHandler := handler.NewStrictHandlerWithOptions(serverLogic, nil, handler.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestBindingErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler,
	})

	// Periodically drop idle login-throttle keys (security review M-3).
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				serverLogic.LoginThrottle().Sweep()
				serverLogic.ResetThrottle().Sweep()
			}
		}
	}()

	r := chi.NewRouter()

	// Recovery runs on every request (including /metrics) so a panic in any
	// handler becomes a 500 instead of crashing the server.
	r.Use(middleware.Recoverer)

	// Prometheus scrape endpoint sits outside the API-key middleware: the
	// backend network is zero-trust-internal only, and scrape targets
	// cannot carry per-caller secrets.
	r.Handle("/metrics", promhttp.Handler())

	// --- API ROUTE GROUP (authenticated, rate-limited, traced) ---
	r.Group(func(r chi.Router) {
		// Request Timeout: limits each request to 30s to prevent hanging ClickHouse queries
		r.Use(middleware.Timeout(30 * time.Second))

		// Request body cap (SEC-015): wrap every body in MaxBytesReader before any
		// decoder runs, so an oversized payload (incl. on the pre-auth /auth/*
		// POSTs) fails with 413 instead of buffering unbounded memory.
		r.Use(bodyCap(maxRequestBodyBytes))

		// CORS: configurable allowed origins via CORS_ALLOWED_ORIGINS env var.
		// POST is required for the /auth/* endpoints (Phase 134 / ADR-040).
		allowedOrigins := strings.Split(cfg.CORSOrigins, ",")
		r.Use(mw.NewCORSHandler(allowedOrigins, []string{"GET", "POST", "OPTIONS"}))

		// Security hardening (Phase 134 / ADR-040): HSTS on every response, and
		// Fetch-Metadata CSRF rejection of cross-site state-changing requests
		// (defense-in-depth on the SameSite=Strict session cookie). Both are
		// frontend-free; machine X-API-Key callers send no Sec-Fetch-Site and
		// are unaffected.
		r.Use(auth.SecurityHeaders)
		r.Use(auth.FetchMetadataCSRF)

		// Inject the client IP so the login throttle (security review M-3) can
		// key by IP from inside the strict handler.
		r.Use(auth.ClientIP)

		// OTel: wraps each request in a span and propagates the trace context
		r.Use(func(next http.Handler) http.Handler {
			return otelhttp.NewHandler(next, "bff-api")
		})

		// Observability (Phase 154). Runs after otelhttp so the server span
		// exists: TraceIDHeader surfaces X-Trace-ID on every response (operator
		// pivots from a 5xx to its trace), RequestLogger emits the access log
		// with the active trace-id, and PrometheusMetrics records the
		// request-rate/latency/error-rate signals scraped from /metrics.
		r.Use(mw.TraceIDHeaderMiddleware)
		r.Use(mw.RequestLogger("bff-api"))
		r.Use(mw.PrometheusMetrics("bff-api"))

		// Rate Limiting (SEC-014): PER-CLIENT token-bucket limiter keyed by the
		// SEC-003 client IP, so one abusive caller exhausts only its own budget
		// instead of 429-ing every user via a single shared limiter. Pre-auth
		// /auth/* requests draw on a separate, tighter per-IP budget (the
		// brute-force surface). Idle buckets are evicted so unique source IPs
		// cannot grow the map without bound.
		clientLimiter := newClientRateLimiter(
			rateBudget{rps: rate.Limit(cfg.RateLimitRPS), burst: cfg.RateLimitBurst},
			rateBudget{rps: rate.Limit(cfg.RateLimitAuthRPS), burst: cfg.RateLimitAuthBurst},
			clientLimiterIdleTTL, clientLimiterSweepInterval,
		)
		r.Use(rateLimiter(clientLimiter))

		// Access control (Phase 134 / ADR-040): every request must carry a
		// valid session cookie OR a valid X-API-Key (the demoted machine
		// credential). The pre-auth endpoints (login, accept-invite,
		// forgot/reset password) and the health probes are exempt.
		r.Use(auth.SessionOrAPIKey(authStore, auth.MiddlewareConfig{
			APIKey:     cfg.APIKey,
			CookieName: cookieName,
			IdleTTL:    time.Duration(cfg.SessionIdleSeconds) * time.Second,
			// Exact mounted paths (the /api/v1 base is hardcoded below in
			// HandlerFromMuxWithBaseURL). Whole-path equality, not suffix, so a
			// crafted path like /api/v1/articles/healthz cannot bypass the gate
			// (SEC-013).
			ExemptPaths: []string{
				"/api/v1/healthz",
				"/api/v1/readyz",
				"/api/v1/auth/login",
				"/api/v1/auth/accept-invite",
				"/api/v1/auth/forgot-password",
				"/api/v1/auth/reset-password",
			},
		}))

		// RBAC: /api/v1/admin/* requires the admin role (runs after auth so the
		// identity is in context). Phase 134 / ADR-040. Prefix-anchored, not a
		// substring match, so it cannot over-block /content/admin/... or be
		// mis-gated by a crafted path (SEC-026); handlers also re-check in-handler
		// (SEC-025).
		r.Use(auth.RequireAdminForPrefix("/api/v1/admin/"))

		// Phase 122j J3: long browser cache for `/content/*` responses.
		// The catalog is loaded from versioned YAML at startup and only
		// changes when an operator restarts the service; the response
		// body also carries `contentVersion` for clients that want
		// stronger validation. Applied AFTER the auth middleware so
		// caller-specific concerns (rate limits, auth) still execute.
		r.Use(mw.CacheControlForPaths(24*time.Hour, "/api/v1/content/"))

		handler.HandlerFromMuxWithBaseURL(strictHandler, r, "/api/v1")
	})

	// --- GRACEFUL SHUTDOWN LOGIC ---
	//
	// HTTP server timeouts are hard defaults: ReadHeaderTimeout guards
	// against slowloris-style header stalls, WriteTimeout must exceed the
	// chi request timeout (30s) so a slow ClickHouse query still has time
	// to serialize its response, and ShutdownTimeout in turn must exceed
	// WriteTimeout so an in-flight request can drain on SIGTERM.
	server := &http.Server{
		Addr:              ":" + cfg.BFFPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	// Start server in a separate goroutine
	go func() {
		slog.Info("AĒR BFF API listening", "port", cfg.BFFPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server crashed", "error", err)
			os.Exit(1)
		}
	}()

	// Block main thread until a signal is received
	<-ctx.Done()
	slog.Info("Shutdown signal received. Shutting down BFF API gracefully...")

	// Grace period for active requests to finish before forced shutdown.
	shutdownTimeout := time.Duration(cfg.ShutdownTimeoutSeconds) * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced server shutdown", "error", err)
	} else {
		slog.Info("BFF API stopped cleanly.")
	}
}

// openSourcesPool opens a small, read-only PostgreSQL pool dedicated to
// the /sources endpoint. The BFF connects as a role that only holds
// SELECT on the `sources` table — any future schema escalation is caught
// at the database layer, not in Go. A short connection lifetime makes
// the pool resilient to long-running deployments where the underlying
// Postgres server restarts.
func openSourcesPool(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(cfg.BFFDBUser),
		url.QueryEscape(cfg.BFFDBPassword),
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open pgx pool: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

// openAuthPool opens the small write pool dedicated to the auth tables
// (Phase 134 / ADR-040). The BFF connects as the `bff_auth` role, which holds
// DML on users/sessions/auth_tokens only — any escalation is caught at the
// database layer, not in Go. Kept separate from the read-only analytics pool.
func openAuthPool(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(cfg.BFFAuthDBUser),
		url.QueryEscape(cfg.BFFAuthDBPassword),
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open auth pgx pool: %w", err)
	}
	db.SetMaxOpenConns(6)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping auth postgres: %w", err)
	}
	return db, nil
}

// clientLimiterIdleTTL / clientLimiterSweepInterval bound the per-client limiter
// map: an IP's buckets are dropped after it has been idle for the TTL, and the
// background sweep runs on the interval. The map therefore tracks only
// recently-active source IPs, never every IP ever seen.
const (
	clientLimiterIdleTTL       = 10 * time.Minute
	clientLimiterSweepInterval = 5 * time.Minute
)

// rateBudget is one token-bucket configuration (steady-state rps + burst).
type rateBudget struct {
	rps   rate.Limit
	burst int
}

// clientRateLimiter enforces a PER-CLIENT (per-IP) token-bucket rate limit
// (SEC-014). A single shared rate.Limiter let one noisy caller exhaust the whole
// app's request budget and 429 everyone; keying the limiter by the SEC-003 client
// IP contains the blast radius to the abuser. Pre-auth /auth/* requests draw on a
// separate, tighter per-IP bucket (the brute-force surface). A background sweep
// evicts idle buckets so unique source IPs cannot grow the map without bound.
type clientRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientBuckets
	general rateBudget
	strict  rateBudget
	idleTTL time.Duration
}

// clientBuckets holds one source IP's two token buckets plus its last-seen time
// (for eviction).
type clientBuckets struct {
	general  *rate.Limiter
	strict   *rate.Limiter
	lastSeen time.Time
}

// newClientRateLimiter builds a per-client limiter and starts its eviction sweep.
func newClientRateLimiter(general, strict rateBudget, idleTTL, sweep time.Duration) *clientRateLimiter {
	c := &clientRateLimiter{
		clients: make(map[string]*clientBuckets),
		general: general,
		strict:  strict,
		idleTTL: idleTTL,
	}
	go c.sweepLoop(sweep)
	return c
}

// sweepLoop periodically evicts idle per-IP buckets on the given interval.
func (c *clientRateLimiter) sweepLoop(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		c.evictIdle(time.Now())
	}
}

// evictIdle drops every per-IP bucket whose last request predates now-idleTTL,
// bounding the map to recently-active source IPs.
func (c *clientRateLimiter) evictIdle(now time.Time) {
	cutoff := now.Add(-c.idleTTL)
	c.mu.Lock()
	for ip, b := range c.clients {
		if b.lastSeen.Before(cutoff) {
			delete(c.clients, ip)
		}
	}
	c.mu.Unlock()
}

// allow reports whether a request from ip is within budget; strict selects the
// tighter pre-auth bucket. Buckets are created on first sight of an IP.
func (c *clientRateLimiter) allow(ip string, strict bool) bool {
	c.mu.Lock()
	b := c.clients[ip]
	if b == nil {
		b = &clientBuckets{
			general: rate.NewLimiter(c.general.rps, c.general.burst),
			strict:  rate.NewLimiter(c.strict.rps, c.strict.burst),
		}
		c.clients[ip] = b
	}
	b.lastSeen = time.Now()
	limiter := b.general
	if strict {
		limiter = b.strict
	}
	c.mu.Unlock()
	return limiter.Allow()
}

// rateLimiter returns a middleware enforcing the per-client limit. Requests over
// budget get an immediate 429. The client IP comes from the SEC-003 ClientIP
// middleware (right-most XFF, port stripped); if absent it falls back to the TCP
// peer with the port stripped, so a missing context value can neither collapse
// every caller into one key nor bypass the limit.
func rateLimiter(limiter *clientRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := auth.ClientIPFromContext(r.Context())
			if ip == "" {
				ip = r.RemoteAddr
				if host, _, err := net.SplitHostPort(ip); err == nil {
					ip = host
				}
			}
			strict := strings.HasPrefix(r.URL.Path, "/api/v1/auth/")
			if !limiter.allow(ip, strict) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// maxRequestBodyBytes caps every request body the BFF will buffer. Its payloads
// are small JSON (auth credentials, saved-analysis state, query parameters); 1
// MiB is generous for those yet blocks the multi-hundred-MB body that would
// otherwise spike memory toward the container limit (SEC-015).
const maxRequestBodyBytes int64 = 1 << 20 // 1 MiB

// bodyCap wraps each request body in http.MaxBytesReader so an oversized payload
// fails fast at decode time with *http.MaxBytesError — mapped to 413 by
// requestBindingErrorHandler — instead of buffering unbounded memory (SEC-015).
func bodyCap(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestBindingErrorHandler runs on param/body binding failure, before the
// handler. An oversized body becomes 413 (SEC-015); every other binding error
// becomes a generic 400 — never the raw error, which leaks parameter names and
// decoder internals (SEC-017). The real error is logged server-side.
func requestBindingErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeJSONError(w, http.StatusRequestEntityTooLarge, `{"code":"payload_too_large","message":"request body too large"}`)
		return
	}
	slog.Warn("request binding failed", "path", r.URL.Path, "error", err)
	writeJSONError(w, http.StatusBadRequest, `{"code":"invalid_request","message":"invalid request"}`)
}

// responseErrorHandler returns a generic 500; the real error is logged, never
// surfaced to the client (SEC-017).
func responseErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("response error", "path", r.URL.Path, "error", err)
	writeJSONError(w, http.StatusInternalServerError, `{"error":"internal error"}`)
}

func writeJSONError(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
