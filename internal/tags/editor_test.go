package tags

import (
	"os"
	"path/filepath"
	"sgound/internal/db"
	"testing"
)

func TestOrganizeFileLogic(t *testing.T) {
	// 1. Setup DB en memoria
	database, _ := db.Connect(":memory:")
	defer database.Conn.Close()

	// 2. Mock Data
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('artist-1', 'Daft Punk')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('album-1', 'artist-1', 'Discovery', 2001)")
	
	tempDir, _ := os.MkdirTemp("", "sgound-test-*")
	defer os.RemoveAll(tempDir)
	
	oldPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(oldPath, []byte("fake mp3"), 0644)

	database.Conn.Exec(`
		INSERT INTO tracks (id, album_id, title, track_num, path, format, genre, disc_num, bit_rate) 
		VALUES ('track-1', 'album-1', 'One More Time', 1, ?, 'mp3', 'House', 1, 320000)`, oldPath)

	editor := NewEditor(database)
	
	// 3. Test Pattern
	pattern := "{artist}/{year} - {album}/{track} - {title}"
	newPath, err := editor.OrganizeFile("track-1", tempDir, pattern)

	if err != nil {
		t.Fatalf("OrganizeFile failed: %v", err)
	}

	expectedRel := "Daft Punk/2001 - Discovery/01 - One More Time.mp3"
	expectedPath := filepath.Join(tempDir, expectedRel)

	if newPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, newPath)
	}

	// 4. Verificar que el archivo se movió realmente
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("Physical file was not moved to new location")
	}
}
