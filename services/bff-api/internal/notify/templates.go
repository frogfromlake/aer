package notify

// Bilingual (EN + DE) auth-mail templates (Phase 153). The deployment serves
// DE+EN researchers and no per-recipient locale is stored (privacy-minimal —
// only email + hash + role + consent, Phase 55), so every message carries both
// languages in one body rather than guessing a locale. Plain text only: no
// HTML, no tracking pixels. Link expiry is phrased softly ("a limited time")
// to stay decoupled from the configurable TTLs.

const mailSignature = "— AĒR"

// langSeparator divides the English block from the German block.
const langSeparator = "\n\n———\n\n"

// inviteMessage renders the accept-invite mail for the given activation link.
func inviteMessage(link string) (subject, body string) {
	subject = "AĒR — Your invitation · Ihre Einladung"
	body = "Hello,\n\n" +
		"You have been invited to AĒR. Open the link below to set your password " +
		"and activate your account:\n\n" +
		link + "\n\n" +
		"The link is valid for a limited time. If you did not expect this " +
		"invitation, you can ignore this email.\n\n" +
		mailSignature +
		langSeparator +
		"Hallo,\n\n" +
		"Sie wurden zu AĒR eingeladen. Öffnen Sie den Link unten, um Ihr Passwort " +
		"zu setzen und Ihr Konto zu aktivieren:\n\n" +
		link + "\n\n" +
		"Der Link ist nur für begrenzte Zeit gültig. Falls Sie diese Einladung " +
		"nicht erwartet haben, können Sie diese E-Mail ignorieren.\n\n" +
		mailSignature + "\n"
	return subject, body
}

// passwordResetMessage renders the password-reset mail for the given link.
func passwordResetMessage(link string) (subject, body string) {
	subject = "AĒR — Password reset · Passwort zurücksetzen"
	body = "Hello,\n\n" +
		"We received a request to reset your AĒR password. Open the link below " +
		"to choose a new password:\n\n" +
		link + "\n\n" +
		"The link is valid for a limited time. If you did not request this, you " +
		"can ignore this email — your password stays unchanged.\n\n" +
		mailSignature +
		langSeparator +
		"Hallo,\n\n" +
		"Wir haben eine Anfrage erhalten, Ihr AĒR-Passwort zurückzusetzen. Öffnen " +
		"Sie den Link unten, um ein neues Passwort zu wählen:\n\n" +
		link + "\n\n" +
		"Der Link ist nur für begrenzte Zeit gültig. Falls Sie das nicht " +
		"angefordert haben, können Sie diese E-Mail ignorieren — Ihr Passwort " +
		"bleibt unverändert.\n\n" +
		mailSignature + "\n"
	return subject, body
}
