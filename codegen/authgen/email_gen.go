package authgen

import (
	"bytes"
	"fmt"
)

// GenerateEmailHandler generates api/auth/email.go which provides:
// - SendEmailParams struct
// - SendEmail function (SMTP sending + token redaction + DB logging)
// - sendViaSMTP function (raw net/smtp sending)
// - HashToken function (SHA-256 for high-entropy tokens)
// - generateSecureToken function (crypto/rand token generation)
func GenerateEmailHandler(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"crypto/rand\"\n")
	buf.WriteString("\t\"crypto/sha256\"\n")
	buf.WriteString("\t\"crypto/tls\"\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"encoding/hex\"\n")
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"net\"\n")
	buf.WriteString("\t\"net/smtp\"\n")
	buf.WriteString("\t\"os\"\n")
	buf.WriteString("\t\"strings\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/config"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/nanoid"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(")\n\n")

	// SendEmailParams struct
	buf.WriteString(`// SendEmailParams holds the parameters for sending an email.
type SendEmailParams struct {
	From            string
	To              string
	Subject         string
	HTMLBody        string
	SensitiveTokens []string // tokens to redact before persisting
}

`)

	// SendEmail function
	buf.WriteString(`// SendEmail sends an HTML email via SMTP and logs it to the sent_emails table.
// Sensitive tokens are redacted in the persisted copy but included in the
// actual email delivered to the recipient.
func SendEmail(ctx context.Context, runner queries.Runner, params SendEmailParams) error {
	// 1. Send via SMTP (real tokens in the email body)
	err := sendViaSMTP(params.From, params.To, params.Subject, params.HTMLBody)

	status := "sent"
	var errorMessage string
	if err != nil {
		status = "failed"
		errorMessage = err.Error()
	}

	// 2. Redact sensitive tokens for storage
	redactedBody := params.HTMLBody
	for _, token := range params.SensitiveTokens {
		redactedBody = strings.ReplaceAll(redactedBody, token, "*****")
	}

	// 3. Persist to sent_emails
	_, insertErr := runner.InsertSentEmail(ctx, queries.InsertSentEmailParams{
		PublicId:      nanoid.New(),
		ToEmail:       params.To,
		Subject:       params.Subject,
		HtmlBody:      redactedBody,
		Status:        status,
		ErrorMessage:  &errorMessage,
	})
	if insertErr != nil {
		// Log but don't fail — the email may have been sent successfully.
		// The persistence failure is operational, not user-facing.
		fmt.Fprintf(os.Stderr, "WARNING: email sent but failed to persist: %v\n", insertErr)
	}

	return err
}

`)

	// sendViaSMTP function
	buf.WriteString(`// sendViaSMTP sends an email using net/smtp.
func sendViaSMTP(from, to, subject, htmlBody string) error {
	host := config.Settings.SMTP_HOST
	port := config.Settings.SMTP_PORT
	username := config.Settings.SMTP_USERNAME
	password := config.Settings.SMTP_PASSWORD

	addr := net.JoinHostPort(host, port)

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer client.Close()

	// STARTTLS for port 587
	if port == "587" {
		tlsCfg := &tls.Config{ServerName: host}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	// Auth (skip for dev servers with empty credentials)
	if username != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
		"\r\n" +
		htmlBody

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	return client.Quit()
}

`)

	// HashToken function
	buf.WriteString(`// HashToken hashes a high-entropy token using SHA-256.
// Unlike passwords, reset/verification tokens are random and don't need bcrypt.
// SHA-256 is the industry-standard approach for high-entropy tokens
// (used by Rails, Django, Laravel).
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

`)

	// generateSecureToken function
	buf.WriteString(`// generateSecureToken generates a cryptographically secure random token
// of the given byte length, returned as a hex string.
func generateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
`)

	return formatSource(buf.Bytes())
}
