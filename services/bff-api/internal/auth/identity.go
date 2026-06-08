package auth

import "context"

// Role is the RBAC role of a user (ADR-040).
type Role string

const (
	RoleAdmin      Role = "admin"
	RoleResearcher Role = "researcher"
)

// Identity is the authenticated principal attached to a request context by the
// SessionOrAPIKey middleware. It is either a human user (authenticated by the
// session cookie) or a machine (authenticated by the X-API-Key — CI / internal
// callers). Handlers that need a real user must check Machine / UserID.
type Identity struct {
	UserID string
	Email  string
	Role   Role
	// SessionIDHash is the sha256 of the presenting session id, so a handler
	// (e.g. logout, change-password) can revoke the current or sibling
	// sessions. Empty for machine callers.
	SessionIDHash string
	// Machine is true for X-API-Key callers, which carry no user identity.
	Machine bool
}

type identityCtxKey struct{}

// WithIdentity returns a copy of ctx carrying the identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, id)
}

// IdentityFromContext returns the request identity, if any.
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(*Identity)
	return id, ok && id != nil
}
