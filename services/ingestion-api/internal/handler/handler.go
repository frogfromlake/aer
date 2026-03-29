package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
)

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
	svc *core.IngestionService
}

// NewHandler creates a new Handler with the given ingestion service.
func NewHandler(svc *core.IngestionService) *Handler {
	return &Handler{svc: svc}
}

// Ingest handles POST /api/v1/ingest — accepts a batch of documents for bronze ingestion.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
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
		slog.Error("Ingestion failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		checks["postgres"] = err.Error()
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.svc.CheckMinio(r.Context()); err != nil {
		checks["minio"] = err.Error()
	} else {
		checks["minio"] = "ok"
	}

	if checks["postgres"] != "ok" || checks["minio"] != "ok" {
		writeJSON(w, http.StatusServiceUnavailable, checks)
		return
	}

	writeJSON(w, http.StatusOK, checks)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}
