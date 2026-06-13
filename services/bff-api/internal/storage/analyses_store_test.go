package storage

import (
	"errors"
	"testing"
)

func TestAnalysesStore_CRUDAndVisibility(t *testing.T) {
	s, ctx := setupAuthStore(t)
	owner := seedUser(t, s, ctx, "owner@x.y", "active")
	grantee := seedUser(t, s, ctx, "grantee@x.y", "active")
	stranger := seedUser(t, s, ctx, "stranger@x.y", "active")
	as := NewAnalysesStore(s.db)

	created, err := as.Create(ctx, owner, "My analysis", "desc", "?activePillar=aleph")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Permission != "editable" || !created.Owned || created.OwnerEmail != "owner@x.y" {
		t.Fatalf("unexpected created item: %+v", created)
	}

	// Owner sees it with state.
	a, err := as.Get(ctx, created.ID, owner)
	if err != nil || a == nil || a.State != "?activePillar=aleph" || a.Permission != "editable" {
		t.Fatalf("owner get: %+v err=%v", a, err)
	}

	// A stranger cannot see it.
	if got, _ := as.Get(ctx, created.ID, stranger); got != nil {
		t.Fatal("stranger must not see the analysis")
	}

	// List for owner shows it; for stranger empty.
	if items, _ := as.ListVisible(ctx, owner); len(items) != 1 {
		t.Fatalf("owner list expected 1, got %d", len(items))
	}
	if items, _ := as.ListVisible(ctx, stranger); len(items) != 0 {
		t.Fatalf("stranger list expected 0, got %d", len(items))
	}

	// Share read-only with grantee.
	if _, err := as.AddShare(ctx, created.ID, owner, "grantee@x.y", false); err != nil {
		t.Fatalf("add share: %v", err)
	}
	g, _ := as.Get(ctx, created.ID, grantee)
	if g == nil || g.Permission != "readable" || g.Owned {
		t.Fatalf("grantee should have readable access, got %+v", g)
	}
	// Read-only grantee cannot edit.
	if ok, _ := as.Update(ctx, created.ID, grantee, "hacked", "", "x"); ok {
		t.Fatal("read-only grantee must not be able to edit")
	}
	// Grantee cannot delete.
	if ok, _ := as.Delete(ctx, created.ID, grantee); ok {
		t.Fatal("grantee must not be able to delete")
	}

	// Upgrade to editable; now grantee can edit but still not delete.
	if _, err := as.AddShare(ctx, created.ID, owner, "grantee@x.y", true); err != nil {
		t.Fatalf("upgrade share: %v", err)
	}
	if ok, _ := as.Update(ctx, created.ID, grantee, "edited by grantee", "d2", "?x=1"); !ok {
		t.Fatal("editable grantee should be able to edit")
	}
	g2, _ := as.Get(ctx, created.ID, grantee)
	if g2.Permission != "editable" || g2.Name != "edited by grantee" {
		t.Fatalf("expected editable + edited name, got %+v", g2)
	}

	// Owner removes the share → grantee loses access.
	if ok, _ := as.RemoveShare(ctx, created.ID, grantee); !ok {
		t.Fatal("expected share removed")
	}
	if got, _ := as.Get(ctx, created.ID, grantee); got != nil {
		t.Fatal("grantee should lose access after revoke")
	}

	// Owner deletes → gone (cascades shares).
	if ok, _ := as.Delete(ctx, created.ID, owner); !ok {
		t.Fatal("owner delete should succeed")
	}
	if got, _ := as.Get(ctx, created.ID, owner); got != nil {
		t.Fatal("analysis should be gone")
	}
}

func TestAnalysesStore_ShareErrors(t *testing.T) {
	s, ctx := setupAuthStore(t)
	owner := seedUser(t, s, ctx, "owner@x.y", "active")
	as := NewAnalysesStore(s.db)
	created, _ := as.Create(ctx, owner, "A", "", "?s=1")

	// Unknown email → ErrGranteeNotFound.
	if _, err := as.AddShare(ctx, created.ID, owner, "nobody@x.y", false); !errors.Is(err, ErrGranteeNotFound) {
		t.Fatalf("expected ErrGranteeNotFound, got %v", err)
	}
	// Sharing with self → ErrCannotShareWithSelf.
	if _, err := as.AddShare(ctx, created.ID, owner, "owner@x.y", false); !errors.Is(err, ErrCannotShareWithSelf) {
		t.Fatalf("expected ErrCannotShareWithSelf, got %v", err)
	}
}

func TestAnalysesStore_IsOwnerAndListShares(t *testing.T) {
	s, ctx := setupAuthStore(t)
	owner := seedUser(t, s, ctx, "owner@x.y", "active")
	other := seedUser(t, s, ctx, "other@x.y", "active")
	g := seedUser(t, s, ctx, "g@x.y", "active")
	as := NewAnalysesStore(s.db)
	created, _ := as.Create(ctx, owner, "A", "", "?s=1")
	_, _ = as.AddShare(ctx, created.ID, owner, "g@x.y", true)

	if ok, _ := as.IsOwner(ctx, created.ID, owner); !ok {
		t.Fatal("owner should be owner")
	}
	if ok, _ := as.IsOwner(ctx, created.ID, other); ok {
		t.Fatal("other is not owner")
	}
	shares, err := as.ListShares(ctx, created.ID)
	if err != nil || len(shares) != 1 || shares[0].Email != "g@x.y" || !shares[0].CanEdit {
		t.Fatalf("unexpected shares: %+v err=%v", shares, err)
	}
	_ = g
}
