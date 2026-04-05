package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_NewEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if store.HasSeen("feed1", "guid1") {
		t.Error("fresh store should not have seen any GUIDs")
	}
}

func TestStore_MarkAndCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	store.MarkSeen("feed1", "guid-abc")

	if !store.HasSeen("feed1", "guid-abc") {
		t.Error("should have seen guid-abc after marking")
	}
	if store.HasSeen("feed1", "guid-xyz") {
		t.Error("should not have seen guid-xyz")
	}
	if store.HasSeen("feed2", "guid-abc") {
		t.Error("guid-abc in feed2 should not be marked")
	}
}

func TestStore_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Create and populate store
	store1, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	store1.MarkSeen("feed1", "guid-1")
	store1.MarkSeen("feed1", "guid-2")
	store1.MarkSeen("feed2", "guid-3")

	if err := store1.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("state file should exist after Save")
	}

	// Reload from disk
	store2, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (reload) failed: %v", err)
	}

	if !store2.HasSeen("feed1", "guid-1") {
		t.Error("reloaded store should have guid-1")
	}
	if !store2.HasSeen("feed1", "guid-2") {
		t.Error("reloaded store should have guid-2")
	}
	if !store2.HasSeen("feed2", "guid-3") {
		t.Error("reloaded store should have guid-3")
	}
	if store2.HasSeen("feed1", "guid-never-added") {
		t.Error("reloaded store should not have guid-never-added")
	}
}

func TestStore_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := NewStore(path)
	if err == nil {
		t.Error("expected error for corrupt state file")
	}
}
