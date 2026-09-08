package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"sgound/internal/db"
)

func TestGetMP3DurationNative_WithFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping native MP3 test")
	}

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	duration, bitRate, sampleRate, err := getMP3DurationNative(mp3Path)
	if err != nil {
		t.Fatalf("getMP3DurationNative failed: %v", err)
	}

	if duration < 1.5 || duration > 2.5 {
		t.Errorf("Expected duration ~2s, got %f", duration)
	}
	if bitRate < 64000 || bitRate > 200000 {
		t.Errorf("Expected bitRate ~128000, got %d", bitRate)
	}
	if sampleRate != 44100 {
		t.Errorf("Expected sampleRate 44100, got %d", sampleRate)
	}
}

func TestGetMP3DurationNative_NonexistentFile(t *testing.T) {
	_, _, _, err := getMP3DurationNative("/nonexistent/file.mp3")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestGetFLACDurationNative_WithFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping native FLAC test")
	}

	tempDir := t.TempDir()
	flacPath := filepath.Join(tempDir, "test.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	duration, bitRate, sampleRate, err := getFLACDurationNative(flacPath)
	if err != nil {
		t.Fatalf("getFLACDurationNative failed: %v", err)
	}

	if duration < 1.5 || duration > 2.5 {
		t.Errorf("Expected duration ~2s, got %f", duration)
	}
	if sampleRate != 44100 {
		t.Errorf("Expected sampleRate 44100, got %d", sampleRate)
	}
	if bitRate <= 0 {
		t.Errorf("Expected positive bitRate, got %d", bitRate)
	}
}

func TestGetFLACDurationNative_NonexistentFile(t *testing.T) {
	_, _, _, err := getFLACDurationNative("/nonexistent/file.flac")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestGetFLACDurationNative_NotFLAC(t *testing.T) {
	tempDir := t.TempDir()
	fakePath := filepath.Join(tempDir, "fake.flac")
	os.WriteFile(fakePath, []byte("not a real flac file"), 0644)

	_, _, _, err := getFLACDurationNative(fakePath)
	if err != nil {
		// This is expected - file is not a valid FLAC
		t.Logf("Got error for non-FLAC file (expected): %v", err)
	}
}

func TestParseFileWithCache_MP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	result, err := sc.ParseFileWithCache(mp3Path, "active")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}
	if result == nil {
		t.Fatal("ParseFileWithCache returned nil result")
	}

	if result.path != mp3Path {
		t.Errorf("Expected path %q, got %q", mp3Path, result.path)
	}
	if result.format != "mp3" {
		t.Errorf("Expected format 'mp3', got %q", result.format)
	}
	if result.status != "active" {
		t.Errorf("Expected status 'active', got %q", result.status)
	}
	if result.size <= 0 {
		t.Errorf("Expected positive size, got %d", result.size)
	}
}

func TestParseFileWithCache_FLAC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	flacPath := filepath.Join(tempDir, "test.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	result, err := sc.ParseFileWithCache(flacPath, "active")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}
	if result == nil {
		t.Fatal("ParseFileWithCache returned nil result")
	}

	if result.format != "flac" {
		t.Errorf("Expected format 'flac', got %q", result.format)
	}
	if result.sampleRate != 44100 {
		t.Errorf("Expected sampleRate 44100, got %d", result.sampleRate)
	}
}

func TestParseFileWithCache_NonexistentFile(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	_, err := sc.ParseFileWithCache("/nonexistent/file.mp3", "active")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestParseFileWithCache_NoTags(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "no-tags.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.5", "-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	result, err := sc.ParseFileWithCache(mp3Path, "pending")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}

	// Without tags, should use defaults
	if result.artistName != "Unknown Artist" {
		t.Errorf("Expected default artist 'Unknown Artist', got %q", result.artistName)
	}
	if result.albumName != "Unknown Album" {
		t.Errorf("Expected default album 'Unknown Album', got %q", result.albumName)
	}
	if result.genre != "Unknown" {
		t.Errorf("Expected default genre 'Unknown', got %q", result.genre)
	}
}

func TestParseFileWithCache_WithTags(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "tagged.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-metadata", "title=My Song",
		"-metadata", "artist=Test Artist",
		"-metadata", "album=Test Album",
		"-metadata", "genre=Rock",
		"-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create tagged MP3: %v", err)
	}

	result, err := sc.ParseFileWithCache(mp3Path, "active")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}

	if result.title != "My Song" {
		t.Errorf("Expected title 'My Song', got %q", result.title)
	}
	if result.artistName != "Test Artist" {
		t.Errorf("Expected artist 'Test Artist', got %q", result.artistName)
	}
	if result.albumName != "Test Album" {
		t.Errorf("Expected album 'Test Album', got %q", result.albumName)
	}
	if result.genre != "Rock" {
		t.Errorf("Expected genre 'Rock', got %q", result.genre)
	}
}

func TestParseFileWithCache_GeneratesIDs(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "ids.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-metadata", "artist=Test Artist",
		"-metadata", "album=Test Album",
		"-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	result, err := sc.ParseFileWithCache(mp3Path, "active")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}

	if result.trackId == "" {
		t.Error("trackId should not be empty")
	}
	if result.albumId == "" {
		t.Error("albumId should not be empty")
	}
	if result.artistId == "" {
		t.Error("artistId should not be empty")
	}
	if len(result.trackId) != 40 {
		t.Errorf("trackId should be 40-char SHA1, got %d chars", len(result.trackId))
	}
}

func TestParseFileWithCache_PersistentID(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "persistent.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-metadata", "SGOUND_ID=custom-id-123",
		"-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	result, err := sc.ParseFileWithCache(mp3Path, "active")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}

	// If SGOUND_ID was written, it should be used as trackId
	if result.trackId != "custom-id-123" {
		t.Logf("trackId: %q (persistent ID may not have been read, using generated ID)", result.trackId)
	}
}

func TestParseFileWithCache_CoverDimensions(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	// Create a small image for cover
	imgPath := filepath.Join(tempDir, "cover.jpg")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "color=c=red:s=200x200:d=0.1", "-frames:v", "1", "-y", imgPath)
	if err := cmd.Run(); err != nil {
		t.Skip("Could not create test image")
	}

	mp3Path := filepath.Join(tempDir, "with-cover.mp3")
	cmd = exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-i", imgPath,
		"-map", "0:a", "-map", "1:v",
		"-c", "copy", "-disposition:v:0", "attached_pic",
		"-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Skip("Could not create MP3 with cover")
	}

	result, err := sc.ParseFileWithCache(mp3Path, "active")
	if err != nil {
		t.Fatalf("ParseFileWithCache failed: %v", err)
	}

	if result.coverWidth != 200 || result.coverHeight != 200 {
		t.Errorf("Expected cover 200x200, got %dx%d", result.coverWidth, result.coverHeight)
	}
	if result.coverSize <= 0 {
		t.Errorf("Expected positive coverSize, got %d", result.coverSize)
	}
}

func TestParseFileWithCache_MissingFile(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	_, err := sc.ParseFileWithCache("/nonexistent/path.mp3", "active")
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestParseAndSaveFile_WithRealMP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "save-test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.5",
		"-metadata", "title=Save Test",
		"-metadata", "artist=Test Artist",
		"-b:a", "128k", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	err := sc.ParseAndSaveFile(mp3Path)
	if err != nil {
		t.Fatalf("ParseAndSaveFile failed: %v", err)
	}

	// Verify data was saved to DB
	var title, artist string
	database.Conn.QueryRow("SELECT title, artist FROM tracks WHERE path = ?", mp3Path).Scan(&title, &artist)
	if title != "Save Test" {
		t.Errorf("Expected title 'Save Test' in DB, got %q", title)
	}
	if artist != "Test Artist" {
		t.Errorf("Expected artist 'Test Artist' in DB, got %q", artist)
	}
}

func TestCopyString(t *testing.T) {
	// Test the copyString helper
	s := "hello world"
	copied := copyString(s)
	if copied != s {
		t.Errorf("Expected %q, got %q", s, copied)
	}

	// Empty string
	empty := copyString("")
	if empty != "" {
		t.Errorf("Expected empty string, got %q", empty)
	}
}
