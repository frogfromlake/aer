package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
)

// genericInternalError is the opaque message returned to clients on any
// internal failure. Real error details are logged server-side only, so
// internal state (driver errors, SQL fragments, stack hints) never leaks
// across the trust boundary. Mirrors the Phase 75 pattern established in
// the bff-api handler package.
const genericInternalError = "internal server error"

// genericBadRequestBody is the opaque message returned when the request body
// cannot be parsed as JSON. We do not leak the decoder error because it can
// echo caller-controlled bytes back into logs and response.
const genericBadRequestBody = "invalid request body"

// IngestRequest is the expected JSON body for POST /api/v1/ingest.
type IngestRequest struct {
	SourceID  int `json:"source_id"`
	Documents []struct {
		Key  string          `json:"key"`
		Data json.RawMessage `json:"data"`
	} `json:"documents"`
}

// Handler holds HTTP handler dependencies.
type Handler struct {
	svc          *core.IngestionService
	maxBodyBytes int64
}

// NewHandler creates a new Handler with the given ingestion service and a
// hard cap on the size of the request body accepted by Ingest. A non-positive
// maxBodyBytes disables the limit, which is only intended for tests.
func NewHandler(svc *core.IngestionService, maxBodyBytes int64) *Handler {
	return &Handler{svc: svc, maxBodyBytes: maxBodyBytes}
}

// Ingest handles POST /api/v1/ingest — accepts a batch of documents for bronze ingestion.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	if h.maxBodyBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	}

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			slog.Warn("ingest body exceeds size limit",
				"op", "Ingest.Decode",
				"limit_bytes", maxErr.Limit,
			)
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body exceeds size limit"})
			return
		}
		slog.Warn("ingest body decode failed", "op", "Ingest.Decode", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": genericBadRequestBody})
		return
	}

	if req.SourceID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source_id must be a positive integer"})
		return
	}
	if len(req.Documents) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "documents array must not be empty"})
		return
	}

	docs := make([]core.Document, 0, len(req.Documents))
	for _, d := range req.Documents {
		if d.Key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "each document must have a non-empty key"})
			return
		}
		docs = append(docs, core.Document{
			Key:  d.Key,
			Data: string(d.Data),
		})
	}

	result, err := h.svc.IngestDocuments(r.Context(), req.SourceID, docs)
	if err != nil {
		slog.Error("handler failure", "op", "Ingest.IngestDocuments", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": genericInternalError})
		return
	}

	status := http.StatusOK
	if result.Failed > 0 {
		status = http.StatusMultiStatus // 207 — partial success
	}
	writeJSON(w, status, result)
}

// Healthz is a liveness probe — returns 200 if the process is alive.
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// Readyz is a readiness probe — returns 200 only if Postgres and MinIO are reachable.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}

	if err := h.svc.CheckPostgres(r.Context()); err != nil {
		slog.Error("handler failure", "op", "Readyz.CheckPostgres", "error", err)
		checks["postgres"] = "unavailable"
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.svc.CheckMinio(r.Context()); err != nil {
		slog.Error("handler failure", "op", "Readyz.CheckMinio", "error", err)
		checks["minio"] = "unavailable"
	} else {
		checks["minio"] = "ok"
	}

	if checks["postgres"] != "ok" || checks["minio"] != "ok" {
		writeJSON(w, http.StatusServiceUnavailable, checks)
		return
	}

	writeJSON(w, http.StatusOK, checks)
}

// GetSources handles GET /api/v1/sources?name=<name> — looks up a source by name.
func (h *Handler) GetSources(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter 'name' is required"})
		return
	}

	id, sourceName, err := h.svc.LookupSource(r.Context(), name)
	if err != nil {
		slog.Info("source lookup miss", "op", "GetSources.LookupSource", "name", name, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "name": sourceName})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}
