// Package notify is the pluggable delivery seam for auth links (ADR-040).
// Phase 134 ships only the LogSender (manual delivery: the link is logged and
// surfaced in the admin UI). An SMTP sender becomes a config switch later,
// without touching the handlers — the same channel will carry future user
// communication.
package notify

import (
	"context"
	"log/slog"
)

// LinkSender delivers one-time auth links (invite, password reset).
type LinkSender interface {
	SendInvite(ctx context.Context, email, link string) error
	SendPasswordReset(ctx context.Context, email, link string) error
}

// LogSender is the manual/POC delivery: it logs the link at WARN so an
// operator can copy it. No SMTP dependency. NEVER use as the sole sender once
// external users self-serve password resets — wire a real sender then.
type LogSender struct{}

func (LogSender) SendInvite(_ context.Context, email, link string) error {
	slog.Warn("auth: invite link (manual sender — no SMTP configured)", "email", email, "link", link)
	return nil
}

func (LogSender) SendPasswordReset(_ context.Context, email, link string) error {
	slog.Warn("auth: password-reset link (manual sender — no SMTP configured)", "email", email, "link", link)
	return nil
}
