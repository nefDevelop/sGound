package scanner

import (
	"os"
	"sgound/internal/db"
	"testing"
)

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
