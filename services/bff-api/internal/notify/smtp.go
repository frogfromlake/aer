// SMTP delivery for auth links (Phase 153 / ADR-043).
//
// AĒR is provider-agnostic: any hosted transactional relay that speaks SMTP
// submission with STARTTLS works (the documented default is Brevo, EU/DSGVO).
// The provider is pure configuration — swapping it touches no code. This file
// depends only on the standard library: no third-party SMTP client is pulled
// in (Occam's Razor; the phase's "no fragile dependency" constraint).
package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// dialTimeout bounds the TCP connect + handshake so a hung relay cannot wedge
// an invite/reset request.
const dialTimeout = 15 * time.Second

// sendTimeout bounds the WHOLE SMTP conversation (STARTTLS/AUTH/MAIL/RCPT/DATA/
// QUIT) via conn.SetDeadline, not just the dial — a relay that accepts the
// connection then tarpits a later phase cannot otherwise hang the goroutine
// indefinitely (SEC-022).
const sendTimeout = 30 * time.Second

// SMTPConfig carries the credentials for a hosted transactional-email relay.
type SMTPConfig struct {
	Host     string // relay host, e.g. smtp-relay.brevo.com
	Port     string // submission port, e.g. 587 (STARTTLS)
	Username string // relay SMTP login
	Password string // relay SMTP key/password
	From     string // header + envelope From address
	FromName string // display name, e.g. "AĒR"
}

// SMTPSender delivers auth links through an SMTP submission relay using
// STARTTLS. It satisfies notify.LinkSender.
type SMTPSender struct {
	cfg SMTPConfig
	// send is the transport seam, injected so tests exercise message framing
	// without a live relay. Defaults to sendStartTLS.
	send func(cfg SMTPConfig, to string, msg []byte) error
	// now stamps the Date header; injectable for deterministic tests.
	now func() time.Time
}

// NewSMTPSender builds a sender bound to the real STARTTLS transport.
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg, send: sendStartTLS, now: time.Now}
}

// SendInvite delivers the accept-invite link.
func (s *SMTPSender) SendInvite(_ context.Context, email, link string) error {
	subject, body := inviteMessage(link)
	return s.dispatch(email, subject, body)
}

// SendPasswordReset delivers the password-reset link.
func (s *SMTPSender) SendPasswordReset(_ context.Context, email, link string) error {
	subject, body := passwordResetMessage(link)
	return s.dispatch(email, subject, body)
}

func (s *SMTPSender) dispatch(email, subject, body string) error {
	// Header-injection guard: a recipient address must be a single line. The
	// address originates from admin input / the auth DB, but defence in depth
	// is cheap here.
	if strings.ContainsAny(email, "\r\n") {
		return fmt.Errorf("notify: invalid recipient address")
	}
	msg := buildMessage(s.cfg, email, subject, body, s.now())
	if err := s.send(s.cfg, email, msg); err != nil {
		return fmt.Errorf("notify: smtp send: %w", err)
	}
	return nil
}

// buildMessage frames an RFC 5322 text/plain message. Non-ASCII headers
// (the AĒR brand, German umlauts in the subject) are RFC 2047 encoded; the
// UTF-8 body is sent 8-bit. No HTML, no tracking pixels (anti-surveillance
// posture — ADR-040 / Phase 55).
func buildMessage(cfg SMTPConfig, to, subject, body string, now time.Time) []byte {
	from := cfg.From
	if cfg.FromName != "" {
		from = mime.QEncoding.Encode("utf-8", cfg.FromName) + " <" + cfg.From + ">"
	}
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + now.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	// Auto-Submitted suppresses vacation auto-replies from the recipient side.
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("\r\n")
	b.WriteString(toCRLF(body))
	return []byte(b.String())
}

// toCRLF normalises body line endings to CRLF, the SMTP wire convention.
func toCRLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}

// sendStartTLS opens an SMTP submission connection, upgrades it with STARTTLS,
// authenticates with PLAIN, and writes one message. It mirrors smtp.SendMail
// but is explicit about requiring STARTTLS before AUTH so credentials never
// cross a plaintext link (TLS 1.2 floor).
func sendStartTLS(cfg SMTPConfig, to string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return err
	}
	// Bound every subsequent phase, not just the dial (SEC-022): a relay that
	// stalls on STARTTLS/AUTH/DATA now fails by deadline instead of hanging.
	if err := conn.SetDeadline(time.Now().Add(sendTimeout)); err != nil {
		_ = conn.Close()
		return err
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	if ok, _ := c.Extension("STARTTLS"); !ok {
		return fmt.Errorf("notify: relay %s does not offer STARTTLS", cfg.Host)
	}
	if err := c.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return err
	}
	if err := c.Mail(cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
