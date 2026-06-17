package handler

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/frogfromlake/aer/services/bff-api/internal/auth"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// mockWebAuthnBackend is an in-memory WebAuthnBackend for the passkey handlers.
type mockWebAuthnBackend struct {
	creds           []webauthn.Credential
	credsErr        error
	hasCreds        bool
	hasCredsErr     error
	metas           []storage.CredentialMeta
	metasErr        error
	deleted         bool
	deleteErr       error
	saveCeremonyErr error
	consumeSession  *webauthn.SessionData
	consumeErr      error
	saveCredMeta    storage.CredentialMeta
	saveCredErr     error
	updateErr       error
}

func (m *mockWebAuthnBackend) CredentialsByUser(context.Context, string) ([]webauthn.Credential, error) {
	return m.creds, m.credsErr
}
func (m *mockWebAuthnBackend) HasCredentials(context.Context, string) (bool, error) {
	return m.hasCreds, m.hasCredsErr
}
func (m *mockWebAuthnBackend) SaveCredential(context.Context, string, *webauthn.Credential, string) (storage.CredentialMeta, error) {
	return m.saveCredMeta, m.saveCredErr
}
func (m *mockWebAuthnBackend) UpdateCredential(context.Context, *webauthn.Credential) error {
	return m.updateErr
}
func (m *mockWebAuthnBackend) ListCredentialMeta(context.Context, string) ([]storage.CredentialMeta, error) {
	return m.metas, m.metasErr
}
func (m *mockWebAuthnBackend) DeleteCredential(context.Context, string, string) (bool, error) {
	return m.deleted, m.deleteErr
}
func (m *mockWebAuthnBackend) SaveCeremony(context.Context, string, string, *webauthn.SessionData, time.Time) error {
	return m.saveCeremonyErr
}
func (m *mockWebAuthnBackend) ConsumeCeremony(context.Context, string, string) (*webauthn.SessionData, error) {
	return m.consumeSession, m.consumeErr
}

func webAuthnServer(t *testing.T, be *mockWebAuthnBackend) *Server {
	t.Helper()
	wa, err := auth.NewWebAuthn("localhost", "AĒR test", []string{"https://localhost"})
	if err != nil {
		t.Fatalf("NewWebAuthn: %v", err)
	}
	return &Server{webAuthn: wa, webAuthnBackend: be, authConfig: AuthConfig{CookieName: "aer_session"}}
}

func webAuthnCtx() context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{UserID: "u1", Email: "alice@example.org", Role: auth.RoleResearcher})
}

// --- RegisterBegin ---

func TestWebAuthnRegisterBegin_Success(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{})
	resp, err := s.PostAuthWebauthnRegisterBegin(webAuthnCtx(), PostAuthWebauthnRegisterBeginRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(PostAuthWebauthnRegisterBegin200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if _, present := got["publicKey"]; !present {
		t.Errorf("registration options must carry a publicKey block, got %v", got)
	}
}

func TestWebAuthnRegisterBegin_LoadUserError_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{credsErr: errTest})
	resp, _ := s.PostAuthWebauthnRegisterBegin(webAuthnCtx(), PostAuthWebauthnRegisterBeginRequestObject{})
	if _, ok := resp.(PostAuthWebauthnRegisterBegin500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestWebAuthnRegisterBegin_SaveCeremonyError_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{saveCeremonyErr: errTest})
	resp, _ := s.PostAuthWebauthnRegisterBegin(webAuthnCtx(), PostAuthWebauthnRegisterBeginRequestObject{})
	if _, ok := resp.(PostAuthWebauthnRegisterBegin500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- RegisterFinish ---

func TestWebAuthnRegisterFinish_NilBody_400(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{})
	resp, _ := s.PostAuthWebauthnRegisterFinish(webAuthnCtx(), PostAuthWebauthnRegisterFinishRequestObject{Body: nil})
	if _, ok := resp.(PostAuthWebauthnRegisterFinish400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400", resp)
	}
}

func TestWebAuthnRegisterFinish_NoCeremony_400(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{consumeSession: nil})
	body := PostAuthWebauthnRegisterFinishJSONRequestBody{"id": "x"}
	resp, _ := s.PostAuthWebauthnRegisterFinish(webAuthnCtx(), PostAuthWebauthnRegisterFinishRequestObject{Body: &body})
	if _, ok := resp.(PostAuthWebauthnRegisterFinish400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400 for an absent ceremony", resp)
	}
}

func TestWebAuthnRegisterFinish_ConsumeError_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{consumeErr: errTest})
	body := PostAuthWebauthnRegisterFinishJSONRequestBody{"id": "x"}
	resp, _ := s.PostAuthWebauthnRegisterFinish(webAuthnCtx(), PostAuthWebauthnRegisterFinishRequestObject{Body: &body})
	if _, ok := resp.(PostAuthWebauthnRegisterFinish500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestWebAuthnRegisterFinish_BadAttestation_400(t *testing.T) {
	// A live ceremony but a body that cannot parse as an attestation.
	s := webAuthnServer(t, &mockWebAuthnBackend{consumeSession: &webauthn.SessionData{}})
	body := PostAuthWebauthnRegisterFinishJSONRequestBody{"not": "a credential"}
	resp, _ := s.PostAuthWebauthnRegisterFinish(webAuthnCtx(), PostAuthWebauthnRegisterFinishRequestObject{Body: &body})
	if _, ok := resp.(PostAuthWebauthnRegisterFinish400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400 for an unparseable attestation", resp)
	}
}

// --- ListCredentials ---

func TestWebAuthnListCredentials_MapsNullableFields(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{metas: []storage.CredentialMeta{
		{ID: "c1", Name: sql.NullString{String: "laptop", Valid: true}, CreatedAt: timeAt("2025-01-01T00:00:00Z"),
			LastUsedAt: sql.NullTime{Time: timeAt("2025-01-02T00:00:00Z"), Valid: true}},
		{ID: "c2"}, // null name + null lastUsedAt → nil pointers
	}})
	resp, err := s.GetAuthWebauthnCredentials(webAuthnCtx(), GetAuthWebauthnCredentialsRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetAuthWebauthnCredentials200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
	if len(got.Credentials) != 2 {
		t.Fatalf("credentials = %d, want 2", len(got.Credentials))
	}
	if got.Credentials[0].Name == nil || *got.Credentials[0].Name != "laptop" {
		t.Errorf("c1 name = %v, want laptop", got.Credentials[0].Name)
	}
	if got.Credentials[1].Name != nil || got.Credentials[1].LastUsedAt != nil {
		t.Errorf("c2 nullable fields should be nil, got name=%v lastUsed=%v", got.Credentials[1].Name, got.Credentials[1].LastUsedAt)
	}
}

func TestWebAuthnListCredentials_Error_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{metasErr: errTest})
	resp, _ := s.GetAuthWebauthnCredentials(webAuthnCtx(), GetAuthWebauthnCredentialsRequestObject{})
	if _, ok := resp.(GetAuthWebauthnCredentials500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- DeleteCredential ---

func TestWebAuthnDeleteCredential_Deleted_204(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{deleted: true})
	resp, _ := s.DeleteAuthWebauthnCredential(webAuthnCtx(), DeleteAuthWebauthnCredentialRequestObject{ID: "c1"})
	if _, ok := resp.(DeleteAuthWebauthnCredential204Response); !ok {
		t.Fatalf("response = %T, want 204", resp)
	}
}

func TestWebAuthnDeleteCredential_NotFound_404(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{deleted: false})
	resp, _ := s.DeleteAuthWebauthnCredential(webAuthnCtx(), DeleteAuthWebauthnCredentialRequestObject{ID: "ghost"})
	if _, ok := resp.(DeleteAuthWebauthnCredential404JSONResponse); !ok {
		t.Fatalf("response = %T, want 404", resp)
	}
}

func TestWebAuthnDeleteCredential_Error_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{deleteErr: errTest})
	resp, _ := s.DeleteAuthWebauthnCredential(webAuthnCtx(), DeleteAuthWebauthnCredentialRequestObject{ID: "c1"})
	if _, ok := resp.(DeleteAuthWebauthnCredential500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

// --- AssertBegin ---

func TestWebAuthnAssertBegin_NoPasskey_400(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{hasCreds: false})
	resp, _ := s.PostAuthWebauthnAssertBegin(webAuthnCtx(), PostAuthWebauthnAssertBeginRequestObject{})
	if _, ok := resp.(PostAuthWebauthnAssertBegin400JSONResponse); !ok {
		t.Fatalf("response = %T, want 400 when no passkey is registered", resp)
	}
}

func TestWebAuthnAssertBegin_HasCredentialsError_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{hasCredsErr: errTest})
	resp, _ := s.PostAuthWebauthnAssertBegin(webAuthnCtx(), PostAuthWebauthnAssertBeginRequestObject{})
	if _, ok := resp.(PostAuthWebauthnAssertBegin500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestWebAuthnAssertBegin_Success(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{
		hasCreds: true,
		creds:    []webauthn.Credential{{ID: []byte("cred-1")}},
	})
	resp, err := s.PostAuthWebauthnAssertBegin(webAuthnCtx(), PostAuthWebauthnAssertBeginRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(PostAuthWebauthnAssertBegin200JSONResponse); !ok {
		t.Fatalf("response = %T, want 200", resp)
	}
}

// --- AssertFinish ---

func TestWebAuthnAssertFinish_NilBody_401(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{})
	resp, _ := s.PostAuthWebauthnAssertFinish(webAuthnCtx(), PostAuthWebauthnAssertFinishRequestObject{Body: nil})
	if _, ok := resp.(PostAuthWebauthnAssertFinish401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401", resp)
	}
}

func TestWebAuthnAssertFinish_NoCeremony_401(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{consumeSession: nil})
	body := PostAuthWebauthnAssertFinishJSONRequestBody{"id": "x"}
	resp, _ := s.PostAuthWebauthnAssertFinish(webAuthnCtx(), PostAuthWebauthnAssertFinishRequestObject{Body: &body})
	if _, ok := resp.(PostAuthWebauthnAssertFinish401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401 for an absent ceremony", resp)
	}
}

func TestWebAuthnAssertFinish_ConsumeError_500(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{consumeErr: errTest})
	body := PostAuthWebauthnAssertFinishJSONRequestBody{"id": "x"}
	resp, _ := s.PostAuthWebauthnAssertFinish(webAuthnCtx(), PostAuthWebauthnAssertFinishRequestObject{Body: &body})
	if _, ok := resp.(PostAuthWebauthnAssertFinish500JSONResponse); !ok {
		t.Fatalf("response = %T, want 500", resp)
	}
}

func TestWebAuthnAssertFinish_BadAssertion_401(t *testing.T) {
	s := webAuthnServer(t, &mockWebAuthnBackend{consumeSession: &webauthn.SessionData{}})
	body := PostAuthWebauthnAssertFinishJSONRequestBody{"not": "an assertion"}
	resp, _ := s.PostAuthWebauthnAssertFinish(webAuthnCtx(), PostAuthWebauthnAssertFinishRequestObject{Body: &body})
	if _, ok := resp.(PostAuthWebauthnAssertFinish401JSONResponse); !ok {
		t.Fatalf("response = %T, want 401 for an unparseable assertion", resp)
	}
}
