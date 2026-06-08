package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnStore persists registered passkey credentials and the short-lived
// ceremony state (ADR-040). Runs under the bff_auth write role.
type WebAuthnStore struct {
	db *sql.DB
}

func NewWebAuthnStore(db *sql.DB) *WebAuthnStore {
	return &WebAuthnStore{db: db}
}

// CredentialMeta is the UI-facing projection of a credential (no key material).
type CredentialMeta struct {
	ID         string
	Name       sql.NullString
	CreatedAt  time.Time
	LastUsedAt sql.NullTime
}

// SaveCredential stores a freshly registered credential and returns its
// display metadata (row id + timestamps).
func (s *WebAuthnStore) SaveCredential(ctx context.Context, userID string, cred *webauthn.Credential, name string) (CredentialMeta, error) {
	raw, err := json.Marshal(cred)
	if err != nil {
		return CredentialMeta{}, fmt.Errorf("marshal credential: %w", err)
	}
	var m CredentialMeta
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO webauthn_credentials (user_id, credential_id, credential, name, sign_count)
		 VALUES ($1::uuid, $2, $3, $4, $5)
		 RETURNING id::text, name, created_at, last_used_at`,
		userID, cred.ID, raw, sql.NullString{String: name, Valid: name != ""},
		int64(cred.Authenticator.SignCount)).Scan(&m.ID, &m.Name, &m.CreatedAt, &m.LastUsedAt)
	if err != nil {
		return CredentialMeta{}, fmt.Errorf("save credential: %w", err)
	}
	return m, nil
}

// CredentialsByUser returns the user's registered credentials (for the ceremony
// user adapter).
func (s *WebAuthnStore) CredentialsByUser(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT credential FROM webauthn_credentials WHERE user_id = $1::uuid`, userID)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query credentials: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []webauthn.Credential
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		var c webauthn.Credential
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("unmarshal credential: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// HasCredentials reports whether the user has at least one passkey (for the
// 2FA-required policy decision).
func (s *WebAuthnStore) HasCredentials(ctx context.Context, userID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM webauthn_credentials WHERE user_id = $1::uuid`, userID).Scan(&n)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, err
	}
	return n > 0, nil
}

// ListCredentialMeta returns the user's credentials for display (no key bytes).
func (s *WebAuthnStore) ListCredentialMeta(ctx context.Context, userID string) ([]CredentialMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id::text, name, created_at, last_used_at
		 FROM webauthn_credentials WHERE user_id = $1::uuid ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list credential meta: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CredentialMeta
	for rows.Next() {
		var m CredentialMeta
		if err := rows.Scan(&m.ID, &m.Name, &m.CreatedAt, &m.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan credential meta: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateCredential persists an updated credential (sign counter after assertion).
func (s *WebAuthnStore) UpdateCredential(ctx context.Context, cred *webauthn.Credential) error {
	raw, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("marshal credential: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE webauthn_credentials
		 SET credential = $2, sign_count = $3, last_used_at = now()
		 WHERE credential_id = $1`,
		cred.ID, raw, int64(cred.Authenticator.SignCount))
	return err
}

// DeleteCredential removes one of the user's credentials by its row id. Reports
// whether a row was deleted (false → not found / not owned).
func (s *WebAuthnStore) DeleteCredential(ctx context.Context, userID, credentialRowID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM webauthn_credentials WHERE id = $1::uuid AND user_id = $2::uuid`,
		credentialRowID, userID)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete credential: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// SaveCeremony upserts the ephemeral SessionData for a (user, purpose) ceremony.
func (s *WebAuthnStore) SaveCeremony(ctx context.Context, userID, purpose string, sd *webauthn.SessionData, expires time.Time) error {
	raw, err := json.Marshal(sd)
	if err != nil {
		return fmt.Errorf("marshal session data: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO webauthn_ceremonies (user_id, purpose, session_data, expires_at)
		 VALUES ($1::uuid, $2, $3, $4)
		 ON CONFLICT (user_id, purpose)
		 DO UPDATE SET session_data = EXCLUDED.session_data, expires_at = EXCLUDED.expires_at`,
		userID, purpose, raw, expires)
	return err
}

// ConsumeCeremony atomically returns and deletes a valid (unexpired) ceremony's
// SessionData. Returns (nil, nil) if absent/expired.
func (s *WebAuthnStore) ConsumeCeremony(ctx context.Context, userID, purpose string) (*webauthn.SessionData, error) {
	row := s.db.QueryRowContext(ctx,
		`DELETE FROM webauthn_ceremonies
		 WHERE user_id = $1::uuid AND purpose = $2 AND expires_at > now()
		 RETURNING session_data`, userID, purpose)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidUUIDErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("consume ceremony: %w", err)
	}
	var sd webauthn.SessionData
	if err := json.Unmarshal(raw, &sd); err != nil {
		return nil, fmt.Errorf("unmarshal session data: %w", err)
	}
	return &sd, nil
}
