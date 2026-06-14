package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-webauthn/webauthn/protocol"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// webAuthnCeremonyTTL bounds how long a registration / assertion challenge is
// valid between begin and finish.
const webAuthnCeremonyTTL = 5 * time.Minute

// sessionUser returns the authenticated identity, or nil when the caller is a
// machine (X-API-Key) or has no session.
func sessionUser(ctx context.Context) *auth.Identity {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Machine || id.UserID == "" {
		return nil
	}
	return id
}

// webAuthnUserFor loads the user's registered credentials and builds the
// go-webauthn user adapter.
func (s *Server) webAuthnUserFor(ctx context.Context, id *auth.Identity) (*auth.WebAuthnUser, error) {
	creds, err := s.webAuthnBackend.CredentialsByUser(ctx, id.UserID)
	if err != nil {
		return nil, err
	}
	return &auth.WebAuthnUser{ID: id.UserID, Email: id.Email, Creds: creds}, nil
}

// toJSONMap re-encodes any value as a generic JSON object so it can be returned
// through an OpenAPI freeform-object response without losing fidelity.
func toJSONMap(v interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// PostAuthWebauthnRegisterBegin starts a passkey registration ceremony.
func (s *Server) PostAuthWebauthnRegisterBegin(ctx context.Context, _ PostAuthWebauthnRegisterBeginRequestObject) (PostAuthWebauthnRegisterBeginResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PostAuthWebauthnRegisterBegin401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	user, err := s.webAuthnUserFor(ctx, id)
	if err != nil {
		slog.Error("webauthn register begin: load user", "error", err)
		return PostAuthWebauthnRegisterBegin500JSONResponse{Message: genericInternalError}, nil
	}
	options, sessionData, err := s.webAuthn.BeginRegistration(user)
	if err != nil {
		slog.Error("webauthn begin registration", "error", err)
		return PostAuthWebauthnRegisterBegin500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.webAuthnBackend.SaveCeremony(ctx, id.UserID, "register", sessionData, time.Now().Add(webAuthnCeremonyTTL)); err != nil {
		slog.Error("webauthn save ceremony", "error", err)
		return PostAuthWebauthnRegisterBegin500JSONResponse{Message: genericInternalError}, nil
	}
	m, err := toJSONMap(options)
	if err != nil {
		slog.Error("webauthn marshal options", "error", err)
		return PostAuthWebauthnRegisterBegin500JSONResponse{Message: genericInternalError}, nil
	}
	return PostAuthWebauthnRegisterBegin200JSONResponse(m), nil
}

// PostAuthWebauthnRegisterFinish verifies the attestation and persists the passkey.
func (s *Server) PostAuthWebauthnRegisterFinish(ctx context.Context, request PostAuthWebauthnRegisterFinishRequestObject) (PostAuthWebauthnRegisterFinishResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PostAuthWebauthnRegisterFinish401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if request.Body == nil {
		return PostAuthWebauthnRegisterFinish400JSONResponse{Code: "invalid_request", Message: "missing request body"}, nil
	}
	sessionData, err := s.webAuthnBackend.ConsumeCeremony(ctx, id.UserID, "register")
	if err != nil {
		slog.Error("webauthn register finish: consume ceremony", "error", err)
		return PostAuthWebauthnRegisterFinish500JSONResponse{Message: genericInternalError}, nil
	}
	if sessionData == nil {
		return PostAuthWebauthnRegisterFinish400JSONResponse{Code: "invalid_ceremony", Message: "no active or unexpired registration ceremony"}, nil
	}
	parsed, err := parseCreationResponse(*request.Body)
	if err != nil {
		return PostAuthWebauthnRegisterFinish400JSONResponse{Code: "invalid_attestation", Message: "could not parse the attestation"}, nil
	}
	user, err := s.webAuthnUserFor(ctx, id)
	if err != nil {
		slog.Error("webauthn register finish: load user", "error", err)
		return PostAuthWebauthnRegisterFinish500JSONResponse{Message: genericInternalError}, nil
	}
	cred, err := s.webAuthn.CreateCredential(user, *sessionData, parsed)
	if err != nil {
		return PostAuthWebauthnRegisterFinish400JSONResponse{Code: "invalid_attestation", Message: "attestation verification failed"}, nil
	}
	meta, err := s.webAuthnBackend.SaveCredential(ctx, id.UserID, cred, "")
	if err != nil {
		slog.Error("webauthn save credential", "error", err)
		return PostAuthWebauthnRegisterFinish500JSONResponse{Message: genericInternalError}, nil
	}
	return PostAuthWebauthnRegisterFinish201JSONResponse{
		ID:         meta.ID,
		CreatedAt:  meta.CreatedAt,
		Name:       nullStrPtr(meta.Name),
		LastUsedAt: nullTimePtr(meta.LastUsedAt),
	}, nil
}

// GetAuthWebauthnCredentials lists the user's passkeys.
func (s *Server) GetAuthWebauthnCredentials(ctx context.Context, _ GetAuthWebauthnCredentialsRequestObject) (GetAuthWebauthnCredentialsResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return GetAuthWebauthnCredentials401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	metas, err := s.webAuthnBackend.ListCredentialMeta(ctx, id.UserID)
	if err != nil {
		slog.Error("webauthn list credentials", "error", err)
		return GetAuthWebauthnCredentials500JSONResponse{Message: genericInternalError}, nil
	}
	var out GetAuthWebauthnCredentials200JSONResponse
	for _, m := range metas {
		out.Credentials = append(out.Credentials, struct {
			CreatedAt  time.Time  `json:"createdAt"`
			ID         string     `json:"id"`
			LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
			Name       *string    `json:"name,omitempty"`
		}{
			CreatedAt:  m.CreatedAt,
			ID:         m.ID,
			LastUsedAt: nullTimePtr(m.LastUsedAt),
			Name:       nullStrPtr(m.Name),
		})
	}
	return out, nil
}

// DeleteAuthWebauthnCredential removes one of the user's passkeys.
func (s *Server) DeleteAuthWebauthnCredential(ctx context.Context, request DeleteAuthWebauthnCredentialRequestObject) (DeleteAuthWebauthnCredentialResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return DeleteAuthWebauthnCredential401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	deleted, err := s.webAuthnBackend.DeleteCredential(ctx, id.UserID, request.ID)
	if err != nil {
		slog.Error("webauthn delete credential", "error", err)
		return DeleteAuthWebauthnCredential500JSONResponse{Message: genericInternalError}, nil
	}
	if !deleted {
		return DeleteAuthWebauthnCredential404JSONResponse{Code: "not_found", Message: "no such passkey"}, nil
	}
	return DeleteAuthWebauthnCredential204Response{}, nil
}

// PostAuthWebauthnAssertBegin starts an assertion (step-up) ceremony.
func (s *Server) PostAuthWebauthnAssertBegin(ctx context.Context, _ PostAuthWebauthnAssertBeginRequestObject) (PostAuthWebauthnAssertBeginResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PostAuthWebauthnAssertBegin401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	has, err := s.webAuthnBackend.HasCredentials(ctx, id.UserID)
	if err != nil {
		slog.Error("webauthn assert begin: has credentials", "error", err)
		return PostAuthWebauthnAssertBegin500JSONResponse{Message: genericInternalError}, nil
	}
	if !has {
		return PostAuthWebauthnAssertBegin400JSONResponse{Code: "no_passkey", Message: "no passkey registered"}, nil
	}
	user, err := s.webAuthnUserFor(ctx, id)
	if err != nil {
		slog.Error("webauthn assert begin: load user", "error", err)
		return PostAuthWebauthnAssertBegin500JSONResponse{Message: genericInternalError}, nil
	}
	options, sessionData, err := s.webAuthn.BeginLogin(user)
	if err != nil {
		slog.Error("webauthn begin login", "error", err)
		return PostAuthWebauthnAssertBegin500JSONResponse{Message: genericInternalError}, nil
	}
	if err := s.webAuthnBackend.SaveCeremony(ctx, id.UserID, "login", sessionData, time.Now().Add(webAuthnCeremonyTTL)); err != nil {
		slog.Error("webauthn save ceremony", "error", err)
		return PostAuthWebauthnAssertBegin500JSONResponse{Message: genericInternalError}, nil
	}
	m, err := toJSONMap(options)
	if err != nil {
		slog.Error("webauthn marshal options", "error", err)
		return PostAuthWebauthnAssertBegin500JSONResponse{Message: genericInternalError}, nil
	}
	return PostAuthWebauthnAssertBegin200JSONResponse(m), nil
}

// PostAuthWebauthnAssertFinish verifies an assertion (step-up).
func (s *Server) PostAuthWebauthnAssertFinish(ctx context.Context, request PostAuthWebauthnAssertFinishRequestObject) (PostAuthWebauthnAssertFinishResponseObject, error) {
	id := sessionUser(ctx)
	if id == nil {
		return PostAuthWebauthnAssertFinish401JSONResponse{Code: "unauthenticated", Message: "no active session"}, nil
	}
	if request.Body == nil {
		return PostAuthWebauthnAssertFinish401JSONResponse{Code: "assertion_failed", Message: "assertion failed"}, nil
	}
	sessionData, err := s.webAuthnBackend.ConsumeCeremony(ctx, id.UserID, "login")
	if err != nil {
		slog.Error("webauthn assert finish: consume ceremony", "error", err)
		return PostAuthWebauthnAssertFinish500JSONResponse{Message: genericInternalError}, nil
	}
	if sessionData == nil {
		return PostAuthWebauthnAssertFinish401JSONResponse{Code: "assertion_failed", Message: "assertion failed"}, nil
	}
	parsed, err := parseAssertionResponse(*request.Body)
	if err != nil {
		return PostAuthWebauthnAssertFinish401JSONResponse{Code: "assertion_failed", Message: "assertion failed"}, nil
	}
	user, err := s.webAuthnUserFor(ctx, id)
	if err != nil {
		slog.Error("webauthn assert finish: load user", "error", err)
		return PostAuthWebauthnAssertFinish500JSONResponse{Message: genericInternalError}, nil
	}
	cred, err := s.webAuthn.ValidateLogin(user, *sessionData, parsed)
	if err != nil {
		return PostAuthWebauthnAssertFinish401JSONResponse{Code: "assertion_failed", Message: "assertion failed"}, nil
	}
	if err := s.webAuthnBackend.UpdateCredential(ctx, cred); err != nil {
		slog.Error("webauthn update credential", "error", err)
		return PostAuthWebauthnAssertFinish500JSONResponse{Message: genericInternalError}, nil
	}
	return PostAuthWebauthnAssertFinish200JSONResponse{Verified: true}, nil
}

// --- parse helpers (freeform body map → go-webauthn parsed response) ---------

func parseCreationResponse(body map[string]interface{}) (*protocol.ParsedCredentialCreationData, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return protocol.ParseCredentialCreationResponseBytes(b)
}

func parseAssertionResponse(body map[string]interface{}) (*protocol.ParsedCredentialAssertionData, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return protocol.ParseCredentialRequestResponseBytes(b)
}

// compile-time assertion that *storage.WebAuthnStore satisfies WebAuthnBackend.
var _ WebAuthnBackend = (*storage.WebAuthnStore)(nil)

func nullStrPtr(n sql.NullString) *string {
	if n.Valid {
		return &n.String
	}
	return nil
}

func nullTimePtr(n sql.NullTime) *time.Time {
	if n.Valid {
		return &n.Time
	}
	return nil
}
