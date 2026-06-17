package handler

import (
	"context"
	"errors"
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
	// Optional overrides for the error/permission branches; nil/zero keeps the
	// default ownerOf/items-driven behaviour the existing tests rely on.
	listVisibleErr error
	getErr         error
	updateResult   *bool
	updateErr      error
	isOwnerErr     error
	listSharesErr  error
	removeShareErr error
	removeShareNo  bool // when true, RemoveShare reports "no such grant"
}

func newMockAnalyses() *mockAnalyses {
	return &mockAnalyses{items: map[string]*storage.Analysis{}, ownerOf: map[string]string{}}
}

func (m *mockAnalyses) ListVisible(_ context.Context, _ string) ([]storage.AnalysisListItem, error) {
	if m.listVisibleErr != nil {
		return nil, m.listVisibleErr
	}
	return []storage.AnalysisListItem{{ID: "a1", Name: "x", OwnerEmail: "o@x.y", Permission: "editable", Owned: true}}, nil
}
func (m *mockAnalyses) Get(_ context.Context, id, userID string) (*storage.Analysis, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
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
	if m.updateErr != nil {
		return false, m.updateErr
	}
	if m.updateResult != nil {
		return *m.updateResult, nil
	}
	return true, nil
}
func (m *mockAnalyses) Delete(_ context.Context, id, userID string) (bool, error) {
	return m.ownerOf[id] == userID, nil
}
func (m *mockAnalyses) IsOwner(_ context.Context, id, userID string) (bool, error) {
	if m.isOwnerErr != nil {
		return false, m.isOwnerErr
	}
	return m.ownerOf[id] == userID, nil
}
func (m *mockAnalyses) ListShares(_ context.Context, _ string) ([]storage.ShareItem, error) {
	if m.listSharesErr != nil {
		return nil, m.listSharesErr
	}
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
func (m *mockAnalyses) RemoveShare(_ context.Context, _, _ string) (bool, error) {
	if m.removeShareErr != nil {
		return false, m.removeShareErr
	}
	return !m.removeShareNo, nil
}

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

	resp, _ := s.DeleteAnalysis(withUser("notowner"), DeleteAnalysisRequestObject{ID: "a1"})
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
	r1, _ := s.PostAnalysisShare(ctx, PostAnalysisShareRequestObject{ID: "a1", Body: &PostAnalysisShareJSONRequestBody{Email: openapi_types.Email("nobody@x.y")}})
	rec := httptest.NewRecorder()
	_ = r1.VisitPostAnalysisShareResponse(rec)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown grantee: expected 404, got %d", rec.Code)
	}

	// Self → 400.
	r2, _ := s.PostAnalysisShare(ctx, PostAnalysisShareRequestObject{ID: "a1", Body: &PostAnalysisShareJSONRequestBody{Email: openapi_types.Email("self@x.y")}})
	rec = httptest.NewRecorder()
	_ = r2.VisitPostAnalysisShareResponse(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self share: expected 400, got %d", rec.Code)
	}

	// Non-owner sharing → 403.
	r3, _ := s.PostAnalysisShare(withUser("stranger"), PostAnalysisShareRequestObject{ID: "a1", Body: &PostAnalysisShareJSONRequestBody{Email: openapi_types.Email("g@x.y")}})
	rec = httptest.NewRecorder()
	_ = r3.VisitPostAnalysisShareResponse(rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner share: expected 403, got %d", rec.Code)
	}
}

func seededAnalysis(m *mockAnalyses, id, owner string) {
	m.items[id] = &storage.Analysis{
		ID: id, Name: "My", Description: "d", State: "?activePillar=aleph",
		OwnerEmail: "o@x.y", Permission: "editable", Owned: true,
	}
	m.ownerOf[id] = owner
}

func TestGetAnalysis_FoundReturnsFullState(t *testing.T) {
	m := newMockAnalyses()
	seededAnalysis(m, "a1", "u1")
	s := analysesServer(m)

	resp, err := s.GetAnalysis(withUser("u1"), GetAnalysisRequestObject{ID: "a1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetAnalysis200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.ID != "a1" || got.State != "?activePillar=aleph" {
		t.Errorf("got %+v, want full state for a1", got)
	}
}

func TestGetAnalysis_Unauthenticated_401(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	resp, _ := s.GetAnalysis(context.Background(), GetAnalysisRequestObject{ID: "a1"})
	if _, ok := resp.(GetAnalysis401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401", resp)
	}
}

func TestGetAnalysis_NotFound_404(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	resp, _ := s.GetAnalysis(withUser("u1"), GetAnalysisRequestObject{ID: "missing"})
	if _, ok := resp.(GetAnalysis404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetAnalysis_StoreError_500(t *testing.T) {
	m := newMockAnalyses()
	m.getErr = errors.New("pg down")
	s := analysesServer(m)
	resp, _ := s.GetAnalysis(withUser("u1"), GetAnalysisRequestObject{ID: "a1"})
	if _, ok := resp.(GetAnalysis500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestPatchAnalysis_UpdatesAndReloads(t *testing.T) {
	m := newMockAnalyses()
	seededAnalysis(m, "a1", "u1")
	s := analysesServer(m)
	newName := "Renamed"
	resp, err := s.PatchAnalysis(withUser("u1"), PatchAnalysisRequestObject{
		ID: "a1", Body: &PatchAnalysisJSONRequestBody{Name: &newName},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(PatchAnalysis200JSONResponse); !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
}

func TestPatchAnalysis_NilBody_403(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	resp, _ := s.PatchAnalysis(withUser("u1"), PatchAnalysisRequestObject{ID: "a1", Body: nil})
	if _, ok := resp.(PatchAnalysis403JSONResponse); !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
}

func TestPatchAnalysis_NotShared_403(t *testing.T) {
	s := analysesServer(newMockAnalyses()) // nothing seeded → Get returns nil
	name := "x"
	resp, _ := s.PatchAnalysis(withUser("u1"), PatchAnalysisRequestObject{ID: "a1", Body: &PatchAnalysisJSONRequestBody{Name: &name}})
	if _, ok := resp.(PatchAnalysis403JSONResponse); !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
}

func TestPatchAnalysis_UpdateDenied_403(t *testing.T) {
	m := newMockAnalyses()
	seededAnalysis(m, "a1", "u1")
	denied := false
	m.updateResult = &denied // Get sees the row, but Update reports not-allowed
	s := analysesServer(m)
	name := "x"
	resp, _ := s.PatchAnalysis(withUser("u1"), PatchAnalysisRequestObject{ID: "a1", Body: &PatchAnalysisJSONRequestBody{Name: &name}})
	if _, ok := resp.(PatchAnalysis403JSONResponse); !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
}

func TestPatchAnalysis_UpdateError_500(t *testing.T) {
	m := newMockAnalyses()
	seededAnalysis(m, "a1", "u1")
	m.updateErr = errors.New("pg down")
	s := analysesServer(m)
	name := "x"
	resp, _ := s.PatchAnalysis(withUser("u1"), PatchAnalysisRequestObject{ID: "a1", Body: &PatchAnalysisJSONRequestBody{Name: &name}})
	if _, ok := resp.(PatchAnalysis500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestGetAnalysisShares_OwnerListsGrantees(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	s := analysesServer(m)
	resp, err := s.GetAnalysisShares(withUser("owner"), GetAnalysisSharesRequestObject{ID: "a1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetAnalysisShares200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if len(got.Shares) != 1 || got.Shares[0].GranteeID != "g1" {
		t.Errorf("shares = %+v, want one g1 grant", got.Shares)
	}
}

func TestGetAnalysisShares_NonOwner_403(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	s := analysesServer(m)
	resp, _ := s.GetAnalysisShares(withUser("stranger"), GetAnalysisSharesRequestObject{ID: "a1"})
	if _, ok := resp.(GetAnalysisShares403JSONResponse); !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
}

func TestGetAnalysisShares_OwnerCheckError_500(t *testing.T) {
	m := newMockAnalyses()
	m.isOwnerErr = errors.New("pg down")
	s := analysesServer(m)
	resp, _ := s.GetAnalysisShares(withUser("owner"), GetAnalysisSharesRequestObject{ID: "a1"})
	if _, ok := resp.(GetAnalysisShares500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestGetAnalysisShares_ListError_500(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	m.listSharesErr = errors.New("pg down")
	s := analysesServer(m)
	resp, _ := s.GetAnalysisShares(withUser("owner"), GetAnalysisSharesRequestObject{ID: "a1"})
	if _, ok := resp.(GetAnalysisShares500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestDeleteAnalysisShare_OwnerRevokes_204(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	s := analysesServer(m)
	resp, err := s.DeleteAnalysisShare(withUser("owner"), DeleteAnalysisShareRequestObject{ID: "a1", GranteeID: "g1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(DeleteAnalysisShare204Response); !ok {
		t.Fatalf("response = %T, want 204", resp)
	}
}

func TestDeleteAnalysisShare_NonOwner_403(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	s := analysesServer(m)
	resp, _ := s.DeleteAnalysisShare(withUser("stranger"), DeleteAnalysisShareRequestObject{ID: "a1", GranteeID: "g1"})
	if _, ok := resp.(DeleteAnalysisShare403JSONResponse); !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
}

func TestDeleteAnalysisShare_NoSuchGrant_404(t *testing.T) {
	m := newMockAnalyses()
	m.ownerOf["a1"] = "owner"
	m.removeShareNo = true
	s := analysesServer(m)
	resp, _ := s.DeleteAnalysisShare(withUser("owner"), DeleteAnalysisShareRequestObject{ID: "a1", GranteeID: "ghost"})
	if _, ok := resp.(DeleteAnalysisShare404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestDeleteAnalysisShare_Unauthenticated_401(t *testing.T) {
	s := analysesServer(newMockAnalyses())
	resp, _ := s.DeleteAnalysisShare(context.Background(), DeleteAnalysisShareRequestObject{ID: "a1", GranteeID: "g1"})
	if _, ok := resp.(DeleteAnalysisShare401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401", resp)
	}
}

func TestListVisible_StoreError_500(t *testing.T) {
	m := newMockAnalyses()
	m.listVisibleErr = errors.New("pg down")
	s := analysesServer(m)
	resp, _ := s.GetAnalyses(withUser("u1"), GetAnalysesRequestObject{})
	if _, ok := resp.(GetAnalyses500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}
