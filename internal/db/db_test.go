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

	database, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Expected no error connecting to DB, got %v", err)
	}
	defer database.Conn.Close()

	// Verify tables are created
	rows, err := database.Conn.Query("SELECT name FROM sqlite_master WHERE type='table' AND name IN ('artists', 'albums', 'tracks');")
	if err != nil {
		t.Fatalf("Expected no error querying schema, got %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 tables created, got %d", count)
	}
}
