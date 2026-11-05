package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/Harsh7213/shorty/auth"
	"github.com/Harsh7213/shorty/db"
	"github.com/Harsh7213/shorty/notifications"
	"golang.org/x/crypto/bcrypt"
)

const verificationTokenLifetime = 24 * time.Hour
const bcryptCost = 12

type userCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func RegisterUser(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}

	email, ok := normalizeEmail(request.Email)
	if !ok {
		http.Error(w, "A valid email address is required", http.StatusBadRequest)
		return
	}
	if len(request.Password) < 12 || len(request.Password) > 72 {
		http.Error(w, "Password must be between 12 and 72 bytes", http.StatusBadRequest)
		return
	}
	if len(request.DisplayName) > 100 {
		http.Error(w, "display_name must be at most 100 characters", http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcryptCost)
	if err != nil {
		http.Error(w, "Failed to create account", http.StatusInternalServerError)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to create account", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO shorty.users (email, display_name, password_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING
		RETURNING id`, email, strings.TrimSpace(request.DisplayName), string(passwordHash),
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "An account with this email already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "Failed to create account", http.StatusInternalServerError)
		return
	}

	token, err := createVerificationToken(r, tx, userID)
	if err != nil {
		http.Error(w, "Failed to create verification token", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to create account", http.StatusInternalServerError)
		return
	}

	if err := notifications.SendVerificationEmail(email, token); err != nil {
		log.Printf("verification email delivery failed for new account: %v", err)
		http.Error(w, "Account created, but verification email could not be sent. Retry using the resend endpoint.", http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "Account created. Check your email to verify it before signing in.",
	})
}

func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Verification token is required", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT t.user_id
		FROM shorty.email_verification_tokens t
		JOIN shorty.users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND t.expires_at > NOW()
		FOR UPDATE OF t, u`, hashVerificationToken(token)).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Verification token is invalid or expired", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE shorty.users
		SET email_verified_at = NOW(), password_setup_required = FALSE, updated_at = NOW()
		WHERE id = $1 AND deactivated_at IS NULL`, userID)
	if err != nil {
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected != 1 {
		http.Error(w, "Account is deactivated", http.StatusConflict)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM shorty.email_verification_tokens WHERE user_id = $1`, userID); err != nil {
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Email verified. You can now sign in."})
}

func ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	email, ok := normalizeEmail(request.Email)
	if !ok {
		http.Error(w, "A valid email address is required", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var userID int64
	var passwordSetupRequired bool
	err = tx.QueryRowContext(r.Context(), `
		SELECT id, password_setup_required FROM shorty.users
		WHERE email = $1 AND email_verified_at IS NULL AND deactivated_at IS NULL`,
		email,
	).Scan(&userID, &passwordSetupRequired)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "If the account needs verification, an email will be sent."})
		return
	}
	if err != nil {
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}

	token, err := createVerificationToken(r, tx, userID)
	if err != nil {
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}
	var sendVerificationErr error
	if passwordSetupRequired {
		sendVerificationErr = notifications.SendInvitationEmail(email, token)
	} else {
		sendVerificationErr = notifications.SendVerificationEmail(email, token)
	}
	if sendVerificationErr != nil {
		log.Printf("verification email delivery failed: %v", sendVerificationErr)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "If the account needs verification, an email will be sent."})
}

func LoginUser(w http.ResponseWriter, r *http.Request) {
	var request userCredentials
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	email, ok := normalizeEmail(request.Email)
	if !ok || request.Password == "" || len(request.Password) > 72 {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	var userID int64
	var passwordHash string
	var verifiedAt sql.NullTime
	var deactivatedAt sql.NullTime
	err := db.DB.QueryRowContext(r.Context(), `
		SELECT id, password_hash, email_verified_at, deactivated_at
		FROM shorty.users WHERE email = $1`, email,
	).Scan(&userID, &passwordHash, &verifiedAt, &deactivatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, hashErr := bcrypt.GenerateFromPassword([]byte(request.Password), bcryptCost); hashErr != nil {
				http.Error(w, "Failed to verify credentials", http.StatusInternalServerError)
				return
			}
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)); err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}
	if deactivatedAt.Valid {
		http.Error(w, "Account is deactivated", http.StatusForbidden)
		return
	}
	if !verifiedAt.Valid {
		http.Error(w, "Verify your email before signing in", http.StatusForbidden)
		return
	}

	token, expiresAt, err := auth.IssueToken(userID)
	if err != nil {
		http.Error(w, "Authentication is not configured", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   expiresAt,
	})
}

func CurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var user struct {
		ID          int64  `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	err := db.DB.QueryRowContext(r.Context(), `
		SELECT id, email, display_name FROM shorty.users
		WHERE id = $1 AND deactivated_at IS NULL`, userID,
	).Scan(&user.ID, &user.Email, &user.DisplayName)
	if err != nil {
		http.Error(w, "Failed to load account", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
		WITH RECURSIVE users_to_delete(id) AS (
			SELECT id FROM shorty.users WHERE id = $1
			UNION
			SELECT m.user_id
			FROM users_to_delete deleting_owner
			JOIN shorty.organizations o ON o.owner_user_id = deleting_owner.id
			JOIN shorty.organization_members m ON m.organization_id = o.id
		)
		DELETE FROM shorty.users u
		USING users_to_delete d
		WHERE u.id = d.id`,
		userID,
	)
	if err != nil {
		http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to verify account deletion", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	if request.CurrentPassword == "" || len(request.CurrentPassword) > 72 {
		http.Error(w, "Current password is required", http.StatusBadRequest)
		return
	}
	if len(request.NewPassword) < 12 || len(request.NewPassword) > 72 {
		http.Error(w, "New password must be between 12 and 72 bytes", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to change password", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var passwordHash string
	err = tx.QueryRowContext(r.Context(), `
		SELECT password_hash FROM shorty.users
		WHERE id = $1 AND deactivated_at IS NULL
		FOR UPDATE`, userID,
	).Scan(&passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to change password", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.CurrentPassword)); err != nil {
		http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcryptCost)
	if err != nil {
		http.Error(w, "Failed to change password", http.StatusInternalServerError)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE shorty.users
		SET password_hash = $1, password_setup_required = FALSE, updated_at = NOW()
		WHERE id = $2 AND deactivated_at IS NULL`,
		string(newPasswordHash), userID,
	); err != nil {
		http.Error(w, "Failed to change password", http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to change password", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully."})
}

func createVerificationToken(r *http.Request, tx *sql.Tx, userID int64) (string, error) {
	randomToken := make([]byte, 32)
	if _, err := rand.Read(randomToken); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(randomToken)
	_, err := tx.ExecContext(r.Context(), `
		INSERT INTO shorty.email_verification_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET token_hash = EXCLUDED.token_hash,
		    expires_at = EXCLUDED.expires_at,
		    created_at = NOW()`,
		userID, hashVerificationToken(token), time.Now().Add(verificationTokenLifetime),
	)
	return token, err
}

func hashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(value string) (string, bool) {
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 {
		return "", false
	}
	return strings.ToLower(value), true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		if err == nil {
			return errors.New("request body contains multiple JSON values")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}
