package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/frogfromlake/aer/pkg/testutils"
)

// setupAuthStore starts a Postgres testcontainer, applies the REAL Phase-134
// auth migration (so this test is a regression guard on the migration itself,
// not a hand-mirrored DDL copy), and returns a store wired to it.
func setupAuthStore(t *testing.T) (*AuthStore, context.Context) {
	t.Helper()
	ctx := context.Background()

	pgImage, err := testutils.GetImageFromCompose("postgres")
	if err != nil {
		t.Fatalf("get postgres image from compose: %v", err)
	}

	pgContainer, err := postgres.Run(ctx,
		pgImage,
		postgres.WithDatabase("aer_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432/tcp", "pgx/v5", func(host string, port nat.Port) string {
				return fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=aer_test sslmode=disable", host, port.Port())
			}).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("open pgx: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Apply the real auth migrations, resolved relative to this test file so
	// the path is stable regardless of the test's working directory.
	_, thisFile, _, _ := runtime.Caller(0)
	migDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..",
		"infra", "postgres", "migrations")
	for _, name := range []string{"000024_auth_schema.up.sql", "000025_webauthn.up.sql"} {
		migSQL, err := os.ReadFile(filepath.Join(migDir, name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := db.ExecContext(ctx, string(migSQL)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}

	return NewAuthStore(db), ctx
}

// seedUser inserts an active user with a known password hash placeholder and
// returns its id.
func seedUser(t *testing.T, s *AuthStore, ctx context.Context, email, status string) string {
	t.Helper()
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, role, status, responsible_use_accepted_at)
		 VALUES ($1, '$argon2id$placeholder', 'researcher', $2, now())
		 RETURNING id::text`, email, status).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func TestAuthStore_GetUserByEmailIsCaseInsensitive(t *testing.T) {
	s, ctx := setupAuthStore(t)
	seedUser(t, s, ctx, "Alice@Example.org", "active")

	u, err := s.GetUserByEmail(ctx, "alice@example.org")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u == nil {
		t.Fatal("expected case-insensitive match")
	}
	if u.Role != "researcher" || u.Status != "active" {
		t.Fatalf("unexpected user projection: %+v", u)
	}

	missing, err := s.GetUserByEmail(ctx, "nobody@example.org")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for unknown email")
	}
}

func TestAuthStore_SessionLifecycle(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")

	now := time.Now()
	if err := s.CreateSession(ctx, "hash-valid", uid, now.Add(time.Hour), now.Add(24*time.Hour), "agent"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	id, err := s.ValidateAndTouchSession(ctx, "hash-valid", 8*time.Hour)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if id == nil || id.UserID != uid || id.Role != "researcher" {
		t.Fatalf("expected valid identity for uid %s, got %+v", uid, id)
	}
	if id.SessionIDHash != "hash-valid" {
		t.Fatalf("expected session hash on identity, got %q", id.SessionIDHash)
	}

	// Revoked → invalid.
	if err := s.RevokeSession(ctx, "hash-valid"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	id, err = s.ValidateAndTouchSession(ctx, "hash-valid", 8*time.Hour)
	if err != nil {
		t.Fatalf("validate after revoke: %v", err)
	}
	if id != nil {
		t.Fatal("expected nil identity for revoked session")
	}
}

func TestAuthStore_ExpiredSessionRejected(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")

	now := time.Now()
	if err := s.CreateSession(ctx, "hash-expired", uid, now.Add(-time.Minute), now.Add(24*time.Hour), ""); err != nil {
		t.Fatalf("create session: %v", err)
	}
	id, err := s.ValidateAndTouchSession(ctx, "hash-expired", 8*time.Hour)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if id != nil {
		t.Fatal("expected nil identity for idle-expired session")
	}
}

func TestAuthStore_SuspendedUserSessionsStopImmediately(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	now := time.Now()
	if err := s.CreateSession(ctx, "hash-x", uid, now.Add(time.Hour), now.Add(24*time.Hour), ""); err != nil {
		t.Fatalf("create session: %v", err)
	}
	// Valid while active.
	if id, _ := s.ValidateAndTouchSession(ctx, "hash-x", time.Hour); id == nil {
		t.Fatal("expected valid before suspension")
	}
	// Suspend → existing session must stop validating (LICENSE §3.3).
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET status='suspended' WHERE id=$1::uuid`, uid); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	id, err := s.ValidateAndTouchSession(ctx, "hash-x", time.Hour)
	if err != nil {
		t.Fatalf("validate after suspend: %v", err)
	}
	if id != nil {
		t.Fatal("expected suspended user's session to stop validating immediately")
	}
}

func TestAuthStore_IdleSlideBoundedByAbsoluteCap(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	now := time.Now()
	// Absolute cap is sooner than the idle TTL would push idle to.
	abs := now.Add(2 * time.Minute)
	if err := s.CreateSession(ctx, "hash-cap", uid, now.Add(time.Minute), abs, ""); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := s.ValidateAndTouchSession(ctx, "hash-cap", 8*time.Hour); err != nil {
		t.Fatalf("validate: %v", err)
	}
	var idle, absStored time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT idle_expires_at, absolute_expires_at FROM sessions WHERE id='hash-cap'`).Scan(&idle, &absStored); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if idle.After(absStored) {
		t.Fatalf("idle expiry %v must not exceed absolute cap %v", idle, absStored)
	}
}

func TestAuthStore_TokenIsSingleUse(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")

	if err := s.CreateToken(ctx, uid, "password_reset", "tok-hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create token: %v", err)
	}
	got, err := s.ConsumeToken(ctx, "tok-hash", "password_reset")
	if err != nil {
		t.Fatalf("consume first: %v", err)
	}
	if got != uid {
		t.Fatalf("expected user id %s, got %q", uid, got)
	}
	// Second consume must fail (single-use).
	got, err = s.ConsumeToken(ctx, "tok-hash", "password_reset")
	if err != nil {
		t.Fatalf("consume second: %v", err)
	}
	if got != "" {
		t.Fatal("expected empty on second consume (single-use)")
	}
	// Wrong purpose must not match.
	if err := s.CreateToken(ctx, uid, "invite", "tok-invite", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create invite token: %v", err)
	}
	if got, _ := s.ConsumeToken(ctx, "tok-invite", "password_reset"); got != "" {
		t.Fatal("expected purpose mismatch to yield empty")
	}
}

func TestAuthStore_ExpiredTokenRejected(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	if err := s.CreateToken(ctx, uid, "invite", "tok-old", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if got, _ := s.ConsumeToken(ctx, "tok-old", "invite"); got != "" {
		t.Fatal("expected expired token to yield empty")
	}
}

func TestAuthStore_ActivateAndResetRevoke(t *testing.T) {
	s, ctx := setupAuthStore(t)
	// Invited user (no password yet).
	var uid string
	if err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, role, status) VALUES ('new@x.y','researcher','invited') RETURNING id::text`).Scan(&uid); err != nil {
		t.Fatalf("seed invited: %v", err)
	}

	if err := s.ActivateUser(ctx, uid, "$argon2id$new"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	u, _ := s.GetUserByID(ctx, uid)
	if u == nil || u.Status != "active" || !u.PasswordHash.Valid {
		t.Fatalf("expected activated user with password, got %+v", u)
	}

	// Two live sessions, then RevokeAll.
	now := time.Now()
	_ = s.CreateSession(ctx, "s1", uid, now.Add(time.Hour), now.Add(24*time.Hour), "")
	_ = s.CreateSession(ctx, "s2", uid, now.Add(time.Hour), now.Add(24*time.Hour), "")
	if err := s.RevokeAllUserSessions(ctx, uid); err != nil {
		t.Fatalf("revoke all: %v", err)
	}
	for _, h := range []string{"s1", "s2"} {
		if id, _ := s.ValidateAndTouchSession(ctx, h, time.Hour); id != nil {
			t.Fatalf("expected session %s revoked", h)
		}
	}
}

func TestAuthStore_ExportUser(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	now := time.Now()
	_ = s.CreateSession(ctx, "sx", uid, now.Add(time.Hour), now.Add(24*time.Hour), "")

	e, err := s.ExportUser(ctx, uid)
	if err != nil || e == nil {
		t.Fatalf("export: e=%v err=%v", e, err)
	}
	if e.Email != "a@b.c" || e.ActiveSessionCount != 1 {
		t.Fatalf("unexpected export: %+v", e)
	}
	if !e.ResponsibleUseAcceptedAt.Valid {
		t.Fatal("expected consent timestamp (seedUser sets it)")
	}
	// Unknown / malformed id → nil, nil.
	if got, _ := s.ExportUser(ctx, "not-a-uuid"); got != nil {
		t.Fatal("expected nil for malformed uuid")
	}
}

func TestAuthStore_DeleteUserCascades(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	now := time.Now()
	_ = s.CreateSession(ctx, "s1", uid, now.Add(time.Hour), now.Add(24*time.Hour), "")
	_ = s.CreateToken(ctx, uid, "password_reset", "tk", now.Add(time.Hour))

	deleted, err := s.DeleteUser(ctx, uid)
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	// User gone.
	if u, _ := s.GetUserByID(ctx, uid); u != nil {
		t.Fatal("expected user removed")
	}
	// Sessions + tokens cascade-deleted.
	var sessions, tokens int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1::uuid`, uid).Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM auth_tokens WHERE user_id=$1::uuid`, uid).Scan(&tokens); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if sessions != 0 || tokens != 0 {
		t.Fatalf("expected cascade delete, sessions=%d tokens=%d", sessions, tokens)
	}
	// Deleting again → no-op.
	if again, _ := s.DeleteUser(ctx, uid); again {
		t.Fatal("expected second delete to be a no-op")
	}
}

func TestAuthStore_CreateInvitedUserAndDuplicate(t *testing.T) {
	s, ctx := setupAuthStore(t)

	id, err := s.CreateInvitedUser(ctx, "Invited@Example.org", "admin")
	if err != nil {
		t.Fatalf("create invited: %v", err)
	}
	u, _ := s.GetUserByID(ctx, id)
	if u == nil || u.Role != "admin" || u.Status != "invited" || u.PasswordHash.Valid {
		t.Fatalf("unexpected invited user: %+v", u)
	}
	// Duplicate email (case-insensitive) → ErrEmailExists.
	if _, err := s.CreateInvitedUser(ctx, "invited@example.org", "researcher"); err != ErrEmailExists {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}

func TestAuthStore_ListUsers(t *testing.T) {
	s, ctx := setupAuthStore(t)
	seedUser(t, s, ctx, "a@b.c", "active")
	seedUser(t, s, ctx, "d@e.f", "active")

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].CreatedAt.IsZero() {
		t.Fatal("expected createdAt to be populated")
	}
}

func TestAuthStore_SetUserStatus(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")

	ok, err := s.SetUserStatus(ctx, uid, "suspended")
	if err != nil || !ok {
		t.Fatalf("expected update to succeed, ok=%v err=%v", ok, err)
	}
	u, _ := s.GetUserByID(ctx, uid)
	if u.Status != "suspended" {
		t.Fatalf("expected suspended, got %s", u.Status)
	}

	// Unknown (but valid) UUID → no row.
	ok, err = s.SetUserStatus(ctx, "11111111-1111-1111-1111-111111111111", "active")
	if err != nil || ok {
		t.Fatalf("expected no-op for unknown uuid, ok=%v err=%v", ok, err)
	}
	// Malformed UUID → not-found, not a 500-class error.
	ok, err = s.SetUserStatus(ctx, "not-a-uuid", "active")
	if err != nil || ok {
		t.Fatalf("expected no-op for malformed uuid, ok=%v err=%v", ok, err)
	}
}

func TestAuthStore_RevokeOtherKeepsCurrent(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	now := time.Now()
	_ = s.CreateSession(ctx, "keep", uid, now.Add(time.Hour), now.Add(24*time.Hour), "")
	_ = s.CreateSession(ctx, "drop", uid, now.Add(time.Hour), now.Add(24*time.Hour), "")

	if err := s.RevokeOtherUserSessions(ctx, uid, "keep"); err != nil {
		t.Fatalf("revoke other: %v", err)
	}
	if id, _ := s.ValidateAndTouchSession(ctx, "keep", time.Hour); id == nil {
		t.Fatal("expected current session to survive")
	}
	if id, _ := s.ValidateAndTouchSession(ctx, "drop", time.Hour); id != nil {
		t.Fatal("expected sibling session to be revoked")
	}
}
