package auth

import (
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAndParseToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-with-sufficient-entropy")

	raw, expiresAt, err := IssueToken(42)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := parseToken(raw)
	if err != nil {
		t.Fatal(err)
	}
	if userID != 42 {
		t.Fatalf("user ID = %d, want 42", userID)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("token expiration %s is not in the future", expiresAt)
	}
}

func TestParseTokenRejectsInvalidClaims(t *testing.T) {
	const secret = "test-secret-with-sufficient-entropy"
	t.Setenv("JWT_SECRET", secret)

	tests := []struct {
		name   string
		claims jwt.RegisteredClaims
	}{
		{
			name: "expired",
			claims: jwt.RegisteredClaims{
				Issuer:    tokenIssuer,
				Subject:   "42",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			},
		},
		{
			name: "missing expiration",
			claims: jwt.RegisteredClaims{
				Issuer:  tokenIssuer,
				Subject: "42",
			},
		},
		{
			name: "invalid subject",
			claims: jwt.RegisteredClaims{
				Issuer:    tokenIssuer,
				Subject:   "user-42",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
		},
		{
			name: "wrong issuer",
			claims: jwt.RegisteredClaims{
				Issuer:    "another-service",
				Subject:   "42",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, test.claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseToken(raw); err == nil {
				t.Fatal("expected invalid claims to be rejected")
			}
		})
	}
}

func TestParseTokenRequiresStrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	if _, err := parseToken("not-a-token"); err == nil {
		t.Fatal("expected short JWT secret to be rejected")
	}
}

func TestSubjectUsesNumericUserID(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-with-sufficient-entropy")
	raw, _, err := IssueToken(7)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := parseToken(raw)
	if err != nil || strconv.FormatInt(userID, 10) != "7" {
		t.Fatalf("parsed user ID = %d, error = %v", userID, err)
	}
}
