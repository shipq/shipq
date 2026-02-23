package authgen

import (
	"bytes"
	"fmt"
)

// GenerateResetPasswordHandler generates api/auth/reset_password.go which provides:
// - ResetPasswordRequest struct
// - ResetPassword handler (POST /auth/reset-password)
//
// The handler accepts a raw token (from the email link) and a new password.
// It hashes the token with SHA-256, looks it up in password_reset_tokens,
// verifies it hasn't expired or been used, then updates the account's
// password hash and invalidates all outstanding reset tokens for that account.
func GenerateResetPasswordHandler(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n\n")
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/crypto"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/lib/httperror"))
	buf.WriteString(fmt.Sprintf("\t%q\n", cfg.ModulePath+"/shipq/queries"))
	buf.WriteString(")\n\n")

	// Request struct
	buf.WriteString(`// ResetPasswordRequest is the request body for POST /auth/reset-password.
type ResetPasswordRequest struct {
	Token    string ` + "`json:\"token\"`" + `
	Password string ` + "`json:\"password\"`" + `
}

`)

	// Handler function
	buf.WriteString(`// ResetPassword handles POST /auth/reset-password.
// It validates the token, updates the account's password, and invalidates
// all outstanding reset tokens for the account.
func ResetPassword(ctx context.Context, req *ResetPasswordRequest) (*struct{}, error) {
	runner := queries.RunnerFromContext(ctx)

	if req.Token == "" {
		return nil, httperror.BadRequest("token is required")
	}
	if req.Password == "" {
		return nil, httperror.BadRequest("password is required")
	}

	// Hash the submitted token with SHA-256 for lookup
	tokenHash := HashToken(req.Token)

	// Look up the token (must be unused and not expired)
	token, err := runner.FindPasswordResetToken(ctx, queries.FindPasswordResetTokenParams{
		TokenHash: tokenHash,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}
	if token == nil {
		return nil, httperror.BadRequest("invalid or expired token")
	}

	// Hash the new password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, httperror.Wrap(500, "failed to hash password", err)
	}

	// Update the account's password
	err = runner.UpdateAccountPassword(ctx, queries.UpdateAccountPasswordParams{
		AccountId:    token.AccountId,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "failed to update password", err)
	}

	// Mark this token as used
	err = runner.MarkPasswordResetTokenUsed(ctx, queries.MarkPasswordResetTokenUsedParams{
		PublicId: token.PublicId,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "internal error", err)
	}

	// Invalidate all other tokens for the account
	_ = runner.InvalidatePasswordResetTokens(ctx, queries.InvalidatePasswordResetTokensParams{
		AccountId: token.AccountId,
	})

	return &struct{}{}, nil
}
`)

	return formatSource(buf.Bytes())
}
