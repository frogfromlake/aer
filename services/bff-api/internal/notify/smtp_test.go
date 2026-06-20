package notify

import (
	"context"
	"strings"
	"testing"
	"time"
)

// newCapturingSender returns an SMTPSender whose transport records the framed
// message instead of dialing a relay, with a fixed Date for determinism.
func newCapturingSender(cfg SMTPConfig, captured *[]byte, toOut *string) *SMTPSender {
	s := NewSMTPSender(cfg)
	s.now = func() time.Time { return time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC) }
	s.send = func(_ SMTPConfig, to string, msg []byte) error {
		*toOut = to
		*captured = msg
		return nil
	}
	return s
}

var testCfg = SMTPConfig{
	Host:     "smtp.example.org",
	Port:     "587",
	Username: "relay-user",
	Password: "relay-key",
	From:     "noreply@aer.example",
	FromName: "AĒR",
}

func TestSMTPSender_SendInvite_FramesBilingualMessage(t *testing.T) {
	var msg []byte
	var to string
	s := newCapturingSender(testCfg, &msg, &to)

	link := "https://aer.example/accept-invite?token=abc123"
	if err := s.SendInvite(context.Background(), "invitee@example.org", link); err != nil {
		t.Fatalf("SendInvite returned error: %v", err)
	}
	got := string(msg)

	if to != "invitee@example.org" {
		t.Errorf("envelope recipient = %q, want invitee@example.org", to)
	}
	for _, want := range []string{
		"To: invitee@example.org\r\n",
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n",
		"Auto-Submitted: auto-generated\r\n",
		"<noreply@aer.example>", // From header carries the address
		link,                    // the activation link is present
		"Hello,",                // English block
		"Hallo,",                // German block
		"Date: ",                // a Date header was stamped
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message missing %q\n--- message ---\n%s", want, got)
		}
	}
	// The link must appear once per language block (EN + DE).
	if n := strings.Count(got, link); n != 2 {
		t.Errorf("link appears %d times, want 2 (one per language block)", n)
	}
	// CRLF line endings on the wire.
	if strings.Contains(got, "\n") && !strings.Contains(got, "\r\n") {
		t.Error("message body is not CRLF-normalised")
	}
}

func TestSMTPSender_SendPasswordReset_HasResetSubject(t *testing.T) {
	var msg []byte
	var to string
	s := newCapturingSender(testCfg, &msg, &to)

	if err := s.SendPasswordReset(context.Background(), "user@example.org", "https://aer.example/reset-password?token=xyz"); err != nil {
		t.Fatalf("SendPasswordReset returned error: %v", err)
	}
	got := string(msg)
	// Subject is RFC 2047 encoded (contains non-ASCII AĒR + umlauts), so assert
	// on the encoded-word marker rather than the raw words.
	if !strings.Contains(got, "Subject: =?") {
		t.Errorf("expected an RFC 2047 encoded Subject header\n%s", got)
	}
	if !strings.Contains(got, "your password stays unchanged") {
		t.Errorf("reset message missing English reassurance copy\n%s", got)
	}
	if !strings.Contains(got, "Ihr Passwort\r\n") && !strings.Contains(got, "bleibt unverändert") {
		t.Errorf("reset message missing German reassurance copy\n%s", got)
	}
}

func TestSMTPSender_RejectsHeaderInjection(t *testing.T) {
	var msg []byte
	var to string
	s := newCapturingSender(testCfg, &msg, &to)

	err := s.SendInvite(context.Background(), "victim@example.org\r\nBcc: evil@example.org", "https://aer.example/x")
	if err == nil {
		t.Fatal("expected an error for a recipient containing CRLF")
	}
	if msg != nil {
		t.Error("transport must not be called for an invalid recipient")
	}
}

func TestBuildMessage_OmitsFromNameWhenEmpty(t *testing.T) {
	cfg := testCfg
	cfg.FromName = ""
	msg := string(buildMessage(cfg, "to@example.org", "Subj", "body", time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)))
	if !strings.Contains(msg, "From: noreply@aer.example\r\n") {
		t.Errorf("expected bare address From header when FromName empty\n%s", msg)
	}
}
