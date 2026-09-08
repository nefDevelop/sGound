package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"sgound/internal/db"
)

func TestScanner_ScanDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sgound-test-music-*")
	if err != nil {
		t.Fatalf("Failed to create temp music dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	artistDir := filepath.Join(tempDir, "Test Artist")
	os.MkdirAll(artistDir, 0755)

	files := []string{
		filepath.Join(artistDir, "track1.mp3"),
		filepath.Join(artistDir, "track2.flac"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("fake audio data"), 0644); err != nil {
			t.Fatalf("Failed to create dummy file %s: %v", f, err)
		}
	}

	dbFile, err := os.CreateTemp("", "sgound-test-db-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, err := db.ConnectMaster(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}
	defer database.Conn.Close()

	sc := NewScanner(database)
	err = sc.ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	var count int
	err = database.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&count)
	if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 tracks in DB, found %d", count)
	}

	var artistName string
	err = database.Conn.QueryRow("SELECT name FROM artists").Scan(&artistName)
	if err != nil {
		t.Errorf("Failed to find artist in DB: %v", err)
	}
}

func TestIsAudioFile_Supported(t *testing.T) {
	supported := []string{
		"track.mp3", "song.flac", "music.ogg", "audio.m4a",
		"clip.mp4", "voice.opus", "recording.wav",
	}
	for _, name := range supported {
		if err := IsAudioFile(name); err != nil {
			t.Errorf("IsAudioFile(%q) should return nil, got %v", name, err)
		}
	}
}

func TestIsAudioFile_Unsupported(t *testing.T) {
	unsupported := []string{
		"doc.txt", "image.jpg", "video.mkv", "archive.zip",
		"readme.md", "data.csv",
	}
	for _, name := range unsupported {
		if err := IsAudioFile(name); err == nil {
			t.Errorf("IsAudioFile(%q) should return error", name)
		}
	}
}

func TestIsAudioFile_CaseInsensitive(t *testing.T) {
	cases := []string{"track.MP3", "song.FLAC", "music.OGG", "audio.M4A", "clip.Opus"}
	for _, name := range cases {
		if err := IsAudioFile(name); err != nil {
			t.Errorf("IsAudioFile(%q) should be case-insensitive, got %v", name, err)
		}
	}
}

func TestIsAudioFile_NoExtension(t *testing.T) {
	if err := IsAudioFile("noext"); err == nil {
		t.Error("IsAudioFile('noext') should return error")
	}
}

func TestIsAudioFile_EmptyString(t *testing.T) {
	if err := IsAudioFile(""); err == nil {
		t.Error("IsAudioFile('') should return error")
	}
}

func TestNewScanner_InitialState(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)

	if sc.DB == nil {
		t.Error("Scanner.DB should not be nil")
	}
	if sc.PriorityJobs == nil {
		t.Error("PriorityJobs channel should not be nil")
	}
	if sc.ramTickets == nil {
		t.Error("ramTickets channel should not be nil")
	}
	if sc.resolvedAlbums == nil {
		t.Error("resolvedAlbums should not be nil")
	}
	if sc.conflictedAlbums == nil {
		t.Error("conflictedAlbums should not be nil")
	}
	if sc.seenArtists == nil {
		t.Error("seenArtists should not be nil")
	}
	if sc.seenAlbums == nil {
		t.Error("seenAlbums should not be nil")
	}
	if sc.dbFileCache == nil {
		t.Error("dbFileCache should not be nil")
	}
}

func TestScanner_GetStatus_Initial(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)
	isPaused, ramMB := sc.GetStatus()

	if isPaused {
		t.Error("Scanner should not be paused initially")
	}
	if ramMB != 0 {
		t.Errorf("Expected RAM 0MB initially, got %d", ramMB)
	}
}

func TestScanner_ScanDirectory_Empty(t *testing.T) {
	tempDir := t.TempDir()

	dbFile, _ := os.CreateTemp("", "sgound-test-db-*.db")
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, err := db.ConnectMaster(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}
	defer database.Conn.Close()

	sc := NewScanner(database)
	err = sc.ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory on empty dir failed: %v", err)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 tracks in empty dir, found %d", count)
	}
}

func TestScanner_ScanDirectory_NestedDirs(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested structure with non-audio files
	nested := filepath.Join(tempDir, "Artist", "Album", "Disc1")
	os.MkdirAll(nested, 0755)
	os.WriteFile(filepath.Join(nested, "readme.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(nested, "notes.md"), []byte("notes"), 0644)

	// Create audio file at root level
	os.WriteFile(filepath.Join(tempDir, "song.mp3"), []byte("audio"), 0644)

	dbFile, _ := os.CreateTemp("", "sgound-test-db-*.db")
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, _ := db.ConnectMaster(dbPath)
	defer database.Conn.Close()

	sc := NewScanner(database)
	err := sc.ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 track (only mp3), found %d", count)
	}
}

func TestScanner_ScanDirectory_SkipsNonAudio(t *testing.T) {
	tempDir := t.TempDir()

	// Mix of audio and non-audio
	os.WriteFile(filepath.Join(tempDir, "song.mp3"), []byte("audio"), 0644)
	os.WriteFile(filepath.Join(tempDir, "cover.jpg"), []byte("img"), 0644)
	os.WriteFile(filepath.Join(tempDir, "lyrics.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(tempDir, "notes.pdf"), []byte("pdf"), 0644)

	dbFile, _ := os.CreateTemp("", "sgound-test-db-*.db")
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, _ := db.ConnectMaster(dbPath)
	defer database.Conn.Close()

	sc := NewScanner(database)
	sc.ScanDirectory(tempDir)

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 track (mp3 only), found %d", count)
	}
}

func TestParseAndSaveFile_NonexistentFile(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	sc := NewScanner(database)
	err := sc.ParseAndSaveFile("/nonexistent/path.mp3")
	if err == nil {
		t.Error("ParseAndSaveFile should fail for nonexistent file")
	}
}

func TestScanner_ScanDirectory_Callbacks(t *testing.T) {
	tempDir := t.TempDir()
	os.WriteFile(filepath.Join(tempDir, "song.mp3"), []byte("audio"), 0644)

	dbFile, _ := os.CreateTemp("", "sgound-test-db-*.db")
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, _ := db.ConnectMaster(dbPath)
	defer database.Conn.Close()

	sc := NewScanner(database)

	progressCalled := false
	discoveryCalled := false

	sc.OnProgress = func(path string, count int) {
		progressCalled = true
	}
	sc.OnDiscovery = func(count int) {
		discoveryCalled = true
	}

	sc.ScanDirectory(tempDir)

	if !discoveryCalled {
		t.Error("OnDiscovery callback should have been called")
	}
	// OnProgress may or may not be called depending on timing
	_ = progressCalled
}
