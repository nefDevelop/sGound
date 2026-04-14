package scanner

import (
	"os"
	"testing"
	"sgound/internal/db"
)

func TestIsAudioFile(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		want bool
	}{
		{"Test mp3", ".mp3", true},
		{"Test flac", ".flac", true},
		{"Test ogg", ".Ogg", true},
		{"Test txt", ".txt", false},
	}
	for _, c := range cases {
		got := isAudioFile("song" + c.ext)
		if (got == nil) != c.want {
			t.Errorf("isAudioFile(%q) == %v, want %v", c.ext, got == nil, c.want)
		}
	}
}

func TestScanDirectoryEmpty(t *testing.T) {
	// Usamos DB en memoria
	f, _ := os.CreateTemp("", "sgound-scan-*.db")
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, _ := db.ConnectMaster(dbPath)
	defer database.Conn.Close()

	s := NewScanner(database)

	tmpDir, _ := os.MkdirTemp("", "music-root")
	defer os.RemoveAll(tmpDir)

	err := s.ScanDirectory(tmpDir)
	if err != nil {
		t.Errorf("Expected scan to pass on empty directory, got: %v", err)
	}
}
