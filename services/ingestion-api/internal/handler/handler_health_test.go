package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
)

// newTestServer builds a Server over the package stubs so the readiness and
// source-lookup handlers can be exercised at the method level (the full HTTP
// stack is covered by the ingest tests in handler_test.go).
func newTestServer(db *stubDB, mio *stubMinio) *Server {
	if db == nil {
		db = &stubDB{}
	}
	if mio == nil {
		mio = &stubMinio{}
	}
	svc := core.NewIngestionService(db, mio, "bronze", serialUploads)
	return NewServer(svc, 0)
}

func TestGetHealthz_ReportsAlive(t *testing.T) {
	resp, err := newTestServer(nil, nil).GetHealthz(context.Background(), GetHealthzRequestObject{})
	if err != nil {
		t.Fatalf("GetHealthz error: %v", err)
	}
	got, ok := resp.(GetHealthz200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetHealthz200JSONResponse", resp)
	}
	if got["status"] != "alive" {
		t.Errorf("status = %q, want alive", got["status"])
	}
}

func TestGetReadyz_AllDependenciesOK(t *testing.T) {
	resp, err := newTestServer(nil, nil).GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("GetReadyz error: %v", err)
	}
	got, ok := resp.(GetReadyz200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetReadyz200JSONResponse", resp)
	}
	if got["postgres"] != "ok" || got["minio"] != "ok" {
		t.Errorf("checks = %v, want both ok", got)
	}
}

func TestGetReadyz_PostgresDownReturns503(t *testing.T) {
	srv := newTestServer(&stubDB{pingErr: errors.New("connection refused")}, nil)
	resp, err := srv.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("GetReadyz error: %v", err)
	}
	got, ok := resp.(GetReadyz503JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetReadyz503JSONResponse", resp)
	}
	if got["postgres"] != "unavailable" {
		t.Errorf("postgres = %q, want unavailable", got["postgres"])
	}
}

func TestGetReadyz_MinioDownReturns503(t *testing.T) {
	srv := newTestServer(nil, &stubMinio{bucketExistsErr: errors.New("minio unreachable")})
	resp, err := srv.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("GetReadyz error: %v", err)
	}
	got, ok := resp.(GetReadyz503JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetReadyz503JSONResponse", resp)
	}
	if got["minio"] != "unavailable" {
		t.Errorf("minio = %q, want unavailable", got["minio"])
	}
}

func TestGetSourceByName_Found(t *testing.T) {
	srv := newTestServer(nil, nil)
	resp, err := srv.GetSourceByName(context.Background(), GetSourceByNameRequestObject{
		Params: GetSourceByNameParams{Name: "test"},
	})
	if err != nil {
		t.Fatalf("GetSourceByName error: %v", err)
	}
	got, ok := resp.(GetSourceByName200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetSourceByName200JSONResponse", resp)
	}
	if got.ID != 1 || got.Name != "test" {
		t.Errorf("source = {%d, %q}, want {1, test}", got.ID, got.Name)
	}
}

func TestGetSourceByName_MissReturns404(t *testing.T) {
	srv := newTestServer(&stubDB{getSourceErr: errors.New("no rows")}, nil)
	resp, err := srv.GetSourceByName(context.Background(), GetSourceByNameRequestObject{
		Params: GetSourceByNameParams{Name: "ghost"},
	})
	if err != nil {
		t.Fatalf("GetSourceByName error: %v", err)
	}
	got, ok := resp.(GetSourceByName404JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetSourceByName404JSONResponse", resp)
	}
	if got.Error == "" {
		t.Error("expected a non-empty error message")
	}
}
