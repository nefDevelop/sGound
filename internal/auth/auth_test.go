package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_ReturnsFormattedHash(t *testing.T) {
	hash, err := HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("Expected hash to start with $argon2id$, got: %s", hash[:20])
	}
	if !strings.Contains(hash, "v=19") {
		t.Errorf("Expected hash to contain version v=19, got: %s", hash)
	}
}

func TestHashPassword_DifferentSalts(t *testing.T) {
	hash1, _ := HashPassword("samepassword")
	hash2, _ := HashPassword("samepassword")
	if hash1 == hash2 {
		t.Error("Two hashes of the same password should differ (random salt)")
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	password := "securepassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	match, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if !match {
		t.Error("VerifyPassword should return true for correct password")
	}
}

func TestVerifyPassword_Wrong(t *testing.T) {
	hash, _ := HashPassword("correctpassword")

	match, err := VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if match {
		t.Error("VerifyPassword should return false for wrong password")
	}
}

func TestVerifyPassword_EmptyPassword(t *testing.T) {
	hash, _ := HashPassword("something")

	match, err := VerifyPassword("", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if match {
		t.Error("VerifyPassword should return false for empty password")
	}
}

func TestVerifyPassword_InvalidHash_TooFewParts(t *testing.T) {
	_, err := VerifyPassword("pass", "$argon2id$only$parts")
	if err != ErrInvalidHash {
		t.Errorf("Expected ErrInvalidHash for malformed hash, got: %v", err)
	}
}

func TestVerifyPassword_InvalidHash_WrongVersion(t *testing.T) {
	// Build a hash with wrong version number
	hash, _ := HashPassword("test")
	// Replace v=19 with v=99
	badHash := strings.Replace(hash, "v=19", "v=99", 1)
	_, err := VerifyPassword("test", badHash)
	if err != ErrIncompatibleVersion {
		t.Errorf("Expected ErrIncompatibleVersion, got: %v", err)
	}
}

func TestVerifyPassword_InvalidHash_BadBase64(t *testing.T) {
	// Valid structure but invalid base64 in salt position
	badHash := "$argon2id$v=19$m=65536,t=3,p=2$!!!invalid!!!$!!!invalid!!!"
	_, err := VerifyPassword("test", badHash)
	if err == nil {
		t.Error("Expected error for invalid base64 hash")
	}
}

func TestVerifyPassword_RoundTrip_MultiplePasswords(t *testing.T) {
	passwords := []string{
		"",
		"a",
		"hello world",
		"p@$$w0rd!#%^&*()",
		" unicode: ñáéíóú ",
		strings.Repeat("long", 100),
	}

	for _, pw := range passwords {
		hash, err := HashPassword(pw)
		if err != nil {
			t.Fatalf("HashPassword failed for %q: %v", pw, err)
		}
		match, err := VerifyPassword(pw, hash)
		if err != nil {
			t.Fatalf("VerifyPassword failed for %q: %v", pw, err)
		}
		if !match {
			t.Errorf("VerifyPassword returned false for correct password %q", pw)
		}
	}
}

func TestGenerateAPIKey_Length(t *testing.T) {
	key := GenerateAPIKey()
	if len(key) != 64 {
		t.Errorf("Expected API key length 64, got %d", len(key))
	}
}

func TestGenerateAPIKey_HexFormat(t *testing.T) {
	key := GenerateAPIKey()
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Expected hex character, got %c", c)
			break
		}
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	key1 := GenerateAPIKey()
	key2 := GenerateAPIKey()
	if key1 == key2 {
		t.Error("Two GenerateAPIKey calls should produce different keys")
	}
}

func TestGenerateAPIKey_NonEmpty(t *testing.T) {
	key := GenerateAPIKey()
	if key == "" {
		t.Error("GenerateAPIKey should not return empty string")
	}
}

func TestDefaultParams_Values(t *testing.T) {
	if DefaultParams.Memory != 16*1024 {
		t.Errorf("Expected Memory=16384, got %d", DefaultParams.Memory)
	}
	if DefaultParams.Iterations != 2 {
		t.Errorf("Expected Iterations=2, got %d", DefaultParams.Iterations)
	}
	if DefaultParams.Parallelism != 2 {
		t.Errorf("Expected Parallelism=2, got %d", DefaultParams.Parallelism)
	}
	if DefaultParams.SaltLength != 16 {
		t.Errorf("Expected SaltLength=16, got %d", DefaultParams.SaltLength)
	}
	if DefaultParams.KeyLength != 32 {
		t.Errorf("Expected KeyLength=32, got %d", DefaultParams.KeyLength)
	}
}
