package authgen

import (
	"bytes"
	"fmt"
)

// GenerateForgotPasswordHandler generates api/auth/forgot_password.go which provides:
// - ForgotPasswordRequest struct
// - ForgotPassword handler (POST /auth/forgot-password)
//
// The handler is timing-safe: it always returns 200 regardless of whether the
// email exists, to prevent email enumeration attacks. It generates a secure
// random token, stores a SHA-256 hash in the password_reset_tokens table,
// invalidates any prior tokens for the account, and dispatches an email
// containing the raw token in a reset link.
func GenerateForgotPasswordHandler(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"os\"\n")
	buf.WriteString("\t\"time\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/config"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/httperror"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/nanoid"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(")\n\n")

	// fromEmail constant
	buf.WriteString("// fromEmailForgotPassword is the sender address for password reset emails.\n")
	buf.WriteString("// TODO: Update this to your application's sender address.\n")
	buf.WriteString("var fromEmailForgotPassword = \"noreply@\" + config.Settings.SMTP_HOST\n\n")

	// Request struct
	buf.WriteString(`// ForgotPasswordRequest is the request body for POST /auth/forgot-password.
type ForgotPasswordRequest struct {
	Email string ` + "`json:\"email\"`" + `
}

`)

	// Handler function
	buf.WriteString(`// ForgotPassword handles POST /auth/forgot-password.
// Always returns 200 regardless of whether the email exists (timing-safe).
// This prevents email enumeration attacks.
func ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) (*struct{}, error) {
	runner := queries.RunnerFromContext(ctx)

	// Look up account (don't reveal if it exists)
	account, err := runner.FindAccountByEmail(ctx, queries.FindAccountByEmailParams{
		Email: req.Email,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}
	if account == nil {
		// Return success anyway to prevent email enumeration
		return &struct{}{}, nil
	}

	// Generate a crypto-random token
	rawToken, err := generateSecureToken(32)
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}
	tokenHash := HashToken(rawToken)

	// Invalidate any existing tokens for this account
	_ = runner.InvalidatePasswordResetTokens(ctx, queries.InvalidatePasswordResetTokensParams{
		AccountId: account.Id,
	})

	// Store hashed token with 1-hour expiry
	_, err = runner.InsertPasswordResetToken(ctx, queries.InsertPasswordResetTokenParams{
		PublicId:   nanoid.New(),
		AccountId:  account.Id,
		TokenHash:  tokenHash,
		ExpiresAt:  time.Now().UTC().Add(1 * time.Hour).Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}

	// Build reset link
	resetURL := config.Settings.APP_URL + "/reset-password?token=" + rawToken

	// Send email
	// NOTE: Rate limiting should be added here to prevent abuse.
	htmlBody := fmt.Sprintf(` + "`" + `<h2>Reset Your Password</h2>
<p>Click the link below to reset your password. This link expires in 1 hour.</p>
<p><a href="%s">Reset Password</a></p>
<p>If you didn't request this, you can safely ignore this email.</p>` + "`" + `,
		resetURL,
	)

	sendParams := SendEmailParams{
		From:            fromEmailForgotPassword,
		To:              req.Email,
		Subject:         "Reset Your Password",
		HTMLBody:        htmlBody,
		SensitiveTokens: []string{rawToken},
	}

	if err := SendEmail(ctx, runner, sendParams); err != nil {
		// Log the error but don't expose it to the user —
		// the token is stored and the email can be retried.
		fmt.Fprintf(os.Stderr, "WARNING: failed to send password reset email: %v\n", err)
	}

	return &struct{}{}, nil
}
`)

	return formatSource(buf.Bytes())
}
