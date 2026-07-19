package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"sbs-bsp-cctv/internal/mailer"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrDisallowedDomain   = errors.New("email domain is not allowed")
	ErrEmailInUse         = errors.New("email is already in use")
	ErrRefreshTokenReused = errors.New("refresh token invalid or already used")
	ErrResetTokenInvalid  = errors.New("reset token invalid, expired, or already used")
)

const (
	RefreshTokenTTL  = 7 * 24 * time.Hour
	PasswordResetTTL = 1 * time.Hour
)

type Service struct {
	pool          *pgxpool.Pool
	issuer        *TokenIssuer
	allowedDomain string
	mailCfg       mailer.Config
}

func NewService(pool *pgxpool.Pool, issuer *TokenIssuer, allowedDomain string, mailCfg mailer.Config) *Service {
	return &Service{pool: pool, issuer: issuer, allowedDomain: strings.ToLower(allowedDomain), mailCfg: mailCfg}
}

func (s *Service) ValidateEmailDomain(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	suffix := "@" + s.allowedDomain
	if !strings.HasSuffix(email, suffix) {
		return ErrDisallowedDomain
	}
	return nil
}

// NormalizeEmail lets callers enter just the part before "@" and have the
// allowed domain appended automatically; addresses that already include a
// domain are passed through untouched.
func (s *Service) NormalizeEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" && !strings.Contains(email, "@") {
		email += "@" + s.allowedDomain
	}
	return email
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	UserID       uuid.UUID
	Email        string
	Roles        []string
}

// CreateUser is admin-only in the API layer (enforced via RBAC middleware);
// this method only enforces the domain restriction and password hashing.
func (s *Service) CreateUser(ctx context.Context, email, fullName, password string, roleNames []string) (uuid.UUID, error) {
	email = s.NormalizeEmail(email)
	if err := s.ValidateEmailDomain(email); err != nil {
		return uuid.Nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name)
		VALUES (lower($1), $2, $3)
		RETURNING id`, email, string(hash), fullName,
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return uuid.Nil, ErrEmailInUse
		}
		return uuid.Nil, err
	}

	for _, roleName := range roleNames {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE name = $2`, userID, roleName)
		if err != nil {
			return uuid.Nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

type UserSummary struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	IsActive bool      `json:"is_active"`
	Roles    []string  `json:"roles"`
}

// ListUsers backs the Admin > Users page.
func (s *Service) ListUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, full_name, is_active FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []UserSummary{}
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.IsActive); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range users {
		roles, err := s.rolesForUser(ctx, users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].Roles = roles
	}
	return users, nil
}

// SetUserRoles replaces a user's role assignments wholesale — simpler and
// less error-prone for an admin UI than diffing add/remove sets.
func (s *Service) SetUserRoles(ctx context.Context, userID uuid.UUID, roleNames []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, roleName := range roleNames {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE name = $2`, userID, roleName); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// UpdateUser edits a user's profile fields. Email and password are changed
// through their own dedicated flows, not this generic edit.
func (s *Service) UpdateUser(ctx context.Context, userID uuid.UUID, fullName string, isActive bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET full_name = $2, is_active = $3, updated_at = now() WHERE id = $1`,
		userID, fullName, isActive)
	return err
}

// AdminSetPassword sets a user's password directly, bypassing the reset-token
// flow. Only callable by admins (enforced at the route level). Revokes the
// user's existing refresh tokens so any active sessions are forced to
// re-authenticate with the new password.
func (s *Service) AdminSetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`,
		userID, string(passwordHash)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	email = s.NormalizeEmail(email)
	if err := s.ValidateEmailDomain(email); err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	var (
		userID       uuid.UUID
		passwordHash string
		isActive     bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, password_hash, is_active FROM users WHERE email = lower($1)`, email,
	).Scan(&userID, &passwordHash, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return AuthResult{}, err
	}
	if !isActive {
		return AuthResult{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	roles, err := s.rolesForUser(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}

	access, err := s.issuer.IssueAccessToken(userID, email, roles)
	if err != nil {
		return AuthResult{}, err
	}

	refresh, err := s.issueRefreshToken(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		AccessToken:  access,
		RefreshToken: refresh,
		UserID:       userID,
		Email:        email,
		Roles:        roles,
	}, nil
}

// Refresh rotates the given refresh token: the old one is revoked and a new
// access/refresh pair is issued. Reuse of an already-revoked or unknown
// token is rejected outright.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (AuthResult, error) {
	hash := hashToken(refreshToken)

	var (
		userID    uuid.UUID
		expiresAt time.Time
		revokedAt *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&userID, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthResult{}, ErrRefreshTokenReused
	}
	if err != nil {
		return AuthResult{}, err
	}
	if revokedAt != nil || time.Now().After(expiresAt) {
		return AuthResult{}, ErrRefreshTokenReused
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1`, hash); err != nil {
		return AuthResult{}, err
	}

	var email string
	if err := s.pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
		return AuthResult{}, err
	}

	roles, err := s.rolesForUser(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}

	access, err := s.issuer.IssueAccessToken(userID, email, roles)
	if err != nil {
		return AuthResult{}, err
	}
	newRefresh, err := s.issueRefreshToken(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{AccessToken: access, RefreshToken: newRefresh, UserID: userID, Email: email, Roles: roles}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	hash := hashToken(refreshToken)
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	return err
}

func (s *Service) rolesForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.name FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}

func (s *Service) issueRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	raw := generateRandomToken()
	hash := hashToken(raw)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, hash, time.Now().Add(RefreshTokenTTL))
	if err != nil {
		return "", err
	}
	return raw, nil
}

// RequestPasswordReset issues a fresh reset token for the given email and
// emails a reset link to it, if an active account with that email exists.
// It never reveals whether the account exists — callers should always
// report success to the caller regardless of the returned error, which is
// only for logging/observability.
func (s *Service) RequestPasswordReset(ctx context.Context, email, baseURL string) error {
	email = s.NormalizeEmail(email)

	var (
		userID   uuid.UUID
		isActive bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, is_active FROM users WHERE email = lower($1)`, email,
	).Scan(&userID, &isActive)
	if errors.Is(err, pgx.ErrNoRows) || !isActive {
		return nil
	}
	if err != nil {
		return err
	}

	raw := generateRandomToken()
	hash := hashToken(raw)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, hash, time.Now().Add(PasswordResetTTL))
	if err != nil {
		return err
	}

	linkURL := baseURL + "/reset-password/" + raw
	sendErr := mailer.SendHTML(s.mailCfg, email,
		"Reset your SBS AssetWatch password",
		passwordResetEmailHTML(linkURL),
		nil,
	)
	if sendErr != nil && !errors.Is(sendErr, mailer.ErrNotConfigured) {
		return sendErr
	}
	return nil
}

// ResetPassword consumes a reset token issued by RequestPasswordReset,
// setting the account's password and revoking all of its refresh tokens so
// any sessions issued under the old password are invalidated.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	hash := hashToken(token)

	var (
		userID    uuid.UUID
		expiresAt time.Time
		usedAt    *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, expires_at, used_at FROM password_reset_tokens WHERE token_hash = $1`, hash,
	).Scan(&userID, &expiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrResetTokenInvalid
	}
	if err != nil {
		return err
	}
	if usedAt != nil || time.Now().After(expiresAt) {
		return ErrResetTokenInvalid
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`,
		userID, string(passwordHash)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = now() WHERE token_hash = $1`, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func passwordResetEmailHTML(linkURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:Arial,Helvetica,sans-serif;">
	<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f5f7;padding:32px 0;">
		<tr>
			<td align="center">
				<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
					<tr>
						<td style="background-color:#1f2937;padding:20px 32px;">
							<span style="color:#ffffff;font-size:18px;font-weight:bold;">SBS AssetWatch</span>
						</td>
					</tr>
					<tr>
						<td style="padding:32px;color:#1f2937;">
							<h1 style="font-size:18px;margin:0 0 16px;">Reset your password</h1>
							<p style="font-size:14px;line-height:1.6;margin:0 0 16px;">
								We received a request to reset the password for your account. Click the button below to choose a new one.
								If you didn't request this, you can safely ignore this email.
							</p>
							<p style="text-align:center;margin:24px 0;">
								<a href="%s" style="background-color:#2563eb;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:6px;font-size:14px;font-weight:bold;display:inline-block;">
									Reset Password
								</a>
							</p>
							<p style="font-size:12px;line-height:1.6;color:#6b7280;margin:0 0 8px;">
								If the button above doesn't work, copy and paste this link into your browser:
							</p>
							<p style="font-size:12px;word-break:break-all;color:#2563eb;margin:0 0 16px;">%s</p>
							<p style="font-size:12px;color:#6b7280;margin:0;">This link expires in 1 hour.</p>
						</td>
					</tr>
				</table>
			</td>
		</tr>
	</table>
</body>
</html>`, linkURL, linkURL)
}

func generateRandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
