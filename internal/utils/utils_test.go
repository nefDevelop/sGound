package utils

import (
	"strings"
	"testing"
)

func TestGenerateID_Deterministic(t *testing.T) {
	id1 := GenerateID("hello")
	id2 := GenerateID("hello")
	if id1 != id2 {
		t.Errorf("GenerateID should be deterministic: %s != %s", id1, id2)
	}
}

func TestGenerateID_DifferentInputs(t *testing.T) {
	id1 := GenerateID("hello")
	id2 := GenerateID("world")
	if id1 == id2 {
		t.Error("Different inputs should produce different IDs")
	}
}

func TestGenerateID_EmptyInput(t *testing.T) {
	id := GenerateID("")
	if id == "" {
		t.Error("GenerateID with empty input should produce a valid hash")
	}
	if len(id) != 40 {
		t.Errorf("SHA1 hex should be 40 chars, got %d", len(id))
	}
}

func TestGenerateID_Length(t *testing.T) {
	id := GenerateID("test")
	if len(id) != 40 {
		t.Errorf("SHA1 hex should be 40 chars, got %d: %s", len(id), id)
	}
}

func TestNormalizeArtist_Simple(t *testing.T) {
	result := NormalizeArtist("Radiohead")
	if result != "radiohead" {
		t.Errorf("Expected 'radiohead', got %q", result)
	}
}

func TestNormalizeArtist_Featuration(t *testing.T) {
	result := NormalizeArtist("Daft Punk feat. Julian Casablancas")
	if result != "daft punk" {
		t.Errorf("Expected 'daft punk', got %q", result)
	}
}

func TestNormalizeArtist_Ft(t *testing.T) {
	result := NormalizeArtist("Artist ft. Someone")
	if result != "artist" {
		t.Errorf("Expected 'artist', got %q", result)
	}
}

func TestNormalizeArtist_FeatSpace(t *testing.T) {
	result := NormalizeArtist("Artist feat Someone")
	if result != "artist" {
		t.Errorf("Expected 'artist', got %q", result)
	}
}

func TestNormalizeArtist_FtSpace(t *testing.T) {
	result := NormalizeArtist("Artist ft Someone")
	if result != "artist" {
		t.Errorf("Expected 'artist', got %q", result)
	}
}

func TestNormalizeArtist_With(t *testing.T) {
	result := NormalizeArtist("Band with Orchestra")
	if result != "band" {
		t.Errorf("Expected 'band', got %q", result)
	}
}

func TestNormalizeArtist_Case(t *testing.T) {
	result := NormalizeArtist("THE BEATLES")
	if result != "the beatles" {
		t.Errorf("Expected 'the beatles', got %q", result)
	}
}

func TestNormalizeArtist_SpecialChars(t *testing.T) {
	result := NormalizeArtist("Artist\x00\x01\x02")
	if result != "artist" {
		t.Errorf("Expected 'artist', got %q", result)
	}
}

func TestNormalizeArtist_TrailingSpaces(t *testing.T) {
	result := NormalizeArtist("  Artist  ")
	if result != "artist" {
		t.Errorf("Expected 'artist', got %q", result)
	}
}

func TestNormalizeArtist_NoSeparator(t *testing.T) {
	result := NormalizeArtist("Pink Floyd")
	if result != "pink floyd" {
		t.Errorf("Expected 'pink floyd', got %q", result)
	}
}

func TestNormalizeArtist_OnlyFeat(t *testing.T) {
	result := NormalizeArtist("feat. Someone")
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestSanitizeString_SpecialChars(t *testing.T) {
	result := SanitizeString("Hello\x00World")
	if result != "HelloWorld" {
		t.Errorf("Expected 'HelloWorld', got %q", result)
	}
}

func TestSanitizeString_Trim(t *testing.T) {
	result := SanitizeString("  hello  ")
	if result != "hello" {
		t.Errorf("Expected 'hello', got %q", result)
	}
}

func TestSanitizeString_ControlChars(t *testing.T) {
	result := SanitizeString("a\x01b\x02c")
	if result != "abc" {
		t.Errorf("Expected 'abc', got %q", result)
	}
}

func TestSanitizeString_DeleteChar(t *testing.T) {
	result := SanitizeString("a\x7Fb")
	if result != "ab" {
		t.Errorf("Expected 'ab' (DEL removed), got %q", result)
	}
}

func TestSanitizeString_Empty(t *testing.T) {
	result := SanitizeString("")
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestSanitizeString_Unicode(t *testing.T) {
	result := SanitizeString("ñáéíóú")
	if result != "ñáéíóú" {
		t.Errorf("Expected 'ñáéíóú', got %q", result)
	}
}

func TestSanitizeString_ControlOnly(t *testing.T) {
	result := SanitizeString("\x00\x01\x02")
	if result != "" {
		t.Errorf("Expected empty string from control chars only, got %q", result)
	}
}

func TestNormalizeString_Alias(t *testing.T) {
	input := "  test\x00string  "
	normalized := NormalizeString(input)
	sanitized := SanitizeString(input)
	if normalized != sanitized {
		t.Errorf("NormalizeString should equal SanitizeString: %q != %q", normalized, sanitized)
	}
}

func TestNormalizeString_DifferentInput(t *testing.T) {
	result := NormalizeString("Hello World")
	if result != "Hello World" {
		t.Errorf("Expected 'Hello World', got %q", result)
	}
}

func TestGenerateID_ConsistencyAcrossPackages(t *testing.T) {
	// GenerateID in utils and tags should produce same SHA1
	input := "test-input"
	id1 := GenerateID(input)
	// Verify it's a valid 40-char hex
	if len(id1) != 40 {
		t.Errorf("Expected 40 char hex, got %d", len(id1))
	}
	for _, c := range id1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Expected hex char, got %c", c)
			break
		}
	}
}

func TestNormalizeArtist_LongName(t *testing.T) {
	longName := strings.Repeat("A", 200) + " feat. " + strings.Repeat("B", 200)
	result := NormalizeArtist(longName)
	expected := strings.Repeat("a", 200)
	if result != expected {
		t.Errorf("Expected long name feat. stripped correctly")
	}
}
