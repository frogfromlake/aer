package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// silverBucket is the canonical Silver-layer bucket name. Hard-coded
// here (and mirrored in the analysis worker) because the bucket layout is
// part of the Medallion contract — it never changes per-environment.
const silverBucket = "silver"

// bronzeBucket is the canonical Bronze-layer bucket name. Bronze stores the raw
// HTML verbatim under the SAME deterministic object key as Silver (different
// bucket). Phase 148c stopped duplicating that raw HTML into Silver, so the L5
// "Raw" view sources it from Bronze on-demand via GetBronzeRawHTML.
const bronzeBucket = "bronze"

// SilverEnvelope mirrors the worker's `SilverEnvelope` Pydantic shape.
// Only the fields the BFF actually surfaces on the article-detail
// endpoint are decoded; SilverMeta is kept as raw JSON because it is
// source-specific and intentionally unstable (see ADR-015).
type SilverEnvelope struct {
	Core                 SilverCore        `json:"core"`
	Meta                 map[string]any    `json:"meta,omitempty"`
	ExtractionProvenance map[string]string `json:"extraction_provenance"`
}

// SilverCore is the universal-minimum Silver contract.
type SilverCore struct {
	DocumentID    string `json:"document_id"`
	Source        string `json:"source"`
	SourceType    string `json:"source_type"`
	RawText       string `json:"raw_text"`
	CleanedText   string `json:"cleaned_text"`
	Language      string `json:"language"`
	Timestamp     string `json:"timestamp"`
	URL           string `json:"url"`
	SchemaVersion int    `json:"schema_version"`
	WordCount     int    `json:"word_count"`
}

// ErrSilverNotFound signals that the requested Silver object does not
// exist (translated to HTTP 404 by the handler). Distinct from a
// transport / auth failure so the handler can return generic 5xx in the
// latter case without leaking implementation details.
var ErrSilverNotFound = errors.New("silver object not found")

// ErrBronzeNotFound signals the Bronze object is gone — typically because it
// aged past the 90-day Bronze TTL. The L5 "Raw" view treats this as "raw source
// no longer available" (cleaned text still shows), never as an error.
var ErrBronzeNotFound = errors.New("bronze object not found")

// SilverStore reads SilverEnvelope objects from MinIO. The BFF's account
// holds GetObject only — a misconfigured store cannot mutate Silver.
type SilverStore struct {
	client *minio.Client
}

// NewSilverStore constructs a SilverStore over the BFF MinIO service
// account. `endpoint` must include the host:port (no scheme).
func NewSilverStore(endpoint, accessKey, secretKey string, useSSL bool) (*SilverStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}
	return &SilverStore{client: client}, nil
}

// GetEnvelope fetches and decodes a Silver envelope by bronze object key
// (which is also the Silver object key — see worker silver.upload_silver).
func (s *SilverStore) GetEnvelope(ctx context.Context, objectKey string) (*SilverEnvelope, error) {
	obj, err := s.client.GetObject(ctx, silverBucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio GetObject: %w", err)
	}
	defer func() { _ = obj.Close() }()

	body, err := io.ReadAll(obj)
	if err != nil {
		// MinIO surfaces "object not found" as a read-time error; map it
		// onto the typed sentinel so the handler can distinguish 404 from
		// 500.
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == "NoSuchKey" {
			return nil, ErrSilverNotFound
		}
		return nil, fmt.Errorf("read silver object: %w", err)
	}

	var env SilverEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode silver envelope: %w", err)
	}
	return &env, nil
}

// GetBronzeRawHTML fetches the raw HTML for a document from the Bronze layer by
// the same object key the Silver envelope uses (Bronze + Silver share the
// deterministic key, different bucket). Phase 148c: raw HTML is no longer
// duplicated into Silver, so the L5 "Raw" view sources it here on-demand. Returns
// ErrBronzeNotFound when the object has aged past the 90-day Bronze TTL.
func (s *SilverStore) GetBronzeRawHTML(ctx context.Context, objectKey string) (string, error) {
	obj, err := s.client.GetObject(ctx, bronzeBucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("minio GetObject (bronze): %w", err)
	}
	defer func() { _ = obj.Close() }()

	body, err := io.ReadAll(obj)
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == "NoSuchKey" {
			return "", ErrBronzeNotFound
		}
		return "", fmt.Errorf("read bronze object: %w", err)
	}

	return parseBronzeRawHTML(body)
}

// parseBronzeRawHTML extracts raw_html from a Bronze envelope payload. Split out
// from the MinIO I/O so the decode is unit-testable without a round-trip (the
// GetObject path itself needs live MinIO and is exercised in integration).
func parseBronzeRawHTML(body []byte) (string, error) {
	// The Bronze object is the crawler's raw envelope; only raw_html is needed.
	var env struct {
		RawHTML string `json:"raw_html"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("decode bronze envelope: %w", err)
	}
	return env.RawHTML, nil
}
