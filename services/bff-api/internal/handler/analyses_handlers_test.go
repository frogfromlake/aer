package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// --- mock AnalysesBackend ---------------------------------------------------

type mockAnalyses struct {
	items     map[string]*storage.Analysis // id -> analysis
	ownerOf   map[string]string            // id -> owner userID
	createErr error
}

func newMockAnalyses() *mockAnalyses {
	return &mockAnalyses{items: map[string]*storage.Analysis{}, ownerOf: map[string]string{}}
}

func (m *mockAnalyses) ListVisible(_ context.Context, _ string) ([]storage.AnalysisListItem, error) {
	return []storage.AnalysisListItem{{ID: "a1", Name: "x", OwnerEmail: "o@x.y", Permission: "editable", Owned: true}}, nil
}
func (m *mockAnalyses) Get(_ context.Context, id, userID string) (*storage.Analysis, error) {
	a, ok := m.items[id]
	if !ok || m.ownerOf[id] != userID {
		return nil, nil
	}
	return a, nil
}
func (m *mockAnalyses) Create(_ context.Context, ownerID, name, description, state string) (storage.AnalysisListItem, error) {
	if m.createErr != nil {
		return storage.AnalysisListItem{}, m.createErr
	}
	return storage.AnalysisListItem{ID: "new", Name: name, Description: description, OwnerEmail: "o@x.y", Permission: "editable", Owned: true}, nil
}
func (m *mockAnalyses) Update(_ context.Context, _, _, _, _, _ string) (bool, error) {
	return true, nil
}
func (m *mockAnalyses) Delete(_ context.Context, id, userID string) (bool, error) {
	return m.ownerOf[id] == userID, nil
}
func (m *mockAnalyses) IsOwner(_ context.Context, id, userID string) (bool, error) {
	return m.ownerOf[id] == userID, nil
}
func (m *mockAnalyses) ListShares(_ context.Context, _ string) ([]storage.ShareItem, error) {
	return []storage.ShareItem{{GranteeID: "g1", Email: "g@x.y", CanEdit: true}}, nil
}
func (m *mockAnalyses) AddShare(_ context.Context, _, _, email string, canEdit bool) (storage.ShareItem, error) {
	switch email {
	case "nobody@x.y":
		return storage.ShareItem{}, storage.ErrGranteeNotFound
	case "self@x.y":
		return storage.ShareItem{}, storage.ErrCannotShareWithSelf
	}
	return storage.ShareItem{GranteeID: "g1", Email: email, CanEdit: canEdit}, nil
}
func (m *mockAnalyses) RemoveShare(_ context.Context, _, _ string) (bool, error) { return true, nil }

func analysesServer(m *mockAnalyses) *Server {
	return &Server{analysesBackend: m}
}

func withUser(userID string) context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{UserID: userID, Email: "o@x.y", Role: auth.RoleResearcher})
}

// --- tests ------------------------------------------------------------------

func TestAnalysesRequireSession(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	ctx := context.Background()

	r1, _ := s.GetAnalyses(ctx, GetAnalysesRequestObject{})
	rec := httptest.NewRecorder()
	_ = r1.VisitGetAnalysesResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("list: expected 401, got %d", rec.Code)
	}

	r2, _ := s.PostAnalyses(ctx, PostAnalysesRequestObject{Body: &PostAnalysesJSONRequestBody{Name: "x", State: "?s=1"}})
	rec = httptest.NewRecorder()
	_ = r2.VisitPostAnalysesResponse(rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("create: expected 401, got %d", rec.Code)
	}
}

func TestAnalysesCreateAndList(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	ctx := withUser("u1")

	resp, _ := s.PostAnalyses(ctx, PostAnalysesRequestObject{Body: &PostAnalysesJSONRequestBody{Name: "My", State: "?activePillar=aleph"}})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAnalysesResponse(rec)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	lresp, _ := s.GetAnalyses(ctx, GetAnalysesRequestObject{})
	rec = httptest.NewRecorder()
	_ = lresp.VisitGetAnalysesResponse(rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAnalysesCreateRequiresName(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	resp, _ := s.PostAnalyses(withUser("u1"), PostAnalysesRequestObject{Body: &PostAnalysesJSONRequestBody{Name: "  ", State: "?s=1"}})
	rec := httptest.NewRecorder()
	_ = resp.VisitPostAnalysesResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blank name, got %d", rec.Code)
	}
}

func TestAnalysesDeleteNonOwnerIs403(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	s := analysesServer(m)

	resp, _ := s.DeleteAnalysis(withUser("notowner"), DeleteAnalysisRequestObject{Id: "a1"})
	rec := httptest.NewRecorder()
	_ = resp.VisitDeleteAnalysisResponse(rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAnalysesShareErrors(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	s := analysesServer(m)
	ctx := withUser("owner")

	// Unknown email → 404.
	r1, _ := s.PostAnalysisShare(ctx, PostAnalysisShareRequestObject{Id: "a1", Body: &PostAnalysisShareJSONRequestBody{Email: openapi_types.Email("nobody@x.y")}})
	rec := httptest.NewRecorder()
	_ = r1.VisitPostAnalysisShareResponse(rec)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown grantee: expected 404, got %d", rec.Code)
	}

	// Self → 400.
	r2, _ := s.PostAnalysisShare(ctx, PostAnalysisShareRequestObject{Id: "a1", Body: &PostAnalysisShareJSONRequestBody{Email: openapi_types.Email("self@x.y")}})
	rec = httptest.NewRecorder()
	_ = r2.VisitPostAnalysisShareResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self share: expected 400, got %d", rec.Code)
	}

	// Non-owner sharing → 403.
	r3, _ := s.PostAnalysisShare(withUser("stranger"), PostAnalysisShareRequestObject{Id: "a1", Body: &PostAnalysisShareJSONRequestBody{Email: openapi_types.Email("g@x.y")}})
	rec = httptest.NewRecorder()
	_ = r3.VisitPostAnalysisShareResponse(rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner share: expected 403, got %d", rec.Code)
	}
}
