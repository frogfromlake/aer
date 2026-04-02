package core

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// --- Mock implementations ---

type mockMetadataStore struct {
	createJobFn       func(ctx context.Context, sourceID int) (int, error)
	updateJobStatusFn func(ctx context.Context, jobID int, status string) error
	logDocumentFn     func(ctx context.Context, jobID int, key, traceID string) error
	updateDocStatusFn func(ctx context.Context, key, status string) error
	pingFn            func(ctx context.Context) error

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
	m.pingFn = func(_ context.Context) error { return nil }
	return m
}

func (m *mockMetadataStore) CreateIngestionJob(ctx context.Context, sourceID int) (int, error) {
	return m.createJobFn(ctx, sourceID)
}
func (m *mockMetadataStore) UpdateJobStatus(ctx context.Context, jobID int, status string) error {
	return m.updateJobStatusFn(ctx, jobID, status)
}
func (m *mockMetadataStore) LogDocument(ctx context.Context, jobID int, key, traceID string) error {
	return m.logDocumentFn(ctx, jobID, key, traceID)
}
func (m *mockMetadataStore) UpdateDocumentStatus(ctx context.Context, key, status string) error {
	return m.updateDocStatusFn(ctx, key, status)
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
	svc := NewIngestionService(newMockDB(), newMockMinio())
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
	svc := NewIngestionService(db, newMockMinio())
	_, err := svc.IngestDocuments(context.Background(), 1, []Document{{Key: "a.json", Data: "{}"}})
	if err == nil {
		t.Fatal("expected error when job creation fails")
	}
}

func TestIngestDocuments_AllSuccess(t *testing.T) {
	db := newMockDB()
	minio := newMockMinio()
	svc := NewIngestionService(db, minio)

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

	svc := NewIngestionService(db, minio)
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

	svc := NewIngestionService(db, minio)
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

	svc := NewIngestionService(db, minio)
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

			result, err := NewIngestionService(db, minio).IngestDocuments(context.Background(), 1, docs)
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
