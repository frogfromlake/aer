package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// revWindow is a fixed valid analysis window reused across the revision tests.
func revWindow() (time.Time, time.Time) {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
}

// revServerWithDossier wires a Server with a dossier (source-scope + per-article
// paths resolve through it) plus the bundled probe registry.
func revServerWithDossier(store *mockStore, dossier *fakeDossier) *Server {
	return NewServerWithOptions(store, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:  dossier,
		Articles: &fakeArticles{},
		Silver:   &fakeSilver{},
	})
}

// --- GetRevisionActivity ---

func TestGetRevisionActivity_ProbeScope_MapsCellsAndTriggerBreakdown(t *testing.T) {
	bucket := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	store := &mockStore{revisionActivity: []storage.RevisionActivityCell{{
		Source:              "tagesschau",
		Bucket:              bucket,
		Revisions:           7,
		ArticlesAffected:    4,
		CdxSnapshotCount:    5,
		RepublicationCount:  2,
		UnknownTriggerCount: 0,
	}}}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()

	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetRevisionActivity200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetRevisionActivity200JSONResponse", resp)
	}
	// Probe scope expands to the probe's full source list.
	if len(store.capturedSources) != 2 {
		t.Errorf("captured sources = %v, want the 2 probe sources", store.capturedSources)
	}
	if got.Scope != RevisionActivityResponseScopeProbe {
		t.Errorf("scope = %q, want probe", got.Scope)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(got.Entries))
	}
	e := got.Entries[0]
	if e.Revisions != 7 || e.ArticlesAffected != 4 {
		t.Errorf("revisions/affected = %d/%d, want 7/4", e.Revisions, e.ArticlesAffected)
	}
	if e.ByTrigger == nil {
		t.Fatalf("byTrigger nil, want a breakdown")
	}
	bt := *e.ByTrigger
	if bt["cdx_snapshot"] != 5 || bt["republication_trigger"] != 2 {
		t.Errorf("byTrigger = %v, want cdx_snapshot=5 republication_trigger=2", bt)
	}
	if _, present := bt["unknown"]; present {
		t.Errorf("byTrigger must omit a zero unknown bucket, got %v", bt)
	}
}

func TestGetRevisionActivity_SourceScope_ResolvesThroughDossier(t *testing.T) {
	store := &mockStore{}
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	s := revServerWithDossier(store, dossier)
	scope := GetRevisionActivityParamsScopeSource
	start, end := revWindow()

	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{Scope: &scope, ScopeID: "tagesschau", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetRevisionActivity200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.Scope != RevisionActivityResponseScopeSource {
		t.Errorf("scope = %q, want source", got.Scope)
	}
	if len(store.capturedSources) != 1 || store.capturedSources[0] != "tagesschau" {
		t.Errorf("captured sources = %v, want [tagesschau]", store.capturedSources)
	}
}

func TestGetRevisionActivity_UnknownProbe_Returns404(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{ScopeID: "probe-does-not-exist", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetRevisionActivity404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetRevisionActivity_SourceNotFound_Returns404(t *testing.T) {
	dossier := &fakeDossier{resolveErr: storage.ErrSourceNotFound}
	s := revServerWithDossier(&mockStore{}, dossier)
	scope := GetRevisionActivityParamsScopeSource
	start, end := revWindow()
	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{Scope: &scope, ScopeID: "ghost", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetRevisionActivity404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetRevisionActivity_ResolveInternalError_Returns500(t *testing.T) {
	dossier := &fakeDossier{resolveErr: errors.New("pg down")}
	s := revServerWithDossier(&mockStore{}, dossier)
	scope := GetRevisionActivityParamsScopeSource
	start, end := revWindow()
	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{Scope: &scope, ScopeID: "x", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetRevisionActivity500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestGetRevisionActivity_InvalidWindow_Returns400(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry())
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // end before start
	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetRevisionActivity400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400", resp)
	}
}

func TestGetRevisionActivity_StorageError_Returns500(t *testing.T) {
	store := &mockStore{revisionActivityErr: errors.New("clickhouse timeout")}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, err := s.GetRevisionActivity(context.Background(), GetRevisionActivityRequestObject{
		Params: GetRevisionActivityParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetRevisionActivity500JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
	if got.Message != genericInternalError {
		t.Errorf("message = %q, want generic", got.Message)
	}
}

// --- GetRevisionDiscourseShift ---

func TestGetRevisionDiscourseShift_MapsCells(t *testing.T) {
	bucket := time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC)
	store := &mockStore{revisionDiscourseShift: []storage.RevisionDiscourseShiftCell{{
		Source:               "tagesschau",
		Bucket:               bucket,
		EditsWithDeltas:      3,
		AvgSentimentDelta:    -0.12,
		NetSentimentDrift:    -0.36,
		AvgTopicShift:        0.4,
		EntitiesAddedTotal:   5,
		EntitiesRemovedTotal: 2,
	}}}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, err := s.GetRevisionDiscourseShift(context.Background(), GetRevisionDiscourseShiftRequestObject{
		Params: GetRevisionDiscourseShiftParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetRevisionDiscourseShift200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(got.Entries))
	}
	e := got.Entries[0]
	if e.EditsWithDeltas != 3 || e.EntitiesAddedTotal != 5 || e.EntitiesRemovedTotal != 2 {
		t.Errorf("counts = %d/%d/%d, want 3/5/2", e.EditsWithDeltas, e.EntitiesAddedTotal, e.EntitiesRemovedTotal)
	}
	if e.AvgSentimentDelta != -0.12 || e.NetSentimentDrift != -0.36 {
		t.Errorf("sentiment = %v/%v, want -0.12/-0.36", e.AvgSentimentDelta, e.NetSentimentDrift)
	}
}

func TestGetRevisionDiscourseShift_UnknownProbe_Returns404(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, _ := s.GetRevisionDiscourseShift(context.Background(), GetRevisionDiscourseShiftRequestObject{
		Params: GetRevisionDiscourseShiftParams{ScopeID: "nope", StartDate: &start, EndDate: &end},
	})
	if _, ok := resp.(GetRevisionDiscourseShift404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetRevisionDiscourseShift_StorageError_Returns500(t *testing.T) {
	store := &mockStore{revisionDiscourseShiftErr: errors.New("boom")}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, _ := s.GetRevisionDiscourseShift(context.Background(), GetRevisionDiscourseShiftRequestObject{
		Params: GetRevisionDiscourseShiftParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if _, ok := resp.(GetRevisionDiscourseShift500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- GetRevisionEditClusters ---

func TestGetRevisionEditClusters_MapsClustersAndClampsMinSources(t *testing.T) {
	store := &mockStore{revisionEditClusters: []storage.RevisionEditClusterRow{{
		Bucket:        time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
		Entity:        "Q567",
		Sources:       []string{"tagesschau", "bundesregierung"},
		EditCount:     6,
		AvgTopicShift: 0.25,
	}}}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	over := 99 // above the [2,10] clamp ceiling
	resp, err := s.GetRevisionEditClusters(context.Background(), GetRevisionEditClustersRequestObject{
		Params: GetRevisionEditClustersParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end, MinSources: &over},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetRevisionEditClusters200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.MinSources != 10 {
		t.Errorf("minSources = %d, want clamped to 10", got.MinSources)
	}
	if store.capturedMinSources != 10 {
		t.Errorf("store saw minSources = %d, want 10", store.capturedMinSources)
	}
	if len(got.Clusters) != 1 || got.Clusters[0].Entity != "Q567" || got.Clusters[0].EditCount != 6 {
		t.Errorf("clusters = %+v, want one Q567/6 cluster", got.Clusters)
	}
}

func TestGetRevisionEditClusters_MinSourcesFloor(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	below := 1 // below the floor of 2
	resp, _ := s.GetRevisionEditClusters(context.Background(), GetRevisionEditClustersRequestObject{
		Params: GetRevisionEditClustersParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end, MinSources: &below},
	})
	got, ok := resp.(GetRevisionEditClusters200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.MinSources != 2 {
		t.Errorf("minSources = %d, want floored to 2", got.MinSources)
	}
}

func TestGetRevisionEditClusters_StorageError_Returns500(t *testing.T) {
	store := &mockStore{revisionEditClustersErr: errors.New("boom")}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, _ := s.GetRevisionEditClusters(context.Background(), GetRevisionEditClustersRequestObject{
		Params: GetRevisionEditClustersParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if _, ok := resp.(GetRevisionEditClusters500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- GetArticleRevisions ---

func TestGetArticleRevisions_DossierNotConfigured_Returns500(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry()) // no dossier
	resp, _ := s.GetArticleRevisions(context.Background(), GetArticleRevisionsRequestObject{ID: "a1"})
	if _, ok := resp.(GetArticleRevisions500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestGetArticleRevisions_ArticleNotFound_Returns404(t *testing.T) {
	dossier := &fakeDossier{articleErr: storage.ErrSourceNotFound}
	s := revServerWithDossier(&mockStore{}, dossier)
	resp, _ := s.GetArticleRevisions(context.Background(), GetArticleRevisionsRequestObject{ID: "ghost"})
	if _, ok := resp.(GetArticleRevisions404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetArticleRevisions_NotEligible_Returns403(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "bundesregierung"},
		eligibility: &storage.SourceEligibilityRow{Name: "bundesregierung", SilverEligible: false},
	}
	s := revServerWithDossier(&mockStore{}, dossier)
	resp, _ := s.GetArticleRevisions(context.Background(), GetArticleRevisionsRequestObject{ID: "a1"})
	got, ok := resp.(GetArticleRevisions403JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
	if got.Gate != SilverEligibility {
		t.Errorf("gate = %q, want silver_eligibility", got.Gate)
	}
}

func TestGetArticleRevisions_EligibleEmptyChain_LookupStatusEmpty(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	s := revServerWithDossier(&mockStore{articleRevisions: nil}, dossier)
	resp, err := s.GetArticleRevisions(context.Background(), GetArticleRevisionsRequestObject{ID: "a1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetArticleRevisions200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.LookupStatus != Empty {
		t.Errorf("lookupStatus = %q, want empty", got.LookupStatus)
	}
	if got.Source != "tagesschau" {
		t.Errorf("source = %q, want tagesschau", got.Source)
	}
}

func TestGetArticleRevisions_MapsChainHeadAndDeltas(t *testing.T) {
	head := time.Date(2025, 1, 5, 8, 0, 0, 0, time.UTC)
	next := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	store := &mockStore{articleRevisions: []storage.ArticleRevisionRow{
		{ // chain head — index 0, no predecessor fields, no computed deltas
			SnapshotAt: head, ContentHash: "h0", RevisionIndex: 0,
			Trigger: "cdx_snapshot", DiffStatus: "changed", DeltasComputed: false,
		},
		{ // mid-chain — has predecessor + computed deltas + archive URL
			SnapshotAt: next, ContentHash: "h1", PrevContentHash: "h0", RevisionIndex: 1,
			TimeSincePrevHours: 25.0, Trigger: "republication_trigger", DiffStatus: "changed",
			ArchiveURL: "https://web.archive.org/x", DeltasComputed: true,
			SentimentDelta: -0.2, TopicShiftScore: 0.3,
			EntitiesAdded: []string{"Q1"}, EntitiesRemoved: []string{},
		},
	}}
	s := revServerWithDossier(store, dossier)
	resp, err := s.GetArticleRevisions(context.Background(), GetArticleRevisionsRequestObject{ID: "a1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetArticleRevisions200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.LookupStatus != Ok {
		t.Errorf("lookupStatus = %q, want ok", got.LookupStatus)
	}
	if len(got.Revisions) != 2 {
		t.Fatalf("revisions = %d, want 2", len(got.Revisions))
	}
	// Chain head: prev fields + deltas omitted.
	headRow := got.Revisions[0]
	if headRow.PrevContentHash != nil || headRow.TimeSincePrevHours != nil {
		t.Errorf("chain head must omit prev fields, got prev=%v tsp=%v", headRow.PrevContentHash, headRow.TimeSincePrevHours)
	}
	if headRow.SentimentDelta != nil {
		t.Errorf("chain head has DeltasComputed=false → sentimentDelta must be nil, got %v", *headRow.SentimentDelta)
	}
	// Mid-chain: prev fields + deltas + archive URL present.
	mid := got.Revisions[1]
	if mid.PrevContentHash == nil || *mid.PrevContentHash != "h0" {
		t.Errorf("mid prevContentHash = %v, want h0", mid.PrevContentHash)
	}
	if mid.ArchiveURL == nil || *mid.ArchiveURL == "" {
		t.Errorf("mid archiveUrl missing")
	}
	if mid.SentimentDelta == nil || *mid.SentimentDelta != -0.2 {
		t.Errorf("mid sentimentDelta = %v, want -0.2", mid.SentimentDelta)
	}
	if mid.EntitiesAdded == nil || len(*mid.EntitiesAdded) != 1 {
		t.Errorf("mid entitiesAdded = %v, want [Q1]", mid.EntitiesAdded)
	}
}

func TestGetArticleRevisions_StorageError_Returns500(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	s := revServerWithDossier(&mockStore{articleRevisionsErr: errors.New("boom")}, dossier)
	resp, _ := s.GetArticleRevisions(context.Background(), GetArticleRevisionsRequestObject{ID: "a1"})
	if _, ok := resp.(GetArticleRevisions500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- GetArticleRevisionDiff ---

func TestGetArticleRevisionDiff_MidChain_MapsOps(t *testing.T) {
	before := time.Date(2025, 1, 5, 8, 0, 0, 0, time.UTC)
	after := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	// One replace op encoded the way the worker writes diff_paragraphs.
	store := &mockStore{articleRevisionDiff: &storage.ArticleRevisionDiffRow{
		ArticleID: "a1", RevisionIndex: 1,
		SnapshotAtBefore: before, SnapshotAtAfter: after,
		HeadlineChanged: true, HeadlineBefore: "Old", HeadlineAfter: "New",
		DiffParagraphs: []string{`{"op":"replace","before":"x","after":"y"}`},
		Source:         "tagesschau",
	}}
	s := revServerWithDossier(store, dossier)
	resp, err := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "a1", RevisionIndex: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetArticleRevisionDiff200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.PairKind != MidChain {
		t.Errorf("pairKind = %q, want mid_chain", got.PairKind)
	}
	if got.Identical {
		t.Errorf("identical = true, want false")
	}
	if got.SnapshotAtBefore == nil {
		t.Errorf("mid-chain must carry snapshotAtBefore")
	}
	if got.HeadlineBefore == nil || got.HeadlineAfter == nil {
		t.Errorf("headline before/after should be present")
	}
	if len(got.DiffParagraphs) != 1 || got.DiffParagraphs[0].Op != "replace" {
		t.Errorf("diffParagraphs = %+v, want one replace op", got.DiffParagraphs)
	}
}

func TestGetArticleRevisionDiff_ChainHead_NoBeforeSnapshot(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	store := &mockStore{articleRevisionDiff: &storage.ArticleRevisionDiffRow{
		ArticleID: "a1", RevisionIndex: 0,
		SnapshotAtAfter: time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC),
		DiffParagraphs:  []string{`{"op":"insert","after":"hello"}`},
		Source:          "tagesschau",
	}}
	s := revServerWithDossier(store, dossier)
	resp, _ := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "a1", RevisionIndex: 0})
	got, ok := resp.(GetArticleRevisionDiff200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.PairKind != ChainHead {
		t.Errorf("pairKind = %q, want chain_head", got.PairKind)
	}
	if got.SnapshotAtBefore != nil {
		t.Errorf("chain head must omit snapshotAtBefore, got %v", *got.SnapshotAtBefore)
	}
}

func TestGetArticleRevisionDiff_Identical_EmptyOps(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	// IsIdentical() recognises the worker's identical sentinel in DiffParagraphs.
	store := &mockStore{articleRevisionDiff: &storage.ArticleRevisionDiffRow{
		ArticleID: "a1", RevisionIndex: 1,
		SnapshotAtBefore: time.Date(2025, 1, 5, 8, 0, 0, 0, time.UTC),
		SnapshotAtAfter:  time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC),
		DiffParagraphs:   []string{`{"op":"identical"}`},
		Source:           "tagesschau",
	}}
	s := revServerWithDossier(store, dossier)
	resp, _ := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "a1", RevisionIndex: 1})
	got, ok := resp.(GetArticleRevisionDiff200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if !got.Identical {
		t.Errorf("identical = false, want true for the sentinel row")
	}
	if len(got.DiffParagraphs) != 0 {
		t.Errorf("identical row must carry empty diffParagraphs, got %d", len(got.DiffParagraphs))
	}
}

func TestGetArticleRevisionDiff_DiffPending_Returns404(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	s := revServerWithDossier(&mockStore{articleRevisionDiffErr: storage.ErrRevisionDiffPending}, dossier)
	resp, _ := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "a1", RevisionIndex: 1})
	if _, ok := resp.(GetArticleRevisionDiff404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetArticleRevisionDiff_NotEligible_Returns403(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "bundesregierung"},
		eligibility: &storage.SourceEligibilityRow{Name: "bundesregierung", SilverEligible: false},
	}
	s := revServerWithDossier(&mockStore{}, dossier)
	resp, _ := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "a1", RevisionIndex: 1})
	if _, ok := resp.(GetArticleRevisionDiff403JSONResponse); !ok {
		t.Fatalf("response = %T, want 403", resp)
	}
}

func TestGetArticleRevisionDiff_ArticleNotFound_Returns404(t *testing.T) {
	dossier := &fakeDossier{articleErr: storage.ErrSourceNotFound}
	s := revServerWithDossier(&mockStore{}, dossier)
	resp, _ := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "ghost", RevisionIndex: 0})
	if _, ok := resp.(GetArticleRevisionDiff404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetArticleRevisionDiff_StorageError_Returns500(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{SourceName: "tagesschau"},
		eligibility: &storage.SourceEligibilityRow{Name: "tagesschau", SilverEligible: true},
	}
	s := revServerWithDossier(&mockStore{articleRevisionDiffErr: errors.New("boom")}, dossier)
	resp, _ := s.GetArticleRevisionDiff(context.Background(), GetArticleRevisionDiffRequestObject{ID: "a1", RevisionIndex: 1})
	if _, ok := resp.(GetArticleRevisionDiff500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- GetRevisionsArticles ---

func TestGetRevisionsArticles_PaginatesAndEncodesCursor(t *testing.T) {
	// Two rows returned with limit=1 → hasMore true (limit+1 fetched), one item.
	rows := []storage.RevisionArticleRow{
		{ArticleID: "a1", Source: "tagesschau", Timestamp: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Language: "de", WordCount: 320, ChainLength: 3, EditorialChangeCount: 2, HasHeadlineChange: true,
			LatestRevisionAt: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)},
		{ArticleID: "a2", Source: "tagesschau", Timestamp: time.Date(2025, 1, 5, 1, 0, 0, 0, time.UTC)},
	}
	store := &mockStore{revisionsArticles: rows}
	s := NewServer(store, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	limit := 1
	resp, err := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end, Limit: &limit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetRevisionsArticles200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if !got.HasMore {
		t.Errorf("hasMore = false, want true")
	}
	if got.NextCursor == nil || *got.NextCursor == "" {
		t.Errorf("nextCursor missing on a hasMore page")
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %d, want 1 (limit trim)", len(got.Items))
	}
	// The store must have been asked for limit+1 to detect hasMore.
	if store.capturedRevisionsFilter.Limit != 2 {
		t.Errorf("store limit = %d, want limit+1=2", store.capturedRevisionsFilter.Limit)
	}
	item := got.Items[0]
	if item.Language == nil || *item.Language != "de" {
		t.Errorf("language = %v, want de", item.Language)
	}
	if item.WordCount == nil || *item.WordCount != 320 {
		t.Errorf("wordCount = %v, want 320", item.WordCount)
	}
	if item.LatestRevisionAt == nil {
		t.Errorf("latestRevisionAt missing")
	}
}

func TestGetRevisionsArticles_EmptyResult_EmptyItemsSlice(t *testing.T) {
	s := NewServer(&mockStore{revisionsArticles: nil}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, _ := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	got, ok := resp.(GetRevisionsArticles200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if got.HasMore {
		t.Errorf("hasMore = true, want false")
	}
	if got.Items == nil {
		t.Errorf("items must be a non-nil empty slice (renders as [] not null)")
	}
}

func TestGetRevisionsArticles_InvalidLimit_Returns400(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	bad := 999
	resp, _ := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end, Limit: &bad},
	})
	if _, ok := resp.(GetRevisionsArticles400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400", resp)
	}
}

func TestGetRevisionsArticles_InvalidCursor_Returns400(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	bad := "!!!not-base64!!!"
	resp, _ := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end, Cursor: &bad},
	})
	if _, ok := resp.(GetRevisionsArticles400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400", resp)
	}
}

func TestGetRevisionsArticles_UnknownProbe_Returns404(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, _ := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{ScopeID: "nope", StartDate: &start, EndDate: &end},
	})
	if _, ok := resp.(GetRevisionsArticles404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestGetRevisionsArticles_StorageError_Returns500(t *testing.T) {
	s := NewServer(&mockStore{revisionsArticlesErr: errors.New("boom")}, nil, nil, nil, testProbeRegistry())
	start, end := revWindow()
	resp, _ := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{ScopeID: "probe-0-de-institutional-web", StartDate: &start, EndDate: &end},
	})
	if _, ok := resp.(GetRevisionsArticles500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestGetRevisionsArticles_SourceScope_FiltersAndCursorRoundTrips(t *testing.T) {
	store := &mockStore{}
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	s := revServerWithDossier(store, dossier)
	scope := GetRevisionsArticlesParamsScopeSource
	start, end := revWindow()
	// A valid cursor (offset=10) decodes and is applied to the filter.
	cursor := encodeRevisionsCursor(10)
	hasHeadline := true
	minChain := 2
	resp, err := s.GetRevisionsArticles(context.Background(), GetRevisionsArticlesRequestObject{
		Params: GetRevisionsArticlesParams{
			Scope: &scope, ScopeID: "tagesschau", StartDate: &start, EndDate: &end,
			Cursor: &cursor, HasHeadlineChange: &hasHeadline, MinChainLength: &minChain,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetRevisionsArticles200JSONResponse); !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	f := store.capturedRevisionsFilter
	if f.Offset != 10 {
		t.Errorf("offset = %d, want 10 from the cursor", f.Offset)
	}
	if !f.HasHeadlineChange || f.MinChainLength != 2 {
		t.Errorf("filter flags = %v/%d, want true/2", f.HasHeadlineChange, f.MinChainLength)
	}
	if len(f.Sources) != 1 || f.Sources[0] != "tagesschau" {
		t.Errorf("sources = %v, want [tagesschau]", f.Sources)
	}
}

// --- resolution / scope converters (table-driven) ---

func TestRevisionResolutionFromParam(t *testing.T) {
	d := GetRevisionActivityParamsResolutionDaily
	w := GetRevisionActivityParamsResolutionWeekly
	m := GetRevisionActivityParamsResolutionMonthly
	snap := GetRevisionActivityParamsResolutionSnapshot
	cases := []struct {
		name string
		in   *GetRevisionActivityParamsResolution
		want storage.RevisionActivityResolution
	}{
		{"nil → snapshot", nil, storage.RevisionResolutionSnapshot},
		{"daily", &d, storage.RevisionResolutionDaily},
		{"weekly", &w, storage.RevisionResolutionWeekly},
		{"monthly", &m, storage.RevisionResolutionMonthly},
		{"snapshot", &snap, storage.RevisionResolutionSnapshot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := revisionResolutionFromParam(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRevisionResolutionToResponse(t *testing.T) {
	cases := []struct {
		in   storage.RevisionActivityResolution
		want RevisionActivityResponseResolution
	}{
		{storage.RevisionResolutionDaily, RevisionActivityResponseResolutionDaily},
		{storage.RevisionResolutionWeekly, RevisionActivityResponseResolutionWeekly},
		{storage.RevisionResolutionMonthly, RevisionActivityResponseResolutionMonthly},
		{storage.RevisionResolutionSnapshot, RevisionActivityResponseResolutionSnapshot},
	}
	for _, tc := range cases {
		if got := revisionResolutionToResponse(tc.in); got != tc.want {
			t.Errorf("%q → %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRevisionScopeToResponse(t *testing.T) {
	if got := revisionScopeToResponse(GetRevisionActivityParamsScopeSource); got != RevisionActivityResponseScopeSource {
		t.Errorf("source → %q, want source", got)
	}
	if got := revisionScopeToResponse(GetRevisionActivityParamsScopeProbe); got != RevisionActivityResponseScopeProbe {
		t.Errorf("probe → %q, want probe", got)
	}
}

func TestDiscourseShiftConverters(t *testing.T) {
	d := GetRevisionDiscourseShiftParamsResolutionDaily
	w := GetRevisionDiscourseShiftParamsResolutionWeekly
	m := GetRevisionDiscourseShiftParamsResolutionMonthly
	s := GetRevisionDiscourseShiftParamsResolutionSnapshot
	if discourseShiftResolutionFromParam(nil) != storage.RevisionResolutionDaily {
		t.Error("nil should default to daily for discourse shift")
	}
	for _, tc := range []struct {
		in   *GetRevisionDiscourseShiftParamsResolution
		want storage.RevisionActivityResolution
	}{
		{&d, storage.RevisionResolutionDaily},
		{&w, storage.RevisionResolutionWeekly},
		{&m, storage.RevisionResolutionMonthly},
		{&s, storage.RevisionResolutionSnapshot},
	} {
		if got := discourseShiftResolutionFromParam(tc.in); got != tc.want {
			t.Errorf("from %v → %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, tc := range []struct {
		in   storage.RevisionActivityResolution
		want RevisionDiscourseShiftResponseResolution
	}{
		{storage.RevisionResolutionDaily, RevisionDiscourseShiftResponseResolutionDaily},
		{storage.RevisionResolutionWeekly, RevisionDiscourseShiftResponseResolutionWeekly},
		{storage.RevisionResolutionMonthly, RevisionDiscourseShiftResponseResolutionMonthly},
		{storage.RevisionResolutionSnapshot, RevisionDiscourseShiftResponseResolutionSnapshot},
	} {
		if got := discourseShiftResolutionToResponse(tc.in); got != tc.want {
			t.Errorf("to %q → %q, want %q", tc.in, got, tc.want)
		}
	}
	if discourseShiftScopeToResponse(GetRevisionActivityParamsScopeSource) != RevisionDiscourseShiftResponseScopeSource {
		t.Error("source scope mismatch")
	}
	if discourseShiftScopeToResponse(GetRevisionActivityParamsScopeProbe) != RevisionDiscourseShiftResponseScopeProbe {
		t.Error("probe scope mismatch")
	}
}

func TestEditClustersConverters(t *testing.T) {
	d := GetRevisionEditClustersParamsResolutionDaily
	w := GetRevisionEditClustersParamsResolutionWeekly
	m := GetRevisionEditClustersParamsResolutionMonthly
	s := GetRevisionEditClustersParamsResolutionSnapshot
	if editClustersResolutionFromParam(nil) != storage.RevisionResolutionDaily {
		t.Error("nil should default to daily for edit clusters")
	}
	for _, tc := range []struct {
		in   *GetRevisionEditClustersParamsResolution
		want storage.RevisionActivityResolution
	}{
		{&d, storage.RevisionResolutionDaily},
		{&w, storage.RevisionResolutionWeekly},
		{&m, storage.RevisionResolutionMonthly},
		{&s, storage.RevisionResolutionSnapshot},
	} {
		if got := editClustersResolutionFromParam(tc.in); got != tc.want {
			t.Errorf("from %v → %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, tc := range []struct {
		in   storage.RevisionActivityResolution
		want RevisionEditClustersResponseResolution
	}{
		{storage.RevisionResolutionDaily, RevisionEditClustersResponseResolutionDaily},
		{storage.RevisionResolutionWeekly, RevisionEditClustersResponseResolutionWeekly},
		{storage.RevisionResolutionMonthly, RevisionEditClustersResponseResolutionMonthly},
		{storage.RevisionResolutionSnapshot, RevisionEditClustersResponseResolutionSnapshot},
	} {
		if got := editClustersResolutionToResponse(tc.in); got != tc.want {
			t.Errorf("to %q → %q, want %q", tc.in, got, tc.want)
		}
	}
	if editClustersScopeToResponse(GetRevisionActivityParamsScopeSource) != RevisionEditClustersResponseScopeSource {
		t.Error("source scope mismatch")
	}
	if editClustersScopeToResponse(GetRevisionActivityParamsScopeProbe) != RevisionEditClustersResponseScopeProbe {
		t.Error("probe scope mismatch")
	}
}
