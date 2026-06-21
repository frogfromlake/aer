package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/notify"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// genericInternalError is the opaque message returned to clients on any
// internal failure. Real error details are logged server-side only, so
// internal state (driver errors, SQL fragments, stack hints) never leaks
// across the trust boundary.
const genericInternalError = "internal server error"

// collectLanguagesForScope returns the distinct languages observed in
// `aer_gold.language_detections` for the requested scope and window.
// Phase 115: powers the cross-frame equivalence gate.
func (s *Server) collectLanguagesForScope(ctx context.Context, start, end time.Time, sources []string) ([]string, error) {
	return s.db.LanguagesForScope(ctx, start, end, sources)
}

// resolutionFromParam maps the OpenAPI-validated query enum onto the
// internal storage.Resolution constant. Unknown values fall back to the
// 5-minute baseline; the generated router rejects values outside the
// enum before the handler runs.
func resolutionFromParam(p *GetMetricsParamsResolution) storage.Resolution {
	if p == nil {
		return storage.ResolutionFiveMinute
	}
	switch *p {
	case GetMetricsParamsResolutionHourly:
		return storage.ResolutionHourly
	case GetMetricsParamsResolutionDaily:
		return storage.ResolutionDaily
	case GetMetricsParamsResolutionWeekly:
		return storage.ResolutionWeekly
	case GetMetricsParamsResolutionMonthly:
		return storage.ResolutionMonthly
	default:
		return storage.ResolutionFiveMinute
	}
}

// unionSourceParams merges the legacy single-source filter with the Phase 114
// comma-separated sourceIds parameter, deduplicating the result. An empty
// slice means no source filter — all sources are included.
func unionSourceParams(source, sourceIds *string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if source != nil {
		add(*source)
	}
	if sourceIds != nil {
		for _, src := range strings.Split(*sourceIds, ",") {
			add(src)
		}
	}
	return out
}

// Store abstracts the data access layer for testability.
type Store interface {
	Ping(ctx context.Context) error
	GetMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, error)
	// Phase 131: time-series with per-bucket sample stddev for the ±1σ band.
	GetMetricsWithSpread(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, error)
	GetNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, int64, error)
	// Phase 115: percentile-rank normalization, deviation labelling, cross-frame gate.
	GetPercentileNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, int64, error)
	CountLanguagesForSources(ctx context.Context, start, end time.Time, sources []string) (int, error)
	// Phase 151: all-time distinct-document count per source (NOT window-scoped),
	// for the Atmosphere dataset-overview readout (Probe.documentCount).
	GetDocumentTotalsBySource(ctx context.Context, sources []string) (map[string]int64, error)
	LanguagesForScope(ctx context.Context, start, end time.Time, sources []string) ([]string, error)
	CheckEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error)
	CheckNormalizationEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error)
	GetProbeEquivalence(ctx context.Context, start, end time.Time, sources []string) ([]storage.ProbeEquivalenceMetric, error)
	GetEquivalenceStatus(ctx context.Context, metricName string) (*storage.EquivalenceStatusRow, error)
	GetTemporalLeadLag(ctx context.Context, referenceSources, comparedSources []string, start, end time.Time, maxLagHours int) (storage.LeadLagResult, error)
	// Phase 125: generalised metric lead-lag (two metrics' hourly series, one scope).
	GetMetricLeadLag(ctx context.Context, sources []string, xMetric, yMetric string, start, end time.Time, maxLagHours int, metadataFilter *storage.MetadataFilter) (storage.LeadLagResult, error)
	// Phase 125: per-article N-metric matrix for parallel coordinates.
	GetParallelCoords(ctx context.Context, metrics, sources []string, start, end time.Time, maxPoints int, metadataFilter *storage.MetadataFilter) (storage.ParallelCoordResult, error)
	CheckBaselineExists(ctx context.Context, metricName string, source *string) (bool, error)
	CheckEquivalenceExists(ctx context.Context, metricName string) (bool, error)
	GetEntities(ctx context.Context, start, end time.Time, sources []string, label *string, limit int) ([]storage.EntityRow, error)
	GetLanguageDetections(ctx context.Context, start, end time.Time, sources []string, language *string, limit int) ([]storage.LanguageDetectionRow, error)
	GetAvailableMetrics(ctx context.Context, start, end time.Time) ([]storage.AvailableMetricRow, error)
	GetScopeAvailableMetrics(ctx context.Context, start, end time.Time, sources []string) (storage.ScopeMetricAvailability, error)
	GetMetricValidationStatus(ctx context.Context, metricName string) (string, error)
	GetMetricCulturalContextNotes(ctx context.Context, metricName string) (string, error)
	// Phase 102: view-mode query endpoints.
	GetMetricDistribution(ctx context.Context, metricName string, sources []string, start, end time.Time, bins int, metadataFilter *storage.MetadataFilter) (storage.DistributionResult, error)
	GetMetricHeatmap(ctx context.Context, metricName string, sources []string, xDim, yDim storage.HeatmapDimension, start, end time.Time) ([]storage.HeatmapCell, error)
	GetMetricCorrelation(ctx context.Context, metricNames []string, sources []string, start, end time.Time, metadataFilter *storage.MetadataFilter) (storage.CorrelationResult, error)
	GetEntityCoOccurrence(ctx context.Context, sources []string, start, end time.Time, topN int, viewerLanguage string, nodeMetric string, minWeight int, nsOverlay bool, colorMetric string) (storage.CoOccurrenceResult, error)
	// Phase 131: paired-metric scatter over aer_gold.metrics (visual-channel binding).
	GetMetricScatter(ctx context.Context, xMetric, yMetric string, sizeMetric, colorMetric *string, sources []string, start, end time.Time, maxPoints int, metadataFilter *storage.MetadataFilter) (storage.ScatterResult, error)
	// Phase 120: BERTopic topic-distribution endpoint over aer_gold.topic_assignments.
	GetTopicDistribution(ctx context.Context, params storage.TopicDistributionParams) ([]storage.TopicDistributionRow, error)
	// Phase 103b: silver-aggregation endpoints over aer_silver.documents.
	GetSilverDistribution(ctx context.Context, field string, source string, start, end time.Time, bins int) (storage.DistributionResult, error)
	GetSilverHeatmap(ctx context.Context, kind storage.SilverAggregationKind, source string, start, end time.Time) ([]storage.HeatmapCell, string, string, error)
	GetSilverCorrelation(ctx context.Context, source string, start, end time.Time) (storage.SilverCorrelationResult, error)
	// Phase 122f: metadata-coverage matrix over aer_gold.metadata_coverage.
	GetMetadataCoverage(ctx context.Context, sources []string) ([]storage.MetadataCoverageCell, error)
	// Task A: per-(source, coverage-field) distinct-value count for the dossier
	// "constant → no signal" marker (over article_metadata + metrics).
	GetFieldCardinality(ctx context.Context, sources []string) (map[storage.FieldKey]storage.FieldCardinality, error)
	// Task C: corpus-wide per-field extraction status for the Reflection
	// "metadata fields" surface (aggregated over all sources).
	GetGlobalFieldStats(ctx context.Context) ([]storage.GlobalFieldStat, error)
	// Phase 133: categorical metadata distribution + per-scope availability gate
	// over aer_gold.article_metadata.
	GetCategoricalDistribution(ctx context.Context, field string, sources []string, start, end time.Time, topN int, metadataFilter *storage.MetadataFilter) (storage.CategoricalDistributionResult, error)
	GetScopeAvailableMetadata(ctx context.Context, start, end time.Time, sources []string) (storage.ScopeMetadataAvailability, error)
	// Phase 125: cross-tab of a categorical field × a numeric metric (article_id JOIN).
	GetCrossTab(ctx context.Context, field, metric string, sources []string, start, end time.Time, topN int, metadataFilter *storage.MetadataFilter) (storage.CrossTabResult, error)
	// Phase 125: alluvial flow across an ordered chain of categorical fields.
	GetSankey(ctx context.Context, fields, sources []string, start, end time.Time, topN int) (storage.SankeyResult, error)
	// Phase 122d.0: Silent-Edit Observability — aggregation + per-article
	// chain over aer_gold.article_revisions.
	GetRevisionActivity(ctx context.Context, sources []string, start, end time.Time, resolution storage.RevisionActivityResolution) ([]storage.RevisionActivityCell, error)
	// Phase 122d.3: Silent-Edit Discourse Shift — re-extraction deltas
	// aggregated by (source, bucket).
	GetRevisionDiscourseShift(ctx context.Context, sources []string, start, end time.Time, resolution storage.RevisionActivityResolution) ([]storage.RevisionDiscourseShiftCell, error)
	GetRevisionEditClusters(ctx context.Context, sources []string, start, end time.Time, resolution storage.RevisionActivityResolution, minSources int) ([]storage.RevisionEditClusterRow, error)
	GetArticleRevisions(ctx context.Context, articleID string) ([]storage.ArticleRevisionRow, error)
	// Phase 122d.1: Silent-Edit Diff Substance + Drilldown.
	GetArticleRevisionDiff(ctx context.Context, articleID string, revisionIndex int) (*storage.ArticleRevisionDiffRow, error)
	GetRevisionsArticles(ctx context.Context, filter storage.RevisionsArticlesFilter) ([]storage.RevisionArticleRow, error)
}

// SourceLister abstracts the source-metadata read path so the handler
// does not care whether its backing store is Postgres, an in-memory fake
// (for tests), or a future alternative. A nil value is valid — the
// /sources endpoint will then return 500, which mirrors the behavior of
// a misconfigured stack where the read path was never wired up.
type SourceLister interface {
	List(ctx context.Context) ([]config.SourceEntry, error)
}

// Server implements the generated StrictServerInterface.
type Server struct {
	db                  Store
	provenance          config.MetricProvenanceMap
	sources             SourceLister
	catalog             config.ContentCatalog
	probes              config.ProbeRegistry
	dossier             DossierStore
	articles            ArticleQuerier
	silver              SilverFetcher
	kAnonymityThreshold int
	// languageManifest gates the `?language=` query parameter (Phase 118a /
	// ADR-024). Nil is permitted only in legacy test constructors that do
	// not exercise language-validated endpoints — callers that hit a
	// language gate with a nil manifest get the same 500 path as a
	// misconfigured stack.
	languageManifest *config.LanguageManifest

	// Access control (Phase 134 / ADR-040). Nil in legacy test constructors
	// that do not exercise the /auth/* endpoints.
	authBackend AuthBackend
	authConfig  AuthConfig
	mailer      notify.LinkSender
	// emailEnabled is true when mailer is a real transactional-email relay
	// (Phase 153). It drives the `delivered` flag on admin action-link responses
	// so the LogSender fallback is never reported as a real send.
	emailEnabled bool
	// WebAuthn / passkeys (Phase 134 / ADR-040). Nil when WebAuthn is not wired.
	webAuthn        *webauthn.WebAuthn
	webAuthnBackend WebAuthnBackend
	// loginThrottle is the brute-force backoff for /auth/login (security review
	// M-3). Always non-nil (initialised in NewServer).
	loginThrottle *auth.LoginThrottle
	// Saved analyses (Phase 135). Nil in legacy test constructors.
	analysesBackend AnalysesBackend
}

// LoginThrottle exposes the throttle so main can sweep it on the cleanup tick.
func (s *Server) LoginThrottle() *auth.LoginThrottle { return s.loginThrottle }

// WebAuthnBackend is the passkey persistence surface, satisfied by
// *storage.WebAuthnStore.
type WebAuthnBackend interface {
	CredentialsByUser(ctx context.Context, userID string) ([]webauthn.Credential, error)
	HasCredentials(ctx context.Context, userID string) (bool, error)
	SaveCredential(ctx context.Context, userID string, cred *webauthn.Credential, name string) (storage.CredentialMeta, error)
	UpdateCredential(ctx context.Context, cred *webauthn.Credential) error
	ListCredentialMeta(ctx context.Context, userID string) ([]storage.CredentialMeta, error)
	DeleteCredential(ctx context.Context, userID, credentialRowID string) (bool, error)
	SaveCeremony(ctx context.Context, userID, purpose string, sd *webauthn.SessionData, expires time.Time) error
	ConsumeCeremony(ctx context.Context, userID, purpose string) (*webauthn.SessionData, error)
}

// AuthBackend is the auth write/read surface the /auth handlers depend on,
// satisfied by *storage.AuthStore. An interface keeps the handlers testable.
type AuthBackend interface {
	GetUserByEmail(ctx context.Context, email string) (*storage.AuthUser, error)
	GetUserByID(ctx context.Context, id string) (*storage.AuthUser, error)
	CreateSession(ctx context.Context, idHash, userID string, idleExp, absExp time.Time, userAgent string) error
	ValidateAndTouchSession(ctx context.Context, idHash string, idleTTL time.Duration) (*auth.Identity, error)
	RevokeSession(ctx context.Context, idHash string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
	RevokeOtherUserSessions(ctx context.Context, userID, keepIDHash string) error
	CreateToken(ctx context.Context, userID, purpose, tokenHash string, exp time.Time) error
	ConsumeToken(ctx context.Context, tokenHash, purpose string) (string, error)
	// Transactional token flows (SEC-078): consume + apply co-commit so a
	// partial failure cannot burn the single-use token.
	ConsumeTokenAndActivate(ctx context.Context, tokenHash, passwordHash string) (string, error)
	ConsumeTokenAndResetPassword(ctx context.Context, tokenHash, passwordHash string) (string, error)
	ActivateUser(ctx context.Context, id, passwordHash string) error
	UpdateUserPassword(ctx context.Context, id, passwordHash string) error
	// Admin (Phase 134 / ADR-040).
	CreateInvitedUser(ctx context.Context, email, role string) (string, error)
	ListUsers(ctx context.Context) ([]storage.AdminUserRow, error)
	SetUserStatus(ctx context.Context, id, status string) (bool, error)
	// DSGVO (Phase 134 / ADR-040).
	ExportUser(ctx context.Context, id string) (*storage.UserExport, error)
	DeleteUser(ctx context.Context, id string) (bool, error)
}

// AuthConfig carries the cookie + session + hashing parameters the auth
// handlers need (derived from config.Config in main).
type AuthConfig struct {
	CookieName      string
	SecureCookies   bool
	SessionIdle     time.Duration
	SessionAbsolute time.Duration
	Argon2          auth.Argon2Params
	ResetTTL        time.Duration
	InviteTTL       time.Duration
	PublicBaseURL   string
}

// ServerOptions carries the optional, Phase 101-introduced dependencies
// (dossier/articles/silver). They are optional because the existing test
// suite constructs Server with only the legacy dependencies.
type ServerOptions struct {
	Dossier             DossierStore
	Articles            ArticleQuerier
	Silver              SilverFetcher
	KAnonymityThreshold int
	LanguageManifest    *config.LanguageManifest
	// Access control (Phase 134 / ADR-040).
	Auth       AuthBackend
	AuthConfig AuthConfig
	Mailer     notify.LinkSender
	// EmailEnabled marks Mailer as a real relay (Phase 153). Leave false for the
	// LogSender so admin responses report delivered=false (manual delivery).
	EmailEnabled bool
	WebAuthn     *webauthn.WebAuthn
	WebAuthnBE   WebAuthnBackend
	Analyses     AnalysesBackend
}

// NewServer creates a new API server instance with only the legacy
// dependencies. Tests that do not exercise the Phase 101 endpoints use
// this constructor unchanged.
func NewServer(db Store, provenance config.MetricProvenanceMap, sources SourceLister, catalog config.ContentCatalog, probes config.ProbeRegistry) *Server {
	return &Server{
		db: db, provenance: provenance, sources: sources, catalog: catalog, probes: probes,
		// Brute-force throttle: 5 free attempts, then 1s→…→5m exponential
		// backoff, auto-resetting after 15m idle (security review M-3).
		loginThrottle: auth.NewLoginThrottle(5, time.Second, 5*time.Minute, 15*time.Minute),
	}
}

// NewServerWithOptions wires the Phase 101 endpoints alongside the
// legacy dependencies. The cmd/server entrypoint uses this form once the
// Postgres dossier store and MinIO Silver store have been initialised.
func NewServerWithOptions(db Store, provenance config.MetricProvenanceMap, sources SourceLister, catalog config.ContentCatalog, probes config.ProbeRegistry, opts ServerOptions) *Server {
	s := NewServer(db, provenance, sources, catalog, probes)
	s.dossier = opts.Dossier
	s.articles = opts.Articles
	s.silver = opts.Silver
	s.kAnonymityThreshold = opts.KAnonymityThreshold
	if s.kAnonymityThreshold <= 0 {
		s.kAnonymityThreshold = 10
	}
	s.languageManifest = opts.LanguageManifest
	s.authBackend = opts.Auth
	s.authConfig = opts.AuthConfig
	s.mailer = opts.Mailer
	s.emailEnabled = opts.EmailEnabled
	s.webAuthn = opts.WebAuthn
	s.webAuthnBackend = opts.WebAuthnBE
	s.analysesBackend = opts.Analyses
	return s
}

// GetHealthz handles GET /healthz — liveness probe, always returns 200 if the process is alive.
func (s *Server) GetHealthz(_ context.Context, _ GetHealthzRequestObject) (GetHealthzResponseObject, error) {
	return GetHealthz200JSONResponse{"status": "alive"}, nil
}

// GetReadyz handles GET /readyz — readiness probe, returns 200 only if ClickHouse is reachable.
func (s *Server) GetReadyz(ctx context.Context, _ GetReadyzRequestObject) (GetReadyzResponseObject, error) {
	if err := s.db.Ping(ctx); err != nil {
		slog.Error("handler failure", "op", "GetReadyz", "error", err)
		return GetReadyz503JSONResponse{"clickhouse": "unavailable"}, nil
	}
	return GetReadyz200JSONResponse{"clickhouse": "ok"}, nil
}

// crossFrameAnchor is the canonical pointer into the methodological surface
// (WP-004 §5.2) used by the Phase-115 cross-frame equivalence refusal.
const crossFrameAnchor = "WP-004#section-5.2"

// crossFrameGateID matches RefusalPayloadGate's metric_equivalence value.
const crossFrameGateID = "metric_equivalence"

// crossFrameRefusalAlternatives are the three concrete user-actionable
// fall-backs surfaced by the Phase-115 refusal payload (Brief §7.4).
var crossFrameRefusalAlternatives = []string{
	"drop normalization to Level 1 (temporal patterns only)",
	"constrain scope to one cultural frame (single language)",
	"use deviation labelling instead of an absolute claim",
}

// crossFrameRefusalMessage is the human-readable summary attached to the
// 400 RefusalPayload when a cross-frame normalization request is refused.
const crossFrameRefusalMessage = "cross-cultural normalization requires validated metric equivalence across the resolved language set; granted out-of-band via WP-004 §5.2"

// invalidLanguageGateID matches RefusalPayloadGate's invalid_language value
// (Phase 118a / ADR-024).
const invalidLanguageGateID = "invalid_language"

// invalidLanguageAnchor points into the methodological surface entry that
// describes the Capability Manifest workflow (Operations Playbook section
// "Editing the Language Capability Manifest"). It is intentionally not a
// working-paper anchor — the gate is engineering-procedural, not
// methodological.
const invalidLanguageAnchor = "ops/playbook#language-capability-manifest"

// validateLanguageQueryParam returns nil if the manifest declares the given
// language code (or if no language was supplied / no manifest is wired).
// Otherwise it returns the structured Error body for the invalid_language
// gate, with `alternatives` set to the manifest's sorted language codes.
//
// Phase 118a / ADR-024: replaces any hand-coded language allowlist in BFF
// handlers. Every endpoint that takes a `?language=` query parameter must
// route through this helper before issuing a query.
func (s *Server) validateLanguageQueryParam(raw *string) (errBody *struct {
	Message            string
	Gate               string
	WorkingPaperAnchor string
	Alternatives       []string
}, ok bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	if s.languageManifest == nil {
		// No manifest wired — the validator cannot run. Permit the request
		// rather than 500, matching the legacy behaviour for tests that
		// construct Server without the manifest dependency.
		return nil, true
	}
	if s.languageManifest.IsKnown(*raw) {
		return nil, true
	}
	codes := s.languageManifest.LanguageCodes()
	return &struct {
		Message            string
		Gate               string
		WorkingPaperAnchor string
		Alternatives       []string
	}{
		Message: fmt.Sprintf(
			"unknown language %q; the Language Capability Manifest declares: %v",
			*raw, codes,
		),
		Gate:               invalidLanguageGateID,
		WorkingPaperAnchor: invalidLanguageAnchor,
		Alternatives:       codes,
	}, false
}

// crossFrameRefusal constructs the structured 400 body for the
// metric_equivalence gate. The fields piggy-back on the Error schema's
// optional refusal extensions so existing callers that decode 400 as
// {message: string} still work.
func crossFrameRefusal() GetMetrics400JSONResponse {
	gate := crossFrameGateID
	anchor := crossFrameAnchor
	alts := append([]string(nil), crossFrameRefusalAlternatives...)
	return GetMetrics400JSONResponse{
		Message:            crossFrameRefusalMessage,
		Gate:               &gate,
		WorkingPaperAnchor: &anchor,
		Alternatives:       &alts,
	}
}

// parseMetadataFilter builds the Phase-125a faceting restriction from the two
// optional query params (metadataFilterField + metadataFilterValue). Both must be
// present and non-blank; otherwise it returns nil (no faceting). Trimmed so a
// stray-whitespace value never silently empties a facet.
func parseMetadataFilter(field, value *string) *storage.MetadataFilter {
	if field == nil || value == nil {
		return nil
	}
	f := strings.TrimSpace(*field)
	v := strings.TrimSpace(*value)
	if f == "" || v == "" {
		return nil
	}
	return &storage.MetadataFilter{Field: f, Value: v}
}

// crossFrameRefusalFields carries the structured 400 refusal payload shared by
// every Phase-125 cross-frame equivalence gate. Each handler returns its own
// typed *400JSONResponse, so the gate hands back these fields rather than a
// concrete response type; the caller maps them onto its response struct.
type crossFrameRefusalFields struct {
	Message            string
	Gate               *string
	WorkingPaperAnchor *string
	Alternatives       *[]string
}

// crossFrameGate runs the Phase-125 cross-frame equivalence gate for a set of
// metrics over a resolved scope. Returns:
//   - (refusal, nil) — the scope spans >1 language and at least one metric
//     lacks a cross-cultural equivalence grant; the caller returns its typed
//     400 built from refusal's fields (Level-1 alternative: stay in one frame).
//   - (nil, err)     — an internal error occurred; the caller returns 500.
//   - (nil, nil)     — the gate passed (single frame, or all metrics granted).
//
// Extracted in Phase 125a from four identical copies (GetMetricCorrelation,
// GetCorrelationLeadLag, GetMetricParallelCoords, GetMetadataCrossTab).
func (s *Server) crossFrameGate(ctx context.Context, metrics []string, sources []string, start, end time.Time) (*crossFrameRefusalFields, error) {
	nLangs, err := s.db.CountLanguagesForSources(ctx, start, end, sources)
	if err != nil {
		return nil, err
	}
	if nLangs <= 1 {
		return nil, nil
	}
	languages, err := s.collectLanguagesForScope(ctx, start, end, sources)
	if err != nil {
		return nil, err
	}
	for _, m := range metrics {
		granted, err := s.db.CheckNormalizationEquivalenceForLanguages(ctx, m, languages)
		if err != nil {
			return nil, err
		}
		if !granted {
			gate := crossFrameGateID
			anchor := crossFrameAnchor
			alts := append([]string(nil), crossFrameRefusalAlternatives...)
			return &crossFrameRefusalFields{
				Message:            crossFrameRefusalMessage,
				Gate:               &gate,
				WorkingPaperAnchor: &anchor,
				Alternatives:       &alts,
			}, nil
		}
	}
	return nil, nil
}
