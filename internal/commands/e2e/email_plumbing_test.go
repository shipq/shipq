//go:build e2e

package e2e

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"testing"
	"time"
)

func skipIfSMTPCredsAbsent(t *testing.T) {
	t.Helper()
	for _, key := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_TO"} {
		if os.Getenv(key) == "" {
			t.Skipf("skipping: %s not set", key)
		}
	}
}

func TestPlumbing_SMTP_SendEmail(t *testing.T) {
	skipIfSMTPCredsAbsent(t)

	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")
	to := os.Getenv("SMTP_TO")

	addr := net.JoinHostPort(host, port)

	// Connect
	client, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("smtp.Dial(%s): %v", addr, err)
	}
	defer client.Close()

	// STARTTLS if port 587
	if port == "587" {
		tlsCfg := &tls.Config{ServerName: host}
		if err := client.StartTLS(tlsCfg); err != nil {
			t.Fatalf("STARTTLS: %v", err)
		}
	}

	// Auth (skip if username is empty — local dev servers like Mailpit)
	if username != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			t.Fatalf("AUTH: %v", err)
		}
	}

	// Compose
	now := time.Now().UTC().Format(time.RFC3339)
	subject := "shipq plumbing test"
	body := fmt.Sprintf("<h1>Plumbing Test</h1><p>Sent at %s</p><p>Token: abc123secret</p>", now)

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="UTF-8"`,
		"",
		body,
	}, "\r\n")

	// Send
	if err := client.Mail(from); err != nil {
		t.Fatalf("MAIL FROM: %v", err)
	}
	if err := client.Rcpt(to); err != nil {
		t.Fatalf("RCPT TO: %v", err)
	}
	w, err := client.Data()
	if err != nil {
		t.Fatalf("DATA: %v", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close data: %v", err)
	}

	if err := client.Quit(); err != nil {
		t.Fatalf("QUIT: %v", err)
	}

	t.Logf("Email sent successfully to %s via %s at %s", to, addr, now)
}
