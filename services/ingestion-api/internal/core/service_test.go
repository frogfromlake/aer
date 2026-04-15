package core

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// serialUploads forces IngestDocuments to process documents one at a time
// so tests can make deterministic ordering and call-count assertions.
// A separate TestIngestDocuments_ConcurrentUploadsPreserveOrdering exercises
// the parallel path and the ordering invariant the production path relies on.
const serialUploads = 1

// counterValue reads the current value of a Prometheus counter for tests
// that assert delta increments. Protected by t.Helper() so failures point
// at the caller.
func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("failed to read counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

// --- Mock implementations ---

type mockMetadataStore struct {
	mu                 sync.Mutex
	createJobFn        func(ctx context.Context, sourceID int) (int, error)
	updateJobStatusFn  func(ctx context.Context, jobID int, status string) error
	logDocumentFn      func(ctx context.Context, jobID int, key, traceID string) error
	updateDocStatusFn  func(ctx context.Context, key, status string) error
	getSourceByNameFn  func(ctx context.Context, name string) (int, string, error)
	pingFn             func(ctx context.Context) error

	// recorded calls
	jobStatuses  []string
	docStatuses  map[string]string
}

func newMockDB() *mockMetadataStore {
	m := &mockMetadataStore{docStatuses: make(map[string]string)}
	m.createJobFn = func(_ context.Context, _ int) (int, error) { return 1, nil }
	m.updateJobStatusFn = func(_ context.Context, _ int, status string) error {
		m.jobStatuses = append(m.jobStatuses, status)
		return nil
	}
	m.logDocumentFn = func(_ context.Context, _ int, _ string, _ string) error { return nil }
	m.updateDocStatusFn = func(_ context.Context, key, status string) error {
		m.docStatuses[key] = status
		return nil
	}
	m.getSourceByNameFn = func(_ context.Context, _ string) (int, string, error) { return 1, "test", nil }
	m.pingFn = func(_ context.Context) error { return nil }
	return m
}

func (m *mockMetadataStore) CreateIngestionJob(ctx context.Context, sourceID int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createJobFn(ctx, sourceID)
}
func (m *mockMetadataStore) UpdateJobStatus(ctx context.Context, jobID int, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateJobStatusFn(ctx, jobID, status)
}
func (m *mockMetadataStore) LogDocument(ctx context.Context, jobID int, key, traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logDocumentFn(ctx, jobID, key, traceID)
}
func (m *mockMetadataStore) UpdateDocumentStatus(ctx context.Context, key, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDocStatusFn(ctx, key, status)
}
func (m *mockMetadataStore) GetSourceByName(ctx context.Context, name string) (int, string, error) {
	return m.getSourceByNameFn(ctx, name)
}
func (m *mockMetadataStore) Ping(ctx context.Context) error { return m.pingFn(ctx) }

type mockObjectStore struct {
	uploadFn       func(ctx context.Context, bucket, key, data string) error
	bucketExistsFn func(ctx context.Context, bucket string) (bool, error)
	uploadedKeys   []string
}

func newMockMinio() *mockObjectStore {
	m := &mockObjectStore{}
	m.uploadFn = func(_ context.Context, _, key, _ string) error {
		m.uploadedKeys = append(m.uploadedKeys, key)
		return nil
	}
	m.bucketExistsFn = func(_ context.Context, _ string) (bool, error) { return true, nil }
	return m
}

func (m *mockObjectStore) UploadJSON(ctx context.Context, bucket, key, data string) error {
	return m.uploadFn(ctx, bucket, key, data)
}
func (m *mockObjectStore) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return m.bucketExistsFn(ctx, bucket)
}

// --- Tests ---

func TestIngestDocuments_EmptyInput(t *testing.T) {
	svc := NewIngestionService(newMockDB(), newMockMinio(), "bronze", serialUploads)
	_, err := svc.IngestDocuments(context.Background(), 1, nil)
	if err == nil {
		t.Fatal("expected error for empty document list")
	}
}

func TestIngestDocuments_CreateJobFailure(t *testing.T) {
	db := newMockDB()
	db.createJobFn = func(_ context.Context, _ int) (int, error) {
		return 0, errors.New("db unavailable")
	}
	svc := NewIngestionService(db, newMockMinio(), "bronze", serialUploads)
	_, err := svc.IngestDocuments(context.Background(), 1, []Document{{Key: "a.json", Data: "{}"}})
	if err == nil {
		t.Fatal("expected error when job creation fails")
	}
}

func TestIngestDocuments_AllSuccess(t *testing.T) {
	db := newMockDB()
	minio := newMockMinio()
	svc := NewIngestionService(db, minio, "bronze", serialUploads)

	docs := []Document{
		{Key: "doc-1.json", Data: `{"id":1}`},
		{Key: "doc-2.json", Data: `{"id":2}`},
	}

	result, err := svc.IngestDocuments(context.Background(), 1, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
	if result.Accepted != 2 {
		t.Errorf("expected 2 accepted, got %d", result.Accepted)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}

	// Job status must be recorded as "completed"
	if !slices.Contains(db.jobStatuses, "completed") {
		t.Errorf("expected job status 'completed' to be set, got %v", db.jobStatuses)
	}

	// Both documents must reach "uploaded" status
	for _, doc := range docs {
		if db.docStatuses[doc.Key] != "uploaded" {
			t.Errorf("expected doc %q status 'uploaded', got %q", doc.Key, db.docStatuses[doc.Key])
		}
	}
}

func TestIngestDocuments_PartialFailure(t *testing.T) {
	db := newMockDB()
	minio := newMockMinio()

	// First upload fails, second succeeds.
	calls := 0
	minio.uploadFn = func(_ context.Context, _, key, _ string) error {
		calls++
		minio.uploadedKeys = append(minio.uploadedKeys, key)
		if calls == 1 {
			return errors.New("upload error")
		}
		return nil
	}

	svc := NewIngestionService(db, minio, "bronze", serialUploads)
	docs := []Document{
		{Key: "fail.json", Data: "{}"},
		{Key: "ok.json", Data: "{}"},
	}

	result, err := svc.IngestDocuments(context.Background(), 1, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "completed_with_errors" {
		t.Errorf("expected 'completed_with_errors', got %q", result.Status)
	}
	if result.Accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", result.Accepted)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}

	if db.docStatuses["fail.json"] != "failed" {
		t.Errorf("expected fail.json status 'failed', got %q", db.docStatuses["fail.json"])
	}
	if db.docStatuses["ok.json"] != "uploaded" {
		t.Errorf("expected ok.json status 'uploaded', got %q", db.docStatuses["ok.json"])
	}
}

func TestIngestDocuments_AllFailed(t *testing.T) {
	db := newMockDB()
	minio := newMockMinio()
	minio.uploadFn = func(_ context.Context, _, key, _ string) error {
		return errors.New("storage unavailable")
	}

	svc := NewIngestionService(db, minio, "bronze", serialUploads)
	docs := []Document{
		{Key: "a.json", Data: "{}"},
		{Key: "b.json", Data: "{}"},
	}

	result, err := svc.IngestDocuments(context.Background(), 1, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", result.Status)
	}
	if result.Accepted != 0 {
		t.Errorf("expected 0 accepted, got %d", result.Accepted)
	}
	if result.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", result.Failed)
	}
}

// TestIngestDocuments_DarkDataPrevention verifies that if LogDocument fails,
// UploadJSON is never called — protecting against orphaned MinIO objects with no DB record.
func TestIngestDocuments_DarkDataPrevention(t *testing.T) {
	db := newMockDB()
	minio := newMockMinio()

	// First doc: LogDocument fails. Second doc: succeeds.
	logCalls := 0
	db.logDocumentFn = func(_ context.Context, _ int, key, _ string) error {
		logCalls++
		if logCalls == 1 {
			return errors.New("db write failed")
		}
		return nil
	}

	svc := NewIngestionService(db, minio, "bronze", serialUploads)
	docs := []Document{
		{Key: "dark.json", Data: "{}"},
		{Key: "light.json", Data: "{}"},
	}

	result, err := svc.IngestDocuments(context.Background(), 1, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The failed-log doc must never reach MinIO.
	if slices.Contains(minio.uploadedKeys, "dark.json") {
		t.Error("dark.json must not be uploaded when LogDocument fails")
	}
	if !slices.Contains(minio.uploadedKeys, "light.json") {
		t.Error("light.json must be uploaded when LogDocument succeeds")
	}

	if result.Status != "completed_with_errors" {
		t.Errorf("expected 'completed_with_errors', got %q", result.Status)
	}
}

// TestIngestDocuments_StatusUpdateFailureDoesNotFailJob is the Phase 77
// regression guard: when upload to bronze succeeds but the subsequent
// PostgreSQL status update fails, the document is already live in MinIO
// and the job must NOT be marked as failed or completed_with_errors.
func TestIngestDocuments_StatusUpdateFailureDoesNotFailJob(t *testing.T) {
	db := newMockDB()
	minio := newMockMinio()

	// Every UpdateDocumentStatus call targeting the "uploaded" status fails;
	// the "failed" transition (used only on upload errors) still succeeds,
	// so this isolates the exact semantic the fix protects.
	db.updateDocStatusFn = func(_ context.Context, key, status string) error {
		db.docStatuses[key] = status
		if status == "uploaded" {
			return errors.New("pg write timeout")
		}
		return nil
	}

	before := counterValue(t, StatusUpdateFailures)

	svc := NewIngestionService(db, minio, "bronze", serialUploads)
	docs := []Document{
		{Key: "doc-a.json", Data: "{}"},
		{Key: "doc-b.json", Data: "{}"},
	}

	result, err := svc.IngestDocuments(context.Background(), 1, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected job status 'completed' despite status-update failures, got %q", result.Status)
	}
	if result.Accepted != 2 {
		t.Errorf("expected 2 accepted (both uploaded to MinIO), got %d", result.Accepted)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if !slices.Contains(db.jobStatuses, "completed") {
		t.Errorf("expected persisted job status 'completed', got %v", db.jobStatuses)
	}
	if len(minio.uploadedKeys) != 2 {
		t.Errorf("expected both docs uploaded to bronze, got %v", minio.uploadedKeys)
	}

	after := counterValue(t, StatusUpdateFailures)
	if got := after - before; got != 2 {
		t.Errorf("expected StatusUpdateFailures counter to increment by 2, got %v", got)
	}
}

// TestJobStatusTransitions checks the three possible terminal states.
func TestJobStatusTransitions(t *testing.T) {
	cases := []struct {
		name           string
		uploadBehavior func(calls int) error
		docCount       int
		wantStatus     string
	}{
		{
			name:           "running -> completed",
			uploadBehavior: func(_ int) error { return nil },
			docCount:       2,
			wantStatus:     "completed",
		},
		{
			name:           "running -> completed_with_errors",
			uploadBehavior: func(calls int) error {
				if calls == 1 {
					return errors.New("err")
				}
				return nil
			},
			docCount:   2,
			wantStatus: "completed_with_errors",
		},
		{
			name:           "running -> failed",
			uploadBehavior: func(_ int) error { return errors.New("err") },
			docCount:       2,
			wantStatus:     "failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newMockDB()
			minio := newMockMinio()
			calls := 0
			minio.uploadFn = func(_ context.Context, _, key, _ string) error {
				calls++
				minio.uploadedKeys = append(minio.uploadedKeys, key)
				return tc.uploadBehavior(calls)
			}

			docs := make([]Document, tc.docCount)
			for i := range docs {
				docs[i] = Document{Key: "doc.json", Data: "{}"}
			}

			result, err := NewIngestionService(db, minio, "bronze", serialUploads).IngestDocuments(context.Background(), 1, docs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != tc.wantStatus {
				t.Errorf("expected %q, got %q", tc.wantStatus, result.Status)
			}
			if !slices.Contains(db.jobStatuses, tc.wantStatus) {
				t.Errorf("job status not persisted: got %v", db.jobStatuses)
			}
		})
	}
}

// TestIngestDocuments_ConcurrentUploadsPreserveOrdering is the Phase 85
// regression guard for the errgroup-based upload loop. It pins three
// invariants the batched hot path relies on:
//
//  1. Per-document outcomes are indexed by the document's position in the
//     input slice, so a mid-batch failure is attributed to the correct doc.
//  2. Concurrent uploads never under- or over-count accepted / failed totals.
//  3. Every document in the batch is actually handed to the object store
//     (no lost work when the errgroup semaphore releases slots).
//
// The fake UploadJSON sleeps just long enough (with reverse-ordered delays)
// to guarantee that completion order differs from input order — otherwise a
// silently serial loop would still pass.
func TestIngestDocuments_ConcurrentUploadsPreserveOrdering(t *testing.T) {
	const (
		batchSize          = 16
		uploadConcurrency  = 4
		stepDelay          = 2 * time.Millisecond
		failEveryNthByIndex = 3 // indices 0, 3, 6, 9, 12, 15
	)

	db := newMockDB()
	minio := newMockMinio()

	var uploadMu sync.Mutex
	minio.uploadFn = func(_ context.Context, _, key, _ string) error {
		// Reverse-ordered sleep so later-submitted docs complete first.
		// Parse the trailing index from the key (e.g. "doc-012.json") and
		// sleep (batchSize - index) * stepDelay. The delay monotonically
		// decreases as index grows — without parallelism, total runtime
		// would exceed batchSize * stepDelay.
		idx := parseDocIndex(t, key)
		time.Sleep(time.Duration(batchSize-idx) * stepDelay)

		uploadMu.Lock()
		defer uploadMu.Unlock()
		minio.uploadedKeys = append(minio.uploadedKeys, key)
		if idx%failEveryNthByIndex == 0 {
			return errors.New("synthetic upload failure")
		}
		return nil
	}

	docs := make([]Document, batchSize)
	for i := range docs {
		docs[i] = Document{Key: fmtDocKey(i), Data: "{}"}
	}

	svc := NewIngestionService(db, minio, "bronze", uploadConcurrency)

	start := time.Now()
	result, err := svc.IngestDocuments(context.Background(), 1, docs)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sanity: every doc was actually handed to UploadJSON.
	if len(minio.uploadedKeys) != batchSize {
		t.Fatalf("expected %d UploadJSON calls, got %d", batchSize, len(minio.uploadedKeys))
	}

	// Accepted / failed totals must match the deterministic failure pattern.
	expectedFailed := 0
	for i := range batchSize {
		if i%failEveryNthByIndex == 0 {
			expectedFailed++
		}
	}
	if result.Failed != expectedFailed {
		t.Errorf("expected %d failed, got %d", expectedFailed, result.Failed)
	}
	if result.Accepted != batchSize-expectedFailed {
		t.Errorf("expected %d accepted, got %d", batchSize-expectedFailed, result.Accepted)
	}

	// Per-index ordering invariant: docStatuses must reflect the failure
	// pattern position-by-position (i.e. the i-th failure is attributed to
	// the i-th failing doc, not a neighbour).
	for i, doc := range docs {
		got := db.docStatuses[doc.Key]
		want := "uploaded"
		if i%failEveryNthByIndex == 0 {
			want = "failed"
		}
		if got != want {
			t.Errorf("doc %q (index %d): expected %q, got %q", doc.Key, i, want, got)
		}
	}

	// Concurrency sanity: the sum of all per-doc sleeps is
	// sum(batchSize..1) * stepDelay. A fully serial loop takes that long.
	// With uploadConcurrency workers we expect roughly 1/uploadConcurrency
	// of that — use half the serial budget as a loose lower-bound guard.
	serialBudget := time.Duration(batchSize*(batchSize+1)/2) * stepDelay
	if elapsed >= serialBudget/2 {
		t.Errorf("batch ran too slowly for concurrency=%d: elapsed=%v, serial-budget=%v",
			uploadConcurrency, elapsed, serialBudget)
	}
}

func fmtDocKey(i int) string {
	return "doc-" + padIndex(i) + ".json"
}

func padIndex(i int) string {
	s := []byte{'0', '0', '0'}
	for j := len(s) - 1; j >= 0 && i > 0; j-- {
		s[j] = byte('0' + i%10)
		i /= 10
	}
	return string(s)
}

func parseDocIndex(t *testing.T, key string) int {
	t.Helper()
	// key format: "doc-NNN.json"
	if len(key) < len("doc-000.json") {
		t.Fatalf("malformed doc key: %q", key)
	}
	n := 0
	for _, b := range key[4:7] {
		n = n*10 + int(b-'0')
	}
	return n
}
