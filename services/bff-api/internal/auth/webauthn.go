package auth

import (
	"github.com/go-webauthn/webauthn/webauthn"
)

// NewWebAuthn builds the WebAuthn relying-party (ADR-040). RPID is the
// registrable domain (e.g. "localhost" or "aer.example"); RPOrigins are the
// full origins the browser will present (e.g. "https://localhost"). Returns nil
// only on misconfiguration.
func NewWebAuthn(rpID, rpDisplayName string, rpOrigins []string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     rpOrigins,
	})
}

// WebAuthnUser adapts an AĒR user and its registered credentials to the
// go-webauthn `webauthn.User` interface. The handler builds it from the store;
// this keeps the auth package free of a storage dependency.
type WebAuthnUser struct {
	ID    string
	Email string
	Creds []webauthn.Credential
}

// WebAuthnID is the stable user handle. The account UUID (36 ASCII bytes) is
// well under the 64-byte limit and never changes.
func (u *WebAuthnUser) WebAuthnID() []byte                         { return []byte(u.ID) }
func (u *WebAuthnUser) WebAuthnName() string                       { return u.Email }
func (u *WebAuthnUser) WebAuthnDisplayName() string                { return u.Email }
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.Creds }
