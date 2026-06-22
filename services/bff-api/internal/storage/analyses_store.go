package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AnalysesStore is the write/read path for saved Workbench analyses + their
// identity-based shares (Phase 135 / ADR-040). Runs under the bff_auth role
// (the same write role as the auth tables).
type AnalysesStore struct {
	db *sql.DB
}

// NewAnalysesStore wraps the bff_auth-scoped *sql.DB handle.
func NewAnalysesStore(db *sql.DB) *AnalysesStore {
	return &AnalysesStore{db: db}
}

// ErrGranteeNotFound is returned when an email to share with maps to no account.
var ErrGranteeNotFound = errors.New("grantee not found")

// ErrCannotShareWithSelf is returned when the owner tries to share with itself.
var ErrCannotShareWithSelf = errors.New("cannot share with self")

// AnalysisListItem is the light list projection (no state blob).
type AnalysisListItem struct {
	ID          string
	Name        string
	Description string
	OwnerName   string // live-joined display name (first+last), email fallback
	OwnerEmail  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Permission  string // "editable" | "readable"
	Owned       bool
}

// Analysis is the full record, including the serialized Workbench state.
type Analysis struct {
	ID          string
	Name        string
	Description string
	State       string
	OwnerName   string // live-joined display name (first+last), email fallback
	OwnerEmail  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Permission  string
	Owned       bool
}

// ShareItem is one grantee of an analysis.
type ShareItem struct {
	GranteeID string
	Email     string
	CanEdit   bool
}

const permissionCase = `
	CASE WHEN a.owner_id = $1::uuid THEN 'editable'
	     WHEN s.can_edit THEN 'editable'
	     ELSE 'readable' END`

// ownerNameExpr is the live-joined owner display name (first + last, space-
// joined and trimmed), falling back to the email when no name is set — so a
// name change propagates to every analysis it owns, never snapshotted.
const ownerNameExpr = `COALESCE(NULLIF(TRIM(u.first_name || ' ' || u.last_name), ''), u.email)`

// ListVisible returns the analyses the user owns OR has been granted, newest
// activity first, with the viewer's derived permission and the owner's email.
func (s *AnalysesStore) ListVisible(ctx context.Context, userID string) ([]AnalysisListItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id::text, a.name, a.description, u.email,
		       `+ownerNameExpr+` AS owner_name, a.created_at, a.updated_at,`+
		permissionCase+`, (a.owner_id = $1::uuid) AS owned
		FROM saved_analyses a
		JOIN users u ON a.owner_id = u.id
		LEFT JOIN saved_analysis_shares s
		       ON s.analysis_id = a.id AND s.grantee_user_id = $1::uuid
		WHERE a.owner_id = $1::uuid OR s.grantee_user_id = $1::uuid
		ORDER BY a.updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list analyses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AnalysisListItem
	for rows.Next() {
		var it AnalysisListItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &it.OwnerEmail, &it.OwnerName,
			&it.CreatedAt, &it.UpdatedAt, &it.Permission, &it.Owned); err != nil {
			return nil, fmt.Errorf("scan analysis: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Get returns one analysis (incl. state) if the user owns it or is a grantee,
// or (nil, nil) when not visible / not found.
func (s *AnalysesStore) Get(ctx context.Context, id, userID string) (*Analysis, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT a.id::text, a.name, a.description, a.state, u.email,
		       `+ownerNameExpr+` AS owner_name, a.created_at, a.updated_at,`+
		permissionCase+`, (a.owner_id = $1::uuid) AS owned
		FROM saved_analyses a
		JOIN users u ON a.owner_id = u.id
		LEFT JOIN saved_analysis_shares s
		       ON s.analysis_id = a.id AND s.grantee_user_id = $1::uuid
		WHERE a.id = $2::uuid AND (a.owner_id = $1::uuid OR s.grantee_user_id = $1::uuid)`,
		userID, id)
	var a Analysis
	if err := row.Scan(&a.ID, &a.Name, &a.Description, &a.State, &a.OwnerEmail, &a.OwnerName,
		&a.CreatedAt, &a.UpdatedAt, &a.Permission, &a.Owned); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidUUIDErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get analysis: %w", err)
	}
	return &a, nil
}

// Create inserts a new analysis owned by the user and returns its list item.
func (s *AnalysesStore) Create(ctx context.Context, ownerID, name, description, state string) (AnalysisListItem, error) {
	var it AnalysisListItem
	err := s.db.QueryRowContext(ctx, `
		WITH ins AS (
			INSERT INTO saved_analyses (owner_id, name, description, state)
			VALUES ($1::uuid, $2, $3, $4)
			RETURNING id, name, description, created_at, updated_at
		)
		SELECT ins.id::text, ins.name, ins.description, u.email,
		       `+ownerNameExpr+` AS owner_name, ins.created_at, ins.updated_at
		FROM ins JOIN users u ON u.id = $1::uuid`,
		ownerID, name, description, state).Scan(
		&it.ID, &it.Name, &it.Description, &it.OwnerEmail, &it.OwnerName, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return AnalysisListItem{}, fmt.Errorf("create analysis: %w", err)
	}
	it.Permission = "editable"
	it.Owned = true
	return it, nil
}

// CountOwned reports how many analyses the user owns, for the per-user row cap
// (SEC-016).
func (s *AnalysesStore) CountOwned(ctx context.Context, ownerID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM saved_analyses WHERE owner_id = $1::uuid`, ownerID).Scan(&n)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("count owned analyses: %w", err)
	}
	return n, nil
}

// Update changes name/description/state if the user is the owner or a can_edit
// grantee. Reports whether a row was updated (false → not found / no permission).
func (s *AnalysesStore) Update(ctx context.Context, id, userID, name, description, state string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE saved_analyses a
		SET name = $3, description = $4, state = $5, updated_at = now()
		WHERE a.id = $1::uuid
		  AND (a.owner_id = $2::uuid
		       OR EXISTS (SELECT 1 FROM saved_analysis_shares s
		                  WHERE s.analysis_id = a.id AND s.grantee_user_id = $2::uuid AND s.can_edit))`,
		id, userID, name, description, state)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("update analysis: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Delete removes an analysis the user OWNS (grantees cannot delete). Cascades to
// shares. Reports whether a row was deleted.
func (s *AnalysesStore) Delete(ctx context.Context, id, userID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM saved_analyses WHERE id = $1::uuid AND owner_id = $2::uuid`, id, userID)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete analysis: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// IsOwner reports whether the user owns the analysis (gate for share endpoints).
func (s *AnalysesStore) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM saved_analyses WHERE id = $1::uuid AND owner_id = $2::uuid)`,
		id, userID).Scan(&ok)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("check owner: %w", err)
	}
	return ok, nil
}

// ListShares returns the grantees of an analysis (caller must be the owner).
func (s *AnalysesStore) ListShares(ctx context.Context, analysisID string) ([]ShareItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id::text, u.email, s.can_edit
		FROM saved_analysis_shares s JOIN users u ON s.grantee_user_id = u.id
		WHERE s.analysis_id = $1::uuid
		ORDER BY u.email`, analysisID)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ShareItem
	for rows.Next() {
		var sh ShareItem
		if err := rows.Scan(&sh.GranteeID, &sh.Email, &sh.CanEdit); err != nil {
			return nil, fmt.Errorf("scan share: %w", err)
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// AddShare grants access to the user with granteeEmail (the caller must own the
// analysis; ownership is verified by the caller). Upserts can_edit. Returns
// ErrGranteeNotFound / ErrCannotShareWithSelf.
func (s *AnalysesStore) AddShare(ctx context.Context, analysisID, ownerID, granteeEmail string, canEdit bool) (ShareItem, error) {
	var granteeID, email string
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, email FROM users WHERE lower(email) = lower($1)`, granteeEmail).Scan(&granteeID, &email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShareItem{}, ErrGranteeNotFound
		}
		return ShareItem{}, fmt.Errorf("resolve grantee: %w", err)
	}
	if granteeID == ownerID {
		return ShareItem{}, ErrCannotShareWithSelf
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO saved_analysis_shares (analysis_id, grantee_user_id, can_edit)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (analysis_id, grantee_user_id) DO UPDATE SET can_edit = EXCLUDED.can_edit`,
		analysisID, granteeID, canEdit); err != nil {
		return ShareItem{}, fmt.Errorf("add share: %w", err)
	}
	return ShareItem{GranteeID: granteeID, Email: email, CanEdit: canEdit}, nil
}

// RemoveShare revokes a grantee (caller must own the analysis; verified by the
// caller). Reports whether a row was removed.
func (s *AnalysesStore) RemoveShare(ctx context.Context, analysisID, granteeID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM saved_analysis_shares WHERE analysis_id = $1::uuid AND grantee_user_id = $2::uuid`,
		analysisID, granteeID)
	if err != nil {
		if isInvalidUUIDErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("remove share: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
