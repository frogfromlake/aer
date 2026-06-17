package storage

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// DossierStore — sources metadata over Postgres.
// ---------------------------------------------------------------------------

func TestDossierStore_FetchSources_MetadataAndClassification(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	id := insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)
	insertSource(t, db, ctx, "elysee", "web", "https://elysee.fr", false)

	// Latest classification wins (two dates for tagesschau).
	if _, err := db.ExecContext(ctx, `INSERT INTO source_classifications
		(source_id, primary_function, secondary_function, emic_designation, emic_context, classified_by, classification_date)
		VALUES ($1, 'epistemic_authority', NULL, 'old desig', 'old ctx', 'r', '2026-01-01'),
		       ($1, 'epistemic_authority', 'cohesion_identity', 'Tagesschau', 'public broadcaster', 'r', '2026-05-01')`,
		id); err != nil {
		t.Fatalf("seed classifications: %v", err)
	}

	store := NewDossierStore(db, nil) // nil ch — counts fall back to empty.
	rows, err := store.FetchSources(ctx, []string{"tagesschau", "elysee"}, nil, nil)
	if err != nil {
		t.Fatalf("FetchSources: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// Ordered by name: elysee, tagesschau.
	if rows[0].Name != "elysee" || rows[1].Name != "tagesschau" {
		t.Errorf("order: want [elysee tagesschau], got [%s %s]", rows[0].Name, rows[1].Name)
	}
	tag := rows[1]
	if !tag.SilverEligible {
		t.Error("tagesschau should be silver-eligible")
	}
	if !tag.EmicDesignation.Valid || tag.EmicDesignation.String != "Tagesschau" {
		t.Errorf("latest classification should win: got %v", tag.EmicDesignation)
	}
	if !tag.SecondaryFunction.Valid || tag.SecondaryFunction.String != "cohesion_identity" {
		t.Errorf("secondary function: want cohesion_identity, got %v", tag.SecondaryFunction)
	}
	// elysee has no classification → NULL emic fields, counts zero.
	if rows[0].EmicDesignation.Valid {
		t.Errorf("elysee should have NULL emic designation, got %v", rows[0].EmicDesignation)
	}
	if rows[0].ArticlesTotal != 0 {
		t.Errorf("elysee count should be 0 with nil ch, got %d", rows[0].ArticlesTotal)
	}
}

func TestDossierStore_FetchSources_EmptyInput(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	store := NewDossierStore(db, nil)
	rows, err := store.FetchSources(ctx, nil, nil, nil)
	if err != nil {
		t.Fatalf("FetchSources: %v", err)
	}
	if rows != nil {
		t.Errorf("empty input must return nil, got %v", rows)
	}
}

func TestDossierStore_ResolveSource(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	id := insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)
	store := NewDossierStore(db, nil)

	// By name.
	gotID, name, err := store.ResolveSource(ctx, "tagesschau")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if gotID != id || name != "tagesschau" {
		t.Errorf("resolve by name: want (%d, tagesschau), got (%d, %q)", id, gotID, name)
	}

	// By numeric id.
	gotID, name, err = store.ResolveSource(ctx, itoa(int(id)))
	if err != nil {
		t.Fatalf("resolve by id: %v", err)
	}
	if gotID != id || name != "tagesschau" {
		t.Errorf("resolve by id: want (%d, tagesschau), got (%d, %q)", id, gotID, name)
	}

	// Unknown → ErrSourceNotFound.
	if _, _, err := store.ResolveSource(ctx, "nonesuch"); err != ErrSourceNotFound {
		t.Errorf("unknown source: want ErrSourceNotFound, got %v", err)
	}
}

func TestDossierStore_ResolveSourceWithEligibility(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	id := insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)
	store := NewDossierStore(db, nil)

	row, err := store.ResolveSourceWithEligibility(ctx, "tagesschau")
	if err != nil {
		t.Fatalf("resolve with eligibility: %v", err)
	}
	if row.ID != id || row.Name != "tagesschau" || row.Type != "web" {
		t.Errorf("eligibility row mismatch: %+v", row)
	}
	if !row.SilverEligible {
		t.Error("tagesschau should be silver-eligible")
	}

	if _, err := store.ResolveSourceWithEligibility(ctx, "nonesuch"); err != ErrSourceNotFound {
		t.Errorf("unknown source: want ErrSourceNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SourceStore — cached /sources list over Postgres.
// ---------------------------------------------------------------------------

func TestSourceStore_ListAndCache(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	insertSource(t, db, ctx, "bundesregierung", "web", "https://bundesregierung.de", false)
	insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)

	store := NewSourceStore(db, time.Minute)
	rows, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 sources, got %d", len(rows))
	}
	// Ordered by name.
	if rows[0].Name != "bundesregierung" || rows[1].Name != "tagesschau" {
		t.Errorf("order: got [%s %s]", rows[0].Name, rows[1].Name)
	}
	if !rows[1].SilverEligible {
		t.Error("tagesschau should be silver-eligible")
	}
	if rows[1].URL == nil || *rows[1].URL != "https://tagesschau.de" {
		t.Errorf("url not mapped: %v", rows[1].URL)
	}

	// Insert a new source; within TTL the cached snapshot is returned unchanged.
	insertSource(t, db, ctx, "elysee", "web", "https://elysee.fr", false)
	cached, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List (cached): %v", err)
	}
	if len(cached) != 2 {
		t.Errorf("cache hit should still show 2 sources, got %d", len(cached))
	}
}

func TestSourceStore_StaleCacheFallbackOnError(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)

	// Zero TTL forces a refresh on every call.
	store := NewSourceStore(db, time.Nanosecond)
	if _, err := store.List(ctx); err != nil {
		t.Fatalf("prime: %v", err)
	}
	// Drop the table so the next fetch fails; the store must serve the
	// last good snapshot instead of erroring. CASCADE clears the FK-dependent
	// crawler/classification tables created by setupSourcesDB.
	if _, err := db.ExecContext(ctx, "DROP TABLE sources CASCADE"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	time.Sleep(2 * time.Nanosecond)
	rows, err := store.List(ctx)
	if err != nil {
		t.Fatalf("expected stale-cache fallback, got error: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("stale snapshot should still hold 1 source, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// GetDiscoveryCoverage
// ---------------------------------------------------------------------------

func TestGetDiscoveryCoverage_PerChannelAndAlert(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	id := insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)

	now := time.Now().UTC()
	older := now.Add(-2 * time.Hour)
	// Two runs: an older run + the most recent run. Two channels.
	runs := []struct {
		channel    string
		discovered int
		dedup      int
		started    time.Time
	}{
		{"sitemap", 100, 90, older},
		{"sitemap", 120, 110, now}, // last run
		{"rss", 30, 28, now},       // last run
	}
	for _, r := range runs {
		if _, err := db.ExecContext(ctx, `INSERT INTO crawler_discovery_runs
			(run_id, source_id, channel, urls_discovered, urls_after_dedup, run_started_at, run_completed_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $5)`,
			id, r.channel, r.discovered, r.dedup, r.started); err != nil {
			t.Fatalf("insert run: %v", err)
		}
	}
	// An active underflow alert.
	if _, err := db.ExecContext(ctx, `INSERT INTO crawler_discovery_alerts
		(source_id, alert_type, first_observed_at, last_observed_at, consecutive_runs, expected_floor, last_urls_observed)
		VALUES ($1, 'underflow', $2, $2, 2, 50, 30)`, id, now); err != nil {
		t.Fatalf("insert alert: %v", err)
	}

	store := NewDossierStore(db, nil)
	summary, err := store.GetDiscoveryCoverage(ctx, id, 30)
	if err != nil {
		t.Fatalf("GetDiscoveryCoverage: %v", err)
	}
	if summary.WindowDays != 30 {
		t.Errorf("window days: want 30, got %d", summary.WindowDays)
	}
	if len(summary.PerChannel) != 2 {
		t.Fatalf("want 2 channels, got %d: %+v", len(summary.PerChannel), summary.PerChannel)
	}
	// Last run totals across both channels: sitemap 120 + rss 30 = 150 discovered.
	if summary.TotalDiscoveredLastRun != 150 {
		t.Errorf("total last-run discovered: want 150, got %d", summary.TotalDiscoveredLastRun)
	}
	if summary.UniqueAfterDedupLastRun != 138 { // 110 + 28
		t.Errorf("unique after dedup last run: want 138, got %d", summary.UniqueAfterDedupLastRun)
	}
	// Per-channel average over the window: sitemap = (100+120)/2 = 110.
	byChannel := map[string]DiscoveryCoverageRow{}
	for _, c := range summary.PerChannel {
		byChannel[c.Channel] = c
	}
	if byChannel["sitemap"].AverageDiscoveredPerRun < 109.9 || byChannel["sitemap"].AverageDiscoveredPerRun > 110.1 {
		t.Errorf("sitemap avg discovered: want 110, got %v", byChannel["sitemap"].AverageDiscoveredPerRun)
	}
	if byChannel["sitemap"].LastRunDiscovered != 120 {
		t.Errorf("sitemap last-run discovered: want 120, got %d", byChannel["sitemap"].LastRunDiscovered)
	}
	if !summary.UnderflowAlertActive {
		t.Error("expected an active underflow alert")
	}
	if !summary.ExpectedFloorPerRun.Valid || summary.ExpectedFloorPerRun.Int64 != 50 {
		t.Errorf("expected floor: want 50, got %v", summary.ExpectedFloorPerRun)
	}
}

func TestGetDiscoveryCoverage_NoAlertNoRuns(t *testing.T) {
	db, ctx := setupSourcesDB(t)
	id := insertSource(t, db, ctx, "tagesschau", "web", "https://tagesschau.de", true)

	store := NewDossierStore(db, nil)
	summary, err := store.GetDiscoveryCoverage(ctx, id, 0) // 0 → defaults to 30
	if err != nil {
		t.Fatalf("GetDiscoveryCoverage: %v", err)
	}
	if summary.WindowDays != 30 {
		t.Errorf("zero window must default to 30, got %d", summary.WindowDays)
	}
	if len(summary.PerChannel) != 0 {
		t.Errorf("no runs → no channels, got %+v", summary.PerChannel)
	}
	if summary.UnderflowAlertActive {
		t.Error("no alert row → UnderflowAlertActive must be false")
	}
}

// ---------------------------------------------------------------------------
// AuthStore — DeleteExpiredSessions / UpdateUserPassword.
// ---------------------------------------------------------------------------

func TestAuthStore_DeleteExpiredSessions(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "purge@example.org", "active")

	now := time.Now()
	// Live session — survives.
	if err := s.CreateSession(ctx, "live", uid, now.Add(time.Hour), now.Add(24*time.Hour), "agent"); err != nil {
		t.Fatalf("create live session: %v", err)
	}
	// Past its absolute cap — purged.
	if err := s.CreateSession(ctx, "expired", uid, now.Add(-2*time.Hour), now.Add(-time.Hour), "agent"); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	// Revoked more than a day ago — purged.
	if err := s.CreateSession(ctx, "old-revoked", uid, now.Add(time.Hour), now.Add(24*time.Hour), "agent"); err != nil {
		t.Fatalf("create revoked session: %v", err)
	}
	if _, err := s.db.ExecContext(ctx,
		"UPDATE sessions SET revoked_at = now() - interval '2 days' WHERE id = 'old-revoked'"); err != nil {
		t.Fatalf("backdate revoke: %v", err)
	}

	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 purged sessions (expired + old-revoked), got %d", n)
	}
	// The live session must still validate.
	id, err := s.ValidateAndTouchSession(ctx, "live", 8*time.Hour)
	if err != nil {
		t.Fatalf("validate live: %v", err)
	}
	if id == nil {
		t.Error("live session should survive the purge")
	}
}

func TestAuthStore_UpdateUserPassword(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "pwchange@example.org", "active")

	if err := s.UpdateUserPassword(ctx, uid, "$argon2id$new-hash"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	var hash string
	if err := s.db.QueryRowContext(ctx,
		"SELECT password_hash FROM users WHERE id = $1::uuid", uid).Scan(&hash); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if hash != "$argon2id$new-hash" {
		t.Errorf("password hash not updated, got %q", hash)
	}
}

// ---------------------------------------------------------------------------
// SilverStore — construction (GetEnvelope needs MinIO, skipped).
// ---------------------------------------------------------------------------

func TestNewSilverStore_Construction(t *testing.T) {
	store, err := NewSilverStore("localhost:9000", "key", "secret", false)
	if err != nil {
		t.Fatalf("NewSilverStore: %v", err)
	}
	if store == nil || store.client == nil {
		t.Fatal("expected a constructed SilverStore with a client")
	}
}
