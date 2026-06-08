package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
)

// AuthStore is the write path for the Phase-134 auth tables (ADR-040). It runs
// under the dedicated `bff_auth` Postgres role (DML on users / sessions /
// auth_tokens only) — a separate, small pool from the read-only analytics pool.
type AuthStore struct {
	db *sql.DB
}

// NewAuthStore wraps an *sql.DB opened under the bff_auth role.
func NewAuthStore(db *sql.DB) *AuthStore {
	return &AuthStore{db: db}
}

// AuthUser is the auth-relevant projection of a user row. PasswordHash is null
// while a user is invited-but-not-activated (and reserved null for future
// SSO-only accounts).
type AuthUser struct {
	ID           string
	Email        string
	Role         string
	Status       string
	PasswordHash sql.NullString
}

const userColumns = `id::text, email, role, status, password_hash`

// GetUserByEmail returns the user whose email matches case-insensitively, or
// (nil, nil) if none exists.
func (s *AuthStore) GetUserByEmail(ctx context.Context, email string) (*AuthUser, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email)
	return scanUser(row)
}

// GetUserByID returns the user with the given id, or (nil, nil) if none exists.
func (s *AuthStore) GetUserByID(ctx context.Context, id string) (*AuthUser, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1::uuid`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*AuthUser, error) {
	var u AuthUser
	if err := row.Scan(&u.ID, &u.Email, &u.Role, &u.Status, &u.PasswordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// ActivateUser sets the password, marks the account active, and records the
// responsible-use consent timestamp (LICENSE §3.2.b) — the invite-acceptance
// transition.
func (s *AuthStore) ActivateUser(ctx context.Context, id, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, status = 'active',
		        responsible_use_accepted_at = now(), updated_at = now()
		 WHERE id = $1::uuid`, id, passwordHash)
	return err
}

// UpdateUserPassword replaces the password hash (reset / change flows).
func (s *AuthStore) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1::uuid`,
		id, passwordHash)
	return err
}

// CreateSession inserts a server-side session keyed by the sha256 of the
// opaque cookie id.
func (s *AuthStore) CreateSession(ctx context.Context, idHash, userID string, idleExp, absExp time.Time, userAgent string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, idle_expires_at, absolute_expires_at, user_agent)
		 VALUES ($1, $2::uuid, $3, $4, $5)`,
		idHash, userID, idleExp, absExp, sql.NullString{String: userAgent, Valid: userAgent != ""})
	return err
}

// ValidateAndTouchSession validates the session by its hashed id, slides the
// idle expiry (bounded by the absolute cap), and returns the identity in a
// single statement. Returns (nil, nil) when the session is missing / expired /
// revoked or the user is not active — so a suspended user's sessions stop
// working immediately (LICENSE §3.3).
func (s *AuthStore) ValidateAndTouchSession(ctx context.Context, idHash string, idleTTL time.Duration) (*auth.Identity, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE sessions s
		SET last_seen_at = now(),
		    idle_expires_at = LEAST(now() + make_interval(secs => $2), s.absolute_expires_at)
		FROM users u
		WHERE s.id = $1
		  AND s.user_id = u.id
		  AND s.revoked_at IS NULL
		  AND s.idle_expires_at > now()
		  AND s.absolute_expires_at > now()
		  AND u.status = 'active'
		RETURNING u.id::text, u.email, u.role`,
		idHash, int64(idleTTL.Seconds()))

	id := &auth.Identity{SessionIDHash: idHash}
	var role string
	if err := row.Scan(&id.UserID, &id.Email, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("validate session: %w", err)
	}
	id.Role = auth.Role(role)
	return id, nil
}

// RevokeSession revokes a single session (logout).
func (s *AuthStore) RevokeSession(ctx context.Context, idHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, idHash)
	return err
}

// RevokeAllUserSessions revokes every session for a user (password reset,
// admin revocation).
func (s *AuthStore) RevokeAllUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE user_id = $1::uuid AND revoked_at IS NULL`,
		userID)
	return err
}

// RevokeOtherUserSessions revokes every session for a user EXCEPT the one
// identified by keepIDHash (password change keeps the current session live).
func (s *AuthStore) RevokeOtherUserSessions(ctx context.Context, userID, keepIDHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = now()
		 WHERE user_id = $1::uuid AND id <> $2 AND revoked_at IS NULL`,
		userID, keepIDHash)
	return err
}

// CreateToken inserts a single-use, hashed invite / password-reset token.
func (s *AuthStore) CreateToken(ctx context.Context, userID, purpose, tokenHash string, exp time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
		 VALUES ($1::uuid, $2, $3, $4)`,
		userID, purpose, tokenHash, exp)
	return err
}

// ConsumeToken atomically marks a valid (unconsumed, unexpired, matching
// purpose) token consumed and returns its user id. Returns ("", nil) when the
// token is invalid — single-use is enforced by the consumed_at guard.
func (s *AuthStore) ConsumeToken(ctx context.Context, tokenHash, purpose string) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE auth_tokens SET consumed_at = now()
		WHERE token_hash = $1 AND purpose = $2
		  AND consumed_at IS NULL AND expires_at > now()
		RETURNING user_id::text`, tokenHash, purpose)
	var userID string
	if err := row.Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("consume token: %w", err)
	}
	return userID, nil
}

// DeleteExpiredSessions purges sessions past their absolute cap and sessions
// revoked more than a day ago. Best-effort housekeeping; returns the row count.
func (s *AuthStore) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE absolute_expires_at < now()
		   OR (revoked_at IS NOT NULL AND revoked_at < now() - interval '1 day')`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
