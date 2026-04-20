package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
)

// serialUploads forces the IngestionService to process documents one at a
// time so handler tests stay deterministic. The core package owns the
// concurrent-upload invariant; see core/service_test.go for the parallel path.
const serialUploads = 1

// --- Mocks for core.MetadataStore and core.ObjectStore ---
//
// These live in the handler package (not imported from core_test.go) because
// internal test files cannot be shared across packages. The surface is small
// enough that duplication is cheaper than exposing the core mocks.

type stubDB struct {
	createJobErr   error
	logDocumentErr error
}

func (s *stubDB) CreateIngestionJob(_ context.Context, _ int) (int, error) {
	if s.createJobErr != nil {
		return 0, s.createJobErr
	}
	return 1, nil
}
func (s *stubDB) UpdateJobStatus(_ context.Context, _ int, _ string) error { return nil }
func (s *stubDB) LogDocument(_ context.Context, _ int, _ string, _ string) error {
	return s.logDocumentErr
}
func (s *stubDB) UpdateDocumentStatus(_ context.Context, _, _ string) error { return nil }
func (s *stubDB) GetSourceByName(_ context.Context, _ string) (int, string, error) {
	return 1, "test", nil
}
func (s *stubDB) Ping(_ context.Context) error { return nil }

type stubMinio struct {
	uploadErr error
}

func (s *stubMinio) UploadJSON(_ context.Context, _, _, _ string) error { return s.uploadErr }
func (s *stubMinio) BucketExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// newTestRouter creates a chi router wired with the strict handler so tests
// exercise the full HTTP stack, including body-size enforcement and the
// custom request-error handler. maxBodyBytes <= 0 disables the body cap.
func newTestRouter(t *testing.T, db *stubDB, mio *stubMinio, maxBodyBytes int64) http.Handler {
	t.Helper()
	if db == nil {
		db = &stubDB{}
	}
	if mio == nil {
		mio = &stubMinio{}
	}
	svc := core.NewIngestionService(db, mio, "bronze", serialUploads)
	srv := NewServer(svc, maxBodyBytes)

	strictH := NewStrictHandlerWithOptions(srv, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: RequestErrorHandler,
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			slog.Error("response encoding failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		},
	})

	r := chi.NewRouter()
	r.Use(srv.BodyLimitMiddleware)
	HandlerFromMuxWithBaseURL(strictH, r, "/api/v1")
	return r
}

// --- Ingest handler: error-leak guard (Phase 82) ---

// TestIngest_InvalidJSONDoesNotLeakDecoderError asserts that a malformed
// request body is answered with a generic 400 and that the decoder's error
// text (which can echo caller-controlled bytes) never reaches the response.
func TestIngest_InvalidJSONDoesNotLeakDecoderError(t *testing.T) {
	r := newTestRouter(t, nil, nil, 16<<20)

	// Body includes a distinctive, fake-but-plausible error fragment.
	// If the decoder error ever reaches the response, the test catches it.
	body := strings.NewReader(`{"source_id": 1, "documents": [UNEXPECTED_TOKEN_SENTINEL`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp["error"] != genericBadRequestBody {
		t.Errorf("expected generic body-error message, got %q", resp["error"])
	}
	if strings.Contains(rr.Body.String(), "UNEXPECTED_TOKEN_SENTINEL") {
		t.Error("response leaks caller-controlled bytes from decoder error")
	}
	if strings.Contains(rr.Body.String(), "invalid character") {
		t.Error("response leaks Go decoder internal error text")
	}
}

// TestIngest_ServiceErrorReturnsGenericMessage asserts that a service-level
// failure (e.g. PostgreSQL unreachable) produces a 500 with a generic
// message — not the raw error text.
func TestIngest_ServiceErrorReturnsGenericMessage(t *testing.T) {
	db := &stubDB{createJobErr: errors.New("postgres: connection refused to host 10.0.0.42:5432")}
	r := newTestRouter(t, db, nil, 16<<20)

	body := strings.NewReader(`{"source_id": 1, "documents": [{"key": "a.json", "data": {}}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp["error"] != genericInternalError {
		t.Errorf("expected generic internal-error message, got %q", resp["error"])
	}
	if strings.Contains(rr.Body.String(), "10.0.0.42") {
		t.Error("response leaks internal address from service error")
	}
	if strings.Contains(rr.Body.String(), "connection refused") {
		t.Error("response leaks internal error text")
	}
}

// --- Ingest handler: body-size limit (Phase 82) ---

// TestIngest_OversizedBodyReturns413 asserts that a request body exceeding
// the configured cap is rejected with 413 before the JSON decoder gets a
// chance to allocate it.
func TestIngest_OversizedBodyReturns413(t *testing.T) {
	const limit int64 = 512

	r := newTestRouter(t, nil, nil, limit)

	// Build a valid JSON envelope whose documents field is a single string
	// padded well beyond the limit.
	bigData := strings.Repeat("A", int(limit)*4)
	body := strings.NewReader(`{"source_id": 1, "documents": [{"key": "a.json", "data": "` + bigData + `"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected non-empty error message for oversized body")
	}
}

// TestIngest_BodyAtLimitStillReadsFully asserts that a request exactly at
// the configured cap is processed normally (off-by-one guard on the limit).
func TestIngest_BodyAtLimitStillReadsFully(t *testing.T) {
	body := `{"source_id": 1, "documents": [{"key": "a.json", "data": {}}]}`
	limit := int64(len(body))

	r := newTestRouter(t, nil, nil, limit)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(body))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 at exact limit, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Ingest handler: happy path ---

func TestIngest_HappyPath(t *testing.T) {
	r := newTestRouter(t, nil, nil, 16<<20)

	body := strings.NewReader(`{"source_id": 7, "documents": [{"key": "a.json", "data": {"id":1}}, {"key": "b.json", "data": {"id":2}}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result IngestResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a valid IngestResult: %v", err)
	}
	if result.Uploaded != 2 {
		t.Errorf("expected 2 uploaded, got %d", result.Uploaded)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
}

// TestIngest_PartialFailureReturns207 asserts that when some documents fail
// to upload, the handler returns 207 Multi-Status with the failure count in
// the body.
func TestIngest_PartialFailureReturns207(t *testing.T) {
	mio := &stubMinio{uploadErr: errors.New("simulated upload failure")}
	r := newTestRouter(t, nil, mio, 16<<20)

	body := strings.NewReader(`{"source_id": 1, "documents": [{"key": "a.json", "data": {}}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusMultiStatus {
		t.Fatalf("expected 207, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Validation branches ---

func TestIngest_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing source_id", `{"documents": [{"key": "a.json", "data": {}}]}`},
		{"negative source_id", `{"source_id": -1, "documents": [{"key": "a.json", "data": {}}]}`},
		{"empty documents", `{"source_id": 1, "documents": []}`},
		{"empty key", `{"source_id": 1, "documents": [{"key": "", "data": {}}]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRouter(t, nil, nil, 16<<20)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}

			// Each validation branch returns a specific human message —
			// these are not decoder error text, so they do not violate the
			// Phase-82 no-leak rule. Just sanity-check the envelope.
			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if resp["error"] == "" {
				t.Error("expected non-empty error message")
			}
		})
	}
}

// Sanity check: the stdlib io.EOF on an empty body still maps to the generic
// decoder-error path (not a panic).
func TestIngest_EmptyBodyIsRejected(t *testing.T) {
	r := newTestRouter(t, nil, nil, 16<<20)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", http.NoBody)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	// The error must be the generic decoder message, not raw io.EOF text.
	if strings.Contains(rr.Body.String(), io.EOF.Error()) {
		t.Error("response leaks io.EOF from decoder")
	}
}
