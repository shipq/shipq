package authgen

import (
	"bytes"
	"fmt"
)

// GenerateVerifyEmailHandler generates api/auth/verify_email.go which provides:
// - VerifyEmailRequest struct
// - VerifyEmail handler (POST /auth/verify-email)
//
// The handler accepts a raw token (from the verification email link), hashes it
// with SHA-256, looks it up in email_verification_tokens, and marks the account
// as verified.
func GenerateVerifyEmailHandler(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/httperror"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(")\n\n")

	// Request struct
	buf.WriteString(`// VerifyEmailRequest is the request body for POST /auth/verify-email.
type VerifyEmailRequest struct {
	Token string ` + "`json:\"token\"`" + `
}

`)

	// Handler function
	buf.WriteString(`// VerifyEmail handles POST /auth/verify-email.
// It validates the token and marks the account as verified.
func VerifyEmail(ctx context.Context, req *VerifyEmailRequest) (*struct{}, error) {
	runner := queries.RunnerFromContext(ctx)

	if req.Token == "" {
		return nil, httperror.BadRequest("token is required")
	}

	// Hash the submitted token with SHA-256 for lookup
	tokenHash := HashToken(req.Token)

	// Look up the token (must be unused and not expired)
	token, err := runner.FindEmailVerificationToken(ctx, queries.FindEmailVerificationTokenParams{
		TokenHash: tokenHash,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}
	if token == nil {
		return nil, httperror.BadRequest("invalid or expired verification token")
	}

	// Mark account as verified
	err = runner.VerifyAccount(ctx, queries.VerifyAccountParams{
		PublicId: token.AccountPublicId,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "failed to verify account", err)
	}

	// Mark token as used
	err = runner.MarkEmailVerificationTokenUsed(ctx, queries.MarkEmailVerificationTokenUsedParams{
		PublicId: token.PublicId,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}

	return &struct{}{}, nil
}
`)

	return formatSource(buf.Bytes())
}

// GenerateResendVerificationHandler generates api/auth/resend_verification.go which provides:
// - ResendVerificationRequest struct
// - ResendVerification handler (POST /auth/resend-verification)
//
// The handler is timing-safe: it always returns 200 regardless of whether the
// email exists or the account is already verified, to prevent enumeration.
func GenerateResendVerificationHandler(cfg AuthGenConfig) ([]byte, error) {
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
	buf.WriteString("// fromEmailResendVerification is the sender address for verification emails.\n")
	buf.WriteString("// TODO: Update this to your application's sender address.\n")
	buf.WriteString("var fromEmailResendVerification = \"noreply@\" + config.Settings.SMTP_HOST\n\n")

	// Request struct
	buf.WriteString(`// ResendVerificationRequest is the request body for POST /auth/resend-verification.
type ResendVerificationRequest struct {
	Email string ` + "`json:\"email\"`" + `
}

`)

	// Handler function
	buf.WriteString(`// ResendVerification handles POST /auth/resend-verification.
// Always returns 200 regardless of whether the email exists or is already verified (timing-safe).
// This prevents email enumeration attacks.
func ResendVerification(ctx context.Context, req *ResendVerificationRequest) (*struct{}, error) {
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

	// If the account is already verified, silently succeed
	if account.Verified {
		return &struct{}{}, nil
	}

	// Generate a new verification token
	rawToken, err := generateSecureToken(32)
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}
	tokenHash := HashToken(rawToken)

	// Store hashed token with 24-hour expiry
	_, err = runner.InsertEmailVerificationToken(ctx, queries.InsertEmailVerificationTokenParams{
		PublicId:   nanoid.New(),
		AccountId:  account.Id,
		TokenHash:  tokenHash,
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}

	// Build verification link
	verifyURL := config.Settings.APP_URL + "/verify-email?token=" + rawToken

	// Send email
	// NOTE: Rate limiting should be added here to prevent abuse.
	htmlBody := fmt.Sprintf(` + "`" + `<h2>Verify Your Email</h2>
<p>Click the link below to verify your email address. This link expires in 24 hours.</p>
<p><a href="%s">Verify Email</a></p>
<p>If you didn't create an account, you can safely ignore this email.</p>` + "`" + `,
		verifyURL,
	)

	sendParams := SendEmailParams{
		From:            fromEmailResendVerification,
		To:              req.Email,
		Subject:         "Verify Your Email",
		HTMLBody:        htmlBody,
		SensitiveTokens: []string{rawToken},
	}

	if err := SendEmail(ctx, runner, sendParams); err != nil {
		// Log the error but don't expose it to the user
		fmt.Fprintf(os.Stderr, "WARNING: failed to send verification email: %v\n", err)
	}

	return &struct{}{}, nil
}
`)

	return formatSource(buf.Bytes())
}
