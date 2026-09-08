package tags

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOpen_MP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	meta, err := Open(mp3Path)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", mp3Path, err)
	}
	defer meta.Close()

	if meta.Title() == "" {
		// Empty title is acceptable for a freshly created file
		t.Log("MP3 opened successfully with empty title (expected)")
	}
}

func TestOpen_FLAC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	flacPath := filepath.Join(tempDir, "test.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	meta, err := Open(flacPath)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", flacPath, err)
	}
	defer meta.Close()
}

func TestOpen_M4A(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	m4aPath := filepath.Join(tempDir, "test.m4a")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-f", "ipod", "-y", m4aPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test M4A: %v", err)
	}

	meta, err := Open(m4aPath)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", m4aPath, err)
	}
	defer meta.Close()
}

func TestOpen_OGG(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	oggPath := filepath.Join(tempDir, "test.ogg")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-f", "ogg", "-y", oggPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test OGG: %v", err)
	}

	meta, err := Open(oggPath)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", oggPath, err)
	}
	defer meta.Close()
}

func TestOpen_Unsupported(t *testing.T) {
	tempDir := t.TempDir()
	txtPath := filepath.Join(tempDir, "file.txt")
	os.WriteFile(txtPath, []byte("text"), 0644)

	_, err := Open(txtPath)
	if err == nil {
		t.Error("Open should fail for unsupported format")
	}
}

func TestOpen_CaseInsensitive(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "test.MP3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	meta, err := Open(mp3Path)
	if err != nil {
		t.Fatalf("Open(%q) should handle uppercase extension: %v", mp3Path, err)
	}
	defer meta.Close()
}

func TestReadPersistentID_MP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// No SGOUND_ID written yet
	id := ReadPersistentID(mp3Path)
	if id != "" {
		t.Errorf("Expected empty persistent ID, got %q", id)
	}
}

func TestReadPersistentID_NonexistentFile(t *testing.T) {
	id := ReadPersistentID("/nonexistent/file.mp3")
	if id != "" {
		t.Errorf("Expected empty ID for nonexistent file, got %q", id)
	}
}

func TestReadPersistentID_FLAC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	tempDir := t.TempDir()
	flacPath := filepath.Join(tempDir, "test.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	id := ReadPersistentID(flacPath)
	if id != "" {
		t.Errorf("Expected empty persistent ID, got %q", id)
	}
}

func TestGenerateUUID_Format(t *testing.T) {
	uuid := GenerateUUID()
	// UUID format: 8-4-4-4-12 (36 chars + 4 hyphens = 40)
	if len(uuid) != 36 {
		t.Errorf("Expected UUID length 36, got %d: %s", len(uuid), uuid)
	}
	// Check it contains hyphens at expected positions
	positions := []int{8, 13, 18, 23}
	for _, pos := range positions {
		if uuid[pos] != '-' {
			t.Errorf("Expected '-' at position %d, got %c in %s", pos, uuid[pos], uuid)
		}
	}
}

func TestGenerateUUID_Unique(t *testing.T) {
	uuid1 := GenerateUUID()
	uuid2 := GenerateUUID()
	if uuid1 == uuid2 {
		t.Error("Two GenerateUUID calls should produce different values")
	}
}

func TestGenerateID_SHA1(t *testing.T) {
	id := GenerateID("hello")
	// SHA1 of "hello" is aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	expected := "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
	if id != expected {
		t.Errorf("Expected SHA1 of 'hello' = %q, got %q", expected, id)
	}
}

func TestGenerateID_Empty(t *testing.T) {
	id := GenerateID("")
	// SHA1 of empty string
	expected := "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	if id != expected {
		t.Errorf("Expected SHA1 of '' = %q, got %q", expected, id)
	}
}

func TestNormalizeArtist_Feat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Artist feat. Someone", "artist"},
		{"Artist ft. Someone", "artist"},
		{"Artist feat Someone", "artist"},
		{"Artist ft Someone", "artist"},
		{"Pink Floyd", "pink floyd"},
		{"  Spaced  ", "spaced"},
	}
	for _, tt := range tests {
		result := NormalizeArtist(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeArtist(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeString_SpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"Hello.World", "helloworld"},
		{"Hello, World", "hello world"},
		{"Hello-World", "helloworld"},
		{"Hello (World)", "hello world"},
		{"Track [Remastered]", "track remastered"},
		{"It's", "its"},
		{"She Said \"Yes\"", "she said yes"},
	}
	for _, tt := range tests {
		result := NormalizeString(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetBoolInt(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int
	}{
		{true, 1},
		{false, 0},
		{"true", 0},
		{nil, 0},
		{1, 0},
	}
	for _, tt := range tests {
		data := map[string]interface{}{"key": tt.input}
		result := getBoolInt(data, "key")
		if result != tt.expected {
			t.Errorf("getBoolInt(%v) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"hello", "hello"},
		{42, "42"},
		{3.14, "3"},
		{nil, ""},
	}
	for _, tt := range tests {
		data := map[string]interface{}{"key": tt.input}
		result := getString(data, "key")
		if result != tt.expected {
			t.Errorf("getString(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetString_MissingKey(t *testing.T) {
	data := map[string]interface{}{"other": "value"}
	result := getString(data, "key")
	if result != "" {
		t.Errorf("getString for missing key should return '', got %q", result)
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int
	}{
		{42.0, 42},
		{42, 42},
		{"42", 42},
		{"not_a_number", 0},
		{nil, 0},
	}
	for _, tt := range tests {
		data := map[string]interface{}{"key": tt.input}
		result := getInt(data, "key")
		if result != tt.expected {
			t.Errorf("getInt(%v) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestIsSoftMatch_Equal(t *testing.T) {
	if !isSoftMatch("title", "hello", "hello") {
		t.Error("isSoftMatch should return true for equal values")
	}
}

func TestIsSoftMatch_LeadingZeros(t *testing.T) {
	if !isSoftMatch("trackNum", "01", "1") {
		t.Error("isSoftMatch should handle leading zeros for trackNum")
	}
	if !isSoftMatch("discNum", "02", "2") {
		t.Error("isSoftMatch should handle leading zeros for discNum")
	}
	if !isSoftMatch("year", "0001", "1") {
		t.Error("isSoftMatch should handle leading zeros for year")
	}
}

func TestIsSoftMatch_Different(t *testing.T) {
	if isSoftMatch("title", "hello", "world") {
		t.Error("isSoftMatch should return false for different values")
	}
}

func TestIsSoftMatch_NonNumericField(t *testing.T) {
	// Leading zero trimming should only apply to trackNum, discNum, year
	if isSoftMatch("title", "01", "1") {
		t.Error("isSoftMatch should NOT trim leading zeros for title")
	}
}
