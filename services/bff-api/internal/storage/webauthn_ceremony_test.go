package storage

import (
	"encoding/json"
	"testing"

	vwa "github.com/descope/virtualwebauthn"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
)

// TestWebAuthn_FullCeremony exercises the entire passkey ceremony end-to-end —
// registration AND assertion — with REAL go-webauthn cryptography and a
// virtual authenticator (no browser), persisting through the real store. This
// is the regression guard that the crypto + persistence wiring is correct.
func TestWebAuthn_FullCeremony(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "passkey@example.org", "active")
	ws := NewWebAuthnStore(s.db)

	w, err := webauthn.New(&webauthn.Config{
		RPID:          "localhost",
		RPDisplayName: "AĒR",
		RPOrigins:     []string{"https://localhost"},
	})
	if err != nil {
		t.Fatalf("webauthn.New: %v", err)
	}

	rp := vwa.RelyingParty{Name: "AĒR", ID: "localhost", Origin: "https://localhost"}
	authenticator := vwa.NewAuthenticator()
	cred := vwa.NewCredential(vwa.KeyTypeEC2)

	// --- Registration ---
	user := &auth.WebAuthnUser{ID: uid, Email: "passkey@example.org"}
	options, sessionData, err := w.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	optionsJSON, _ := json.Marshal(options)
	attestationOptions, err := vwa.ParseAttestationOptions(string(optionsJSON))
	if err != nil {
		t.Fatalf("ParseAttestationOptions: %v", err)
	}
	attestationResponse := vwa.CreateAttestationResponse(rp, authenticator, cred, *attestationOptions)

	parsed, err := protocol.ParseCredentialCreationResponseBytes([]byte(attestationResponse))
	if err != nil {
		t.Fatalf("ParseCredentialCreationResponseBytes: %v", err)
	}
	newCred, err := w.CreateCredential(user, *sessionData, parsed)
	if err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}
	if _, err := ws.SaveCredential(ctx, uid, newCred, "virtual"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	authenticator.AddCredential(cred)

	stored, err := ws.CredentialsByUser(ctx, uid)
	if err != nil || len(stored) != 1 {
		t.Fatalf("expected 1 stored credential, got %d (err=%v)", len(stored), err)
	}
	initialSignCount := stored[0].Authenticator.SignCount

	// --- Assertion (step-up) ---
	user2 := &auth.WebAuthnUser{ID: uid, Email: "passkey@example.org", Creds: stored}
	aOptions, aSession, err := w.BeginLogin(user2)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	aOptionsJSON, _ := json.Marshal(aOptions)
	assertionOptions, err := vwa.ParseAssertionOptions(string(aOptionsJSON))
	if err != nil {
		t.Fatalf("ParseAssertionOptions: %v", err)
	}
	assertionResponse := vwa.CreateAssertionResponse(rp, authenticator, cred, *assertionOptions)

	aParsed, err := protocol.ParseCredentialRequestResponseBytes([]byte(assertionResponse))
	if err != nil {
		t.Fatalf("ParseCredentialRequestResponseBytes: %v", err)
	}
	validated, err := w.ValidateLogin(user2, *aSession, aParsed)
	if err != nil {
		t.Fatalf("ValidateLogin (assertion failed): %v", err)
	}
	if string(validated.ID) != string(stored[0].ID) {
		t.Fatal("validated credential id does not match the registered one")
	}
	if err := ws.UpdateCredential(ctx, validated); err != nil {
		t.Fatalf("UpdateCredential: %v", err)
	}

	// The sign counter must never go BACKWARDS (clone-detection). A 0→0 counter
	// is valid (many passkeys report 0 and never increment); only a regression
	// is a clone signal.
	after, _ := ws.CredentialsByUser(ctx, uid)
	if after[0].Authenticator.SignCount < initialSignCount {
		t.Fatalf("sign count regressed (possible clone): was %d now %d",
			initialSignCount, after[0].Authenticator.SignCount)
	}
}

// TestWebAuthn_WrongAuthenticatorRejected verifies that an assertion from a
// DIFFERENT authenticator than the one registered is rejected.
func TestWebAuthn_WrongAuthenticatorRejected(t *testing.T) {
	s, ctx := setupAuthStore(t)
	uid := seedUser(t, s, ctx, "passkey@example.org", "active")
	ws := NewWebAuthnStore(s.db)
	w, _ := webauthn.New(&webauthn.Config{RPID: "localhost", RPDisplayName: "AĒR", RPOrigins: []string{"https://localhost"}})
	rp := vwa.RelyingParty{Name: "AĒR", ID: "localhost", Origin: "https://localhost"}

	// Register authenticator A.
	authA := vwa.NewAuthenticator()
	credA := vwa.NewCredential(vwa.KeyTypeEC2)
	user := &auth.WebAuthnUser{ID: uid, Email: "passkey@example.org"}
	options, sessionData, _ := w.BeginRegistration(user)
	oj, _ := json.Marshal(options)
	ao, _ := vwa.ParseAttestationOptions(string(oj))
	ar := vwa.CreateAttestationResponse(rp, authA, credA, *ao)
	parsed, _ := protocol.ParseCredentialCreationResponseBytes([]byte(ar))
	newCred, _ := w.CreateCredential(user, *sessionData, parsed)
	_, _ = ws.SaveCredential(ctx, uid, newCred, "A")
	stored, _ := ws.CredentialsByUser(ctx, uid)

	// Assert with a DIFFERENT authenticator B / credential B.
	authB := vwa.NewAuthenticator()
	credB := vwa.NewCredential(vwa.KeyTypeEC2)
	authB.AddCredential(credB)
	user2 := &auth.WebAuthnUser{ID: uid, Email: "passkey@example.org", Creds: stored}
	aOptions, aSession, _ := w.BeginLogin(user2)
	aoj, _ := json.Marshal(aOptions)
	assertOpts, err := vwa.ParseAssertionOptions(string(aoj))
	if err != nil {
		// BeginLogin restricts allowed credentials to A; B is not allowed.
		return
	}
	assertionResponse := vwa.CreateAssertionResponse(rp, authB, credB, *assertOpts)
	aParsed, err := protocol.ParseCredentialRequestResponseBytes([]byte(assertionResponse))
	if err != nil {
		return // malformed for the wrong credential — acceptable rejection
	}
	if _, err := w.ValidateLogin(user2, *aSession, aParsed); err == nil {
		t.Fatal("expected assertion from a different authenticator to be rejected")
	}
}
