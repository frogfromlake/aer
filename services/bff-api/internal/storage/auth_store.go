package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
)

// ErrEmailExists is returned by CreateInvitedUser when the email is already
// registered (Postgres unique-violation 23505).
var ErrEmailExists = errors.New("email already exists")

// isInvalidUUIDErr reports whether err is a Postgres invalid-text-representation
// (22P02) — i.e. a malformed UUID path parameter. The caller maps it to "not
// found" rather than a 500.
func isInvalidUUIDErr(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

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
	FirstName    string
	LastName     string
	Role         string
	Status       string
	PasswordHash sql.NullString
}

const userColumns = `id::text, email, first_name, last_name, role, status, password_hash`

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
	if err := row.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.Status, &u.PasswordHash); err != nil {
		// No row, or a malformed UUID path parameter — both "not found".
		if errors.Is(err, sql.ErrNoRows) || isInvalidUUIDErr(err) {
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

// UpdateUserNames sets the user's display name (self-service edit, Phase 148e).
// Read live everywhere the name shows, so the change propagates without a
// snapshot.
func (s *AuthStore) UpdateUserNames(ctx context.Context, id, firstName, lastName string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET first_name = $2, last_name = $3, updated_at = now() WHERE id = $1::uuid`,
		id, firstName, lastName)
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

// SessionInfo is one of a user's active sessions, for their own
// active-sessions view (SEC-005). IDHash is the sha256 of the cookie id — it
// NEVER leaves the BFF; the handler uses it only to mark the caller's current
// session.
type SessionInfo struct {
	IDHash     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	UserAgent  string
}

// ListUserSessions returns a user's currently-active (non-revoked, unexpired)
// sessions, most-recently-seen first (SEC-005).
func (s *AuthStore) ListUserSessions(ctx context.Context, userID string) ([]SessionInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_at, last_seen_at, user_agent
		FROM sessions
		WHERE user_id = $1::uuid
		  AND revoked_at IS NULL
		  AND idle_expires_at > now()
		  AND absolute_expires_at > now()
		ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SessionInfo
	for rows.Next() {
		var si SessionInfo
		var ua sql.NullString
		if err := rows.Scan(&si.IDHash, &si.CreatedAt, &si.LastSeenAt, &ua); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		si.UserAgent = ua.String
		out = append(out, si)
	}
	return out, rows.Err()
}

// InvalidateUserTokens marks every still-unconsumed token of the given purpose
// for a user as consumed, so a freshly-issued token becomes the only live one
// (SEC-022 — collapse repeated forgot-password requests to a single outstanding
// reset link). bff_auth already holds UPDATE on auth_tokens (ConsumeToken).
func (s *AuthStore) InvalidateUserTokens(ctx context.Context, userID, purpose string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE auth_tokens SET consumed_at = now()
		 WHERE user_id = $1::uuid AND purpose = $2 AND consumed_at IS NULL`,
		userID, purpose)
	if err != nil {
		return fmt.Errorf("invalidate user tokens: %w", err)
	}
	return nil
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

// ConsumeTokenAndActivate consumes a single-use invite token and activates the
// user in one transaction (SEC-078): a partial failure can no longer burn the
// token while leaving the account un-activated. Returns ("", nil) when the
// token is invalid / expired / already consumed (transaction rolled back).
func (s *AuthStore) ConsumeTokenAndActivate(ctx context.Context, tokenHash, passwordHash, firstName, lastName string) (string, error) {
	return s.consumeTokenTx(ctx, tokenHash, "invite", func(ctx context.Context, tx *sql.Tx, userID string) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE users SET password_hash = $2, first_name = $3, last_name = $4, status = 'active',
			        responsible_use_accepted_at = now(), updated_at = now()
			 WHERE id = $1::uuid`, userID, passwordHash, firstName, lastName)
		return err
	})
}

// ConsumeTokenAndResetPassword consumes a single-use reset token, sets the new
// password, and revokes all of the user's sessions in one transaction
// (SEC-078): the password change and the session revocation co-commit, so a
// partial failure can neither burn the token nor leave stale sessions live.
// Returns ("", nil) for an invalid token (transaction rolled back).
func (s *AuthStore) ConsumeTokenAndResetPassword(ctx context.Context, tokenHash, passwordHash string) (string, error) {
	return s.consumeTokenTx(ctx, tokenHash, "password_reset", func(ctx context.Context, tx *sql.Tx, userID string) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1::uuid`,
			userID, passwordHash); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE sessions SET revoked_at = now() WHERE user_id = $1::uuid AND revoked_at IS NULL`,
			userID)
		return err
	})
}

// consumeTokenTx burns the single-use token and applies the supplied follow-up
// writes atomically in one transaction. The token-consume UPDATE and apply()
// share the same *sql.Tx, so either all writes commit or none do. apply() runs
// only when a valid token was consumed; an invalid token returns ("", nil) with
// the transaction rolled back. Password hashing is deliberately done by the
// caller before this call so the CPU-bound argon2 work never holds the tx open.
func (s *AuthStore) consumeTokenTx(
	ctx context.Context,
	tokenHash, purpose string,
	apply func(context.Context, *sql.Tx, string) error,
) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin token tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op once Commit succeeds

	var userID string
	err = tx.QueryRowContext(ctx, `
		UPDATE auth_tokens SET consumed_at = now()
		WHERE token_hash = $1 AND purpose = $2
		  AND consumed_at IS NULL AND expires_at > now()
		RETURNING user_id::text`, tokenHash, purpose).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil // invalid token — rollback leaves it unconsumed
		}
		return "", fmt.Errorf("consume token: %w", err)
	}

	if err := apply(ctx, tx, userID); err != nil {
		return "", fmt.Errorf("apply token effect: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit token tx: %w", err)
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

// --- DSGVO (Phase 134 / ADR-040) --------------------------------------------

// UserExport is the complete, privacy-minimal record AĒR holds about a user
// (DSGVO Art. 15 / 20). No analytical-activity data exists to export.
type UserExport struct {
	ID                       string
	Email                    string
	Role                     string
	Status                   string
	CreatedAt                time.Time
	ResponsibleUseAcceptedAt sql.NullTime
	ActiveSessionCount       int
	LastSeenAt               sql.NullTime
}

// ExportUser returns everything stored about the user, or (nil, nil) if absent.
func (s *AuthStore) ExportUser(ctx context.Context, id string) (*UserExport, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.email, u.role, u.status, u.created_at, u.responsible_use_accepted_at,
		       (SELECT count(*) FROM sessions s
		         WHERE s.user_id = u.id AND s.revoked_at IS NULL
		           AND s.idle_expires_at > now() AND s.absolute_expires_at > now()),
		       (SELECT max(s.last_seen_at) FROM sessions s WHERE s.user_id = u.id)
		FROM users u WHERE u.id = $1::uuid`, id)
	var e UserExport
	if err := row.Scan(&e.ID, &e.Email, &e.Role, &e.Status, &e.CreatedAt,
		&e.ResponsibleUseAcceptedAt, &e.ActiveSessionCount, &e.LastSeenAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidUUIDErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("export user: %w", err)
	}
	return &e, nil
}

// DeleteUser permanently deletes a user (DSGVO Art. 17). Sessions and pending
// auth_tokens cascade-delete via their FK. Reports whether a row was deleted.
func (s *AuthStore) DeleteUser(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1::uuid`, id)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete user: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// --- admin (Phase 134 / ADR-040) --------------------------------------------

// AdminUserRow is the admin projection of a user (no password material).
type AdminUserRow struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
	Role      string
	Status    string
	CreatedAt time.Time
}

// CreateInvitedUser inserts an invited (not-yet-activated) user with the given
// role and returns its id. Returns ErrEmailExists on a duplicate email.
func (s *AuthStore) CreateInvitedUser(ctx context.Context, email, role string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, role, status) VALUES ($1, $2, 'invited') RETURNING id::text`,
		email, role).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrEmailExists
		}
		return "", fmt.Errorf("create invited user: %w", err)
	}
	return id, nil
}

// ListUsers returns all users, oldest first.
func (s *AuthStore) ListUsers(ctx context.Context) ([]AdminUserRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id::text, email, first_name, last_name, role, status, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AdminUserRow
	for rows.Next() {
		var u AdminUserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetUserStatus updates a user's status and reports whether a row was affected
// (false → no such user / malformed id, which the handler maps to 404).
func (s *AuthStore) SetUserStatus(ctx context.Context, id, status string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET status = $2, updated_at = now() WHERE id = $1::uuid`, id, status)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("set user status: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
