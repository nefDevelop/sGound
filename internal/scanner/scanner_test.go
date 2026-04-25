package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"sgound/internal/db"
)

func TestScanner_ScanDirectory(t *testing.T) {
	// 1. Create temporary directory for music
	tempDir, err := os.MkdirTemp("", "sgound-test-music-*")
	if err != nil {
		t.Fatalf("Failed to create temp music dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Create some dummy files
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

	// 3. Setup temporary database
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

	// 4. Initialize scanner and scan
	sc := NewScanner(database)
	err = sc.ScanDirectory(tempDir)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// 5. Verify results in DB
	var count int
	err = database.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&count)
	if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 tracks in DB, found %d", count)
	}

	// Verify artist and album were created
	var artistName string
	err = database.Conn.QueryRow("SELECT name FROM artists").Scan(&artistName)
	if err != nil {
		t.Errorf("Failed to find artist in DB: %v", err)
	}
}
