package db

import (
	"os"
	"testing"
)

func TestConnectAndInitSchema(t *testing.T) {
	// Usamos un archivo de prueba en memoria o temporal
	f, err := os.CreateTemp("", "sgound-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp test db: %v", err)
	}
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, err := ConnectMaster(dbPath)
	if err != nil {
		t.Fatalf("Expected no error connecting to DB, got %v", err)
	}
	defer database.Conn.Close()

	// Verify key tables are created in the centralized schema
	tables := []string{"users", "artists", "albums", "tracks", "user_tracks", "playlists"}
	for _, table := range tables {
		var name string
		err := database.Conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name = ?;", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created", table)
		}
	}
}
