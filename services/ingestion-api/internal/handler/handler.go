package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
)

// genericInternalError is the opaque message returned to clients on any
// internal failure. Real error details are logged server-side only, so
// internal state (driver errors, SQL fragments, stack hints) never leaks
// across the trust boundary.
const genericInternalError = "internal server error"

// genericBadRequestBody is the opaque message returned when the request body
// cannot be parsed as JSON. We do not leak the decoder error because it can
// echo caller-controlled bytes back into logs and response.
const genericBadRequestBody = "invalid request body"

// Server implements StrictServerInterface and holds handler dependencies.
type Server struct {
	svc          *core.IngestionService
	maxBodyBytes int64
}

// NewServer creates a Server. A non-positive maxBodyBytes disables the body cap (tests only).
func NewServer(svc *core.IngestionService, maxBodyBytes int64) *Server {
	return &Server{svc: svc, maxBodyBytes: maxBodyBytes}
}

// BodyLimitMiddleware wraps the request body in MaxBytesReader so the strict
// handler's JSON decoder returns a detectable *http.MaxBytesError on oversized input.
func (s *Server) BodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.maxBodyBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// RequestErrorHandler is used as StrictHTTPServerOptions.RequestErrorHandlerFunc.
// It maps *http.MaxBytesError to 413 and all other decode failures to a generic 400
// so that neither decoder internals nor caller-controlled bytes reach the response.
func RequestErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		slog.Warn("ingest body exceeds size limit", "op", "RequestErrorHandler", "limit_bytes", maxErr.Limit)
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body exceeds size limit"})
		return
	}
	slog.Warn("request body decode failed", "op", "RequestErrorHandler", "error", err)
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": genericBadRequestBody})
}

func (s *Server) IngestDocuments(ctx context.Context, request IngestDocumentsRequestObject) (IngestDocumentsResponseObject, error) {
	body := request.Body

	if body.SourceId <= 0 {
		return IngestDocuments400JSONResponse{Error: "source_id must be a positive integer"}, nil
	}
	if len(body.Documents) == 0 {
		return IngestDocuments400JSONResponse{Error: "documents array must not be empty"}, nil
	}

	docs := make([]core.Document, 0, len(body.Documents))
	for _, d := range body.Documents {
		if d.Key == "" {
			return IngestDocuments400JSONResponse{Error: "each document must have a non-empty key"}, nil
		}
		raw, err := json.Marshal(d.Data)
		if err != nil {
			slog.Error("handler failure", "op", "IngestDocuments.marshalData", "key", d.Key, "error", err)
			return IngestDocuments500JSONResponse{Error: genericInternalError}, nil
		}
		docs = append(docs, core.Document{Key: d.Key, Data: string(raw)})
	}

	result, err := s.svc.IngestDocuments(ctx, int(body.SourceId), docs)
	if err != nil {
		slog.Error("handler failure", "op", "IngestDocuments", "error", err)
		return IngestDocuments500JSONResponse{Error: genericInternalError}, nil
	}

	r := IngestResult{
		JobId:    int32(result.JobID),
		Uploaded: int32(result.Accepted),
		Failed:   int32(result.Failed),
	}
	if result.Failed > 0 {
		return IngestDocuments207JSONResponse(r), nil
	}
	return IngestDocuments200JSONResponse(r), nil
}

func (s *Server) GetHealthz(_ context.Context, _ GetHealthzRequestObject) (GetHealthzResponseObject, error) {
	return GetHealthz200JSONResponse(HealthStatus{"status": "alive"}), nil
}

func (s *Server) GetReadyz(ctx context.Context, _ GetReadyzRequestObject) (GetReadyzResponseObject, error) {
	checks := HealthStatus{}

	if err := s.svc.CheckPostgres(ctx); err != nil {
		slog.Error("handler failure", "op", "GetReadyz.CheckPostgres", "error", err)
		checks["postgres"] = "unavailable"
	} else {
		checks["postgres"] = "ok"
	}

	if err := s.svc.CheckMinio(ctx); err != nil {
		slog.Error("handler failure", "op", "GetReadyz.CheckMinio", "error", err)
		checks["minio"] = "unavailable"
	} else {
		checks["minio"] = "ok"
	}

	if checks["postgres"] != "ok" || checks["minio"] != "ok" {
		return GetReadyz503JSONResponse(checks), nil
	}
	return GetReadyz200JSONResponse(checks), nil
}

func (s *Server) GetSourceByName(ctx context.Context, request GetSourceByNameRequestObject) (GetSourceByNameResponseObject, error) {
	id, sourceName, err := s.svc.LookupSource(ctx, request.Params.Name)
	if err != nil {
		slog.Info("source lookup miss", "op", "GetSourceByName.LookupSource", "name", request.Params.Name, "error", err)
		return GetSourceByName404JSONResponse{Error: "source not found"}, nil
	}

	return GetSourceByName200JSONResponse(SourceLookup{
		Id:   int32(id),
		Name: sourceName,
	}), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}
