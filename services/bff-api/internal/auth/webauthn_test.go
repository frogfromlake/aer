package auth

import (
	"bytes"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestNewWebAuthn_ValidConfig(t *testing.T) {
	w, err := NewWebAuthn("localhost", "AĒR", []string{"https://localhost"})
	if err != nil {
		t.Fatalf("expected valid config to construct, got error: %v", err)
	}
	if w == nil {
		t.Fatal("expected a non-nil *webauthn.WebAuthn")
	}
}

// The go-webauthn config validator rejects a relying party with no origins.
// (It does not validate RPID / display-name / origin-format, so those are not
// error cases for this constructor.)
func TestNewWebAuthn_InvalidConfig(t *testing.T) {
	cases := []struct {
		name      string
		rpOrigins []string
	}{
		{"nil origins", nil},
		{"empty origins", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewWebAuthn("localhost", "AĒR", tc.rpOrigins)
			if err == nil {
				t.Fatalf("expected error for %s, got %+v", tc.name, w)
			}
			if w != nil {
				t.Fatalf("expected nil WebAuthn on error, got %+v", w)
			}
		})
	}
}

func TestWebAuthnUser_Accessors(t *testing.T) {
	creds := []webauthn.Credential{
		{ID: []byte("cred-1")},
		{ID: []byte("cred-2")},
	}
	u := &WebAuthnUser{
		ID:    "8f14e45f-ceea-467d-9a0d-1a2b3c4d5e6f",
		Email: "researcher@aer.example",
		Creds: creds,
	}

	if got := u.WebAuthnID(); !bytes.Equal(got, []byte(u.ID)) {
		t.Fatalf("WebAuthnID = %q, want %q", got, u.ID)
	}
	if got := u.WebAuthnName(); got != u.Email {
		t.Fatalf("WebAuthnName = %q, want %q", got, u.Email)
	}
	if got := u.WebAuthnDisplayName(); got != u.Email {
		t.Fatalf("WebAuthnDisplayName = %q, want %q", got, u.Email)
	}
	got := u.WebAuthnCredentials()
	if len(got) != 2 || !bytes.Equal(got[0].ID, []byte("cred-1")) || !bytes.Equal(got[1].ID, []byte("cred-2")) {
		t.Fatalf("WebAuthnCredentials = %+v, want the two seeded creds", got)
	}
}

func TestWebAuthnUser_EmptyCredentials(t *testing.T) {
	u := &WebAuthnUser{ID: "id", Email: "e@x"}
	if got := u.WebAuthnCredentials(); len(got) != 0 {
		t.Fatalf("expected no credentials, got %d", len(got))
	}
}
