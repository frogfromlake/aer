package storage

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestWebAuthnStore_CredentialRoundTrip(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	ws := NewWebAuthnStore(s.db)

	cred := &webauthn.Credential{
		ID:        []byte{0x01, 0x02, 0x03},
		PublicKey: []byte{0xaa, 0xbb},
	}
	cred.Authenticator.SignCount = 5
	meta0, err := ws.SaveCredential(ctx, uid, cred, "My Yubikey")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if meta0.ID == "" || !meta0.Name.Valid {
		t.Fatalf("expected returned meta with id + name, got %+v", meta0)
	}

	has, err := ws.HasCredentials(ctx, uid)
	if err != nil || !has {
		t.Fatalf("expected user to have credentials, has=%v err=%v", has, err)
	}

	creds, err := ws.CredentialsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(creds) != 1 || string(creds[0].ID) != string(cred.ID) || creds[0].Authenticator.SignCount != 5 {
		t.Fatalf("unexpected loaded credential: %+v", creds)
	}

	// Update sign counter.
	creds[0].Authenticator.SignCount = 9
	if err := ws.UpdateCredential(ctx, &creds[0]); err != nil {
		t.Fatalf("update: %v", err)
	}
	creds2, _ := ws.CredentialsByUser(ctx, uid)
	if creds2[0].Authenticator.SignCount != 9 {
		t.Fatalf("expected sign count 9, got %d", creds2[0].Authenticator.SignCount)
	}

	// Meta listing + delete.
	meta, err := ws.ListCredentialMeta(ctx, uid)
	if err != nil || len(meta) != 1 || !meta[0].Name.Valid || meta[0].Name.String != "My Yubikey" {
		t.Fatalf("unexpected meta: %+v err=%v", meta, err)
	}
	deleted, err := ws.DeleteCredential(ctx, uid, meta[0].ID)
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if has, _ := ws.HasCredentials(ctx, uid); has {
		t.Fatal("expected no credentials after delete")
	}
}

func TestWebAuthnStore_Ceremony(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	ws := NewWebAuthnStore(s.db)

	sd := &webauthn.SessionData{Challenge: "abc123", UserID: []byte(uid)}
	if err := ws.SaveCeremony(ctx, uid, "register", sd, time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("save ceremony: %v", err)
	}
	// Upsert: a second begin overwrites.
	sd2 := &webauthn.SessionData{Challenge: "xyz789", UserID: []byte(uid)}
	if err := ws.SaveCeremony(ctx, uid, "register", sd2, time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("upsert ceremony: %v", err)
	}
	got, err := ws.ConsumeCeremony(ctx, uid, "register")
	if err != nil || got == nil {
		t.Fatalf("consume: got=%v err=%v", got, err)
	}
	if got.Challenge != "xyz789" {
		t.Fatalf("expected latest challenge, got %q", got.Challenge)
	}
	// Single-use: second consume → nil.
	if again, _ := ws.ConsumeCeremony(ctx, uid, "register"); again != nil {
		t.Fatal("expected ceremony consumed (single-use)")
	}
}

func TestWebAuthnStore_ExpiredCeremonyRejected(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "a@b.c", "active")
	ws := NewWebAuthnStore(s.db)

	sd := &webauthn.SessionData{Challenge: "old"}
	if err := ws.SaveCeremony(ctx, uid, "login", sd, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got, _ := ws.ConsumeCeremony(ctx, uid, "login"); got != nil {
		t.Fatal("expected expired ceremony to be rejected")
	}
}
