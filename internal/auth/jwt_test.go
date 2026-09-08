package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateJWT_ReturnsToken(t *testing.T) {
	token, err := GenerateJWT("testuser", []byte("secret123"))
	if err != nil {
		t.Fatalf("GenerateJWT returned error: %v", err)
	}
	if token == "" {
		t.Error("GenerateJWT should return non-empty token")
	}
}

func TestGenerateJWT_Structure(t *testing.T) {
	token, _ := GenerateJWT("testuser", []byte("secret123"))
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("JWT should have 3 parts, got %d", len(parts))
	}
}

func TestVerifyJWT_Valid(t *testing.T) {
	secret := []byte("test-secret")
	token, err := GenerateJWT("alice", secret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	claims, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.Username != "alice" {
		t.Errorf("Expected username 'alice', got %q", claims.Username)
	}
}

func TestVerifyJWT_ClaimsHaveExpiration(t *testing.T) {
	secret := []byte("test-secret")
	token, _ := GenerateJWT("user", secret)
	claims, _ := VerifyJWT(token, secret)

	if claims.ExpiresAt == nil {
		t.Fatal("Claims should have expiration time")
	}
	if claims.ExpiresAt.Time.Before(time.Now()) {
		t.Error("Token should not be expired at creation time")
	}
}

func TestVerifyJWT_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret")

	// Create expired token manually
	expiredClaims := &Claims{
		Username: "expired",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenString, _ := token.SignedString(secret)

	_, err := VerifyJWT(tokenString, secret)
	if err == nil {
		t.Error("VerifyJWT should return error for expired token")
	}
}

func TestVerifyJWT_TamperedToken(t *testing.T) {
	secret := []byte("test-secret")
	token, _ := GenerateJWT("user", secret)

	tampered := token[:len(token)-5] + "XXXXX"
	_, err := VerifyJWT(tampered, secret)
	if err == nil {
		t.Error("VerifyJWT should return error for tampered token")
	}
}

func TestVerifyJWT_WrongSecret(t *testing.T) {
	token, _ := GenerateJWT("user", []byte("correct-secret"))
	_, err := VerifyJWT(token, []byte("wrong-secret"))
	if err == nil {
		t.Error("VerifyJWT should return error with wrong secret")
	}
}

func TestVerifyJWT_InvalidFormat(t *testing.T) {
	_, err := VerifyJWT("not-a-jwt-token", []byte("secret"))
	if err == nil {
		t.Error("VerifyJWT should return error for invalid format")
	}
}

func TestVerifyJWT_EmptyString(t *testing.T) {
	_, err := VerifyJWT("", []byte("secret"))
	if err == nil {
		t.Error("VerifyJWT should return error for empty string")
	}
}

func TestVerifyJWT_DifferentUsers(t *testing.T) {
	secret := []byte("test-secret")
	token1, _ := GenerateJWT("alice", secret)
	token2, _ := GenerateJWT("bob", secret)

	claims1, _ := VerifyJWT(token1, secret)
	claims2, _ := VerifyJWT(token2, secret)

	if claims1.Username != "alice" {
		t.Errorf("Expected 'alice', got %q", claims1.Username)
	}
	if claims2.Username != "bob" {
		t.Errorf("Expected 'bob', got %q", claims2.Username)
	}
}

func TestVerifyJWT_LongUsername(t *testing.T) {
	secret := []byte("test-secret")
	longName := strings.Repeat("a", 256)
	token, _ := GenerateJWT(longName, secret)
	claims, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.Username != longName {
		t.Errorf("Username should be preserved for long names")
	}
}

func TestVerifyJWT_SpecialCharactersUsername(t *testing.T) {
	secret := []byte("test-secret")
	token, _ := GenerateJWT("user@domain.com", secret)
	claims, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.Username != "user@domain.com" {
		t.Errorf("Expected 'user@domain.com', got %q", claims.Username)
	}
}
