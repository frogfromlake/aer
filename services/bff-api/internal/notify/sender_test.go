package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

// captureLogs swaps the default slog logger for a JSON handler writing to buf,
// restoring the original on cleanup. LogSender writes the link at WARN, so the
// test asserts both the no-error contract and that the link actually reaches
// the log (the only delivery channel the POC sender offers — ADR-040).
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func logContains(t *testing.T, buf *bytes.Buffer, wantEmail, wantLink string) {
	t.Helper()
	var saw bool
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("log line not JSON: %v (%s)", err, line)
		}
		if rec["email"] == wantEmail && rec["link"] == wantLink {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected a log line with email=%q link=%q, got: %s", wantEmail, wantLink, buf.String())
	}
}

func TestLogSender_SendInvite_LogsLinkAndSucceeds(t *testing.T) {
	buf := captureLogs(t)
	var s LinkSender = LogSender{}

	if err := s.SendInvite(context.Background(), "invitee@example.org", "https://aer.test/accept?token=abc"); err != nil {
		t.Fatalf("SendInvite returned error: %v", err)
	}
	logContains(t, buf, "invitee@example.org", "https://aer.test/accept?token=abc")
}

func TestLogSender_SendPasswordReset_LogsLinkAndSucceeds(t *testing.T) {
	buf := captureLogs(t)
	var s LinkSender = LogSender{}

	if err := s.SendPasswordReset(context.Background(), "user@example.org", "https://aer.test/reset?token=xyz"); err != nil {
		t.Fatalf("SendPasswordReset returned error: %v", err)
	}
	logContains(t, buf, "user@example.org", "https://aer.test/reset?token=xyz")
}
