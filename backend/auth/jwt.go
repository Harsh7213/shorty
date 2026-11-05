package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Harsh7213/shorty/db"
	"github.com/golang-jwt/jwt/v5"
)

const tokenLifetime = 15 * time.Minute
const tokenIssuer = "shorty-api"

type contextKey struct{}

type claims struct {
	jwt.RegisteredClaims
}

func UserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(contextKey{}).(int64)
	return userID, ok
}

func IssueToken(userID int64) (string, time.Time, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return "", time.Time{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}

	expiresAt := time.Now().Add(tokenLifetime)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}
	return signed, expiresAt, nil
}

func RequireJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(os.Getenv("JWT_SECRET")) < 32 {
			http.Error(w, "Authentication is not configured", http.StatusInternalServerError)
			return
		}

		header := strings.Fields(r.Header.Get("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			http.Error(w, "Bearer token is required", http.StatusUnauthorized)
			return
		}

		userID, err := parseToken(header[1])
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		var active bool
		err = db.DB.QueryRowContext(r.Context(), `
			SELECT email_verified_at IS NOT NULL AND deactivated_at IS NULL
			FROM shorty.users
			WHERE id = $1`, userID).Scan(&active)
		if err == sql.ErrNoRows || (err == nil && !active) {
			http.Error(w, "Account is not active or email is not verified", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Failed to verify account", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), contextKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func parseToken(raw string) (int64, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return 0, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}

	token, err := jwt.ParseWithClaims(raw, &claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(), jwt.WithIssuer(tokenIssuer))
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid or expired token")
	}
	tokenClaims, ok := token.Claims.(*claims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}
	userID, err := strconv.ParseInt(tokenClaims.Subject, 10, 64)
	if err != nil || userID < 1 {
		return 0, fmt.Errorf("token subject must be a positive numeric user ID")
	}
	return userID, nil
}
