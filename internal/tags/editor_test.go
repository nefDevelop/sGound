package tags

import (
	"os"
	"os/exec"
	"path/filepath"
	"sgound/internal/db"
	"strings"
	"testing"
	"time"
)

func setupTestDB() *db.MasterDatabase {
	database, _ := db.ConnectMaster(":memory:")
	return database
}

func TestOrganizeFileComprehensive(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir, _ := os.MkdirTemp("", "sgound-org-test-*")
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name     string
		artist   string
		album    string
		year     int
		title    string
		track    int
		disc     int
		genre    string
		bitrate  int
		pattern  string
		expected string // Relative to tempDir
	}{
		{
			name:     "Standard Pattern",
			artist:   "Daft Punk",
			album:    "Discovery",
			year:     2001,
			title:    "One More Time",
			track:    1,
			disc:     1,
			genre:    "House",
			bitrate:  320000,
			pattern:  "{artist}/{year} - {album}/{track} - {title}",
			expected: "Daft Punk/2001 - Discovery/01 - One More Time.mp3",
		},
		{
			name:     "Genre and Bitrate",
			artist:   "Radiohead",
			album:    "Kid A",
			year:     2000,
			title:    "Everything In Its Right Place",
			track:    1,
			disc:     1,
			genre:    "Electronic",
			bitrate:  1411000,
			pattern:  "{genre}/{artist}/{album} ({bitrate})/{title}",
			expected: "Electronic/Radiohead/Kid A (1411k)/Everything In Its Right Place.mp3",
		},
		{
			name:     "Special Characters Cleaning",
			artist:   "AC/DC",
			album:    "Back in Black: Special Edition",
			year:     1980,
			title:    "Hells Bells?",
			track:    1,
			disc:     1,
			genre:    "Rock",
			bitrate:  256000,
			pattern:  "{artist}/{album}/{title}",
			expected: "AC-DC/Back in Black- Special Edition/Hells Bells-.mp3",
		},
		{
			name:     "Missing Tags Fallbacks",
			artist:   "",
			album:    "",
			year:     0,
			title:    "Unknown Track",
			track:    0,
			disc:     0,
			genre:    "",
			bitrate:  128000,
			pattern:  "{genre}/{artist}/{year}/{album}/{title}",
			expected: "Unknown/Unknown Artist/Unknown Year/Unknown Album/Unknown Track.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Limpiar DB para cada subtest
			db.Conn.Exec("DELETE FROM tracks")
			db.Conn.Exec("DELETE FROM albums")
			db.Conn.Exec("DELETE FROM artists")

			artistID := "art-" + tt.name
			albumID := "alb-" + tt.name
			trackID := "trk-" + tt.name

			db.Conn.Exec("INSERT INTO artists (id, name) VALUES (?, ?)", artistID, tt.artist)
			db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumID, artistID, tt.album, tt.year)
			
			// Crear archivo físico inicial
			oldRel := "unsorted/random_file.mp3"
			oldPath := filepath.Join(tempDir, oldRel)
			os.MkdirAll(filepath.Dir(oldPath), 0755)
			os.WriteFile(oldPath, []byte("audio data"), 0644)

			db.Conn.Exec(`
				INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, format, genre, bit_rate) 
				VALUES (?, ?, ?, ?, ?, ?, 'mp3', ?, ?)`,
				trackID, albumID, tt.title, tt.track, tt.disc, oldPath, tt.genre, tt.bitrate)

			editor := NewEditor(db)
			newPath, err := editor.OrganizeFile(trackID, tempDir, tt.pattern)
			if err != nil {
				t.Fatalf("OrganizeFile failed: %v", err)
			}

			expectedPath := filepath.Join(tempDir, tt.expected)
			if newPath != expectedPath {
				t.Errorf("Path mismatch.\nExpected: %s\nGot:      %s", expectedPath, newPath)
			}

			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				t.Error("Physical file missing at new location")
			}
		})
	}
}

func TestOrganizeFileFolderCleanup(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir, _ := os.MkdirTemp("", "sgound-cleanup-test-*")
	defer os.RemoveAll(tempDir)

	// Crear estructura: tempDir/old_folder/sub/file.mp3
	oldFolder := filepath.Join(tempDir, "old_folder", "sub")
	os.MkdirAll(oldFolder, 0755)
	oldPath := filepath.Join(oldFolder, "file.mp3")
	os.WriteFile(oldPath, []byte("data"), 0644)

	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	db.Conn.Exec(`INSERT INTO tracks (id, album_id, title, path, format) VALUES ('t1', 'al1', 'Song', ?, 'mp3')`, oldPath)

	editor := NewEditor(db)
	_, err := editor.OrganizeFile("t1", tempDir, "new_folder/{title}")
	if err != nil {
		t.Fatal(err)
	}

	// Esperar un momento a que la goroutine de limpieza actúe (más que el sleep interno de 100ms)
	time.Sleep(200 * time.Millisecond)

	// Verificar que old_folder/sub ha desaparecido
	if _, err := os.Stat(oldFolder); !os.IsNotExist(err) {
		t.Error("Old folder should have been deleted")
	}
	
	parentOfOld := filepath.Join(tempDir, "old_folder")
	if _, err := os.Stat(parentOfOld); !os.IsNotExist(err) {
		t.Error("Old parent folder should have been deleted")
	}
}

func TestOrganizeFile_MaliciousPattern(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir := t.TempDir()
	
	// Crear archivo físico inicial
	oldRel := "unsorted/random_file.mp3"
	oldPath := filepath.Join(tempDir, oldRel)
	os.MkdirAll(filepath.Dir(oldPath), 0755)
	os.WriteFile(oldPath, []byte("audio data"), 0644)

	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	db.Conn.Exec(`INSERT INTO tracks (id, album_id, title, path, format) VALUES ('t1', 'al1', 'Song', ?, 'mp3')`, oldPath)

	editor := NewEditor(db)
	
	// Patrón malicioso que intenta escapar de tempDir
	maliciousPattern := "../../../etc/hacked/{title}"
	
	_, err := editor.OrganizeFile("t1", tempDir, maliciousPattern)
	if err == nil {
		t.Fatal("Expected OrganizeFile to block path traversal, but it succeeded")
	}
	
	if !strings.Contains(err.Error(), "security violation") {
		t.Errorf("Expected security violation error, got: %v", err)
	}
}

func TestOrganizeFile_MaliciousTags(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir := t.TempDir()
	
	oldRel := "unsorted/random_file.mp3"
	oldPath := filepath.Join(tempDir, oldRel)
	os.MkdirAll(filepath.Dir(oldPath), 0755)
	os.WriteFile(oldPath, []byte("audio data"), 0644)

	// Inyectar tags con barras y otros caracteres
	maliciousArtist := "../Artist"
	maliciousTitle := "Song/../../escaped"
	
	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', ?)", maliciousArtist)
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	db.Conn.Exec(`INSERT INTO tracks (id, album_id, title, path, format) VALUES ('t1', 'al1', ?, ?, 'mp3')`, maliciousTitle, oldPath)

	editor := NewEditor(db)
	
	safePattern := "{artist}/{title}"
	
	newPath, err := editor.OrganizeFile("t1", tempDir, safePattern)
	if err != nil {
		t.Fatalf("OrganizeFile failed on tags with special chars: %v", err)
	}
	
	// Verificar que clean() haya funcionado (reemplaza / por -)
	expectedRel := "..-Artist/Song-..-..-escaped.mp3"
	expectedPath := filepath.Join(tempDir, expectedRel)
	
	if newPath != expectedPath {
		t.Errorf("Path mismatch.\nExpected: %s\nGot:      %s", expectedPath, newPath)
	}
}

func TestMoveFile_Failures(t *testing.T) {
	tempDir := t.TempDir()
	
	// 1. Mover desde un archivo que no existe
	err := moveFile(filepath.Join(tempDir, "does-not-exist.mp3"), filepath.Join(tempDir, "dest.mp3"))
	if err == nil {
		t.Error("Expected moveFile to fail when source does not exist")
	}
	
	// 2. Mover a un destino donde no hay permisos (usando un directorio sin permisos)
	srcPath := filepath.Join(tempDir, "src.mp3")
	os.WriteFile(srcPath, []byte("data"), 0644)
	
	destDir := filepath.Join(tempDir, "readonly_dir")
	os.MkdirAll(destDir, 0555) // Read and execute only
	defer os.Chmod(destDir, 0755) // Limpiar después
	
	destPath := filepath.Join(destDir, "dest.mp3")
	err = moveFile(srcPath, destPath)
	if err == nil {
		// En algunos sistemas (Windows o root), esto puede no fallar.
		// Asumimos que corre en entorno estándar sin root.
		t.Error("Expected moveFile to fail when destination is read-only")
	}
}

func createTestMP3(t *testing.T, path string) {
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-y", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}
}

func TestUpdateTags_Success(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "song.mp3")
	createTestMP3(t, testPath)

	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('art1', 'Old Artist')")
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('alb1', 'art1', 'Old Album', 2000)")
	db.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, format) VALUES ('trk1', 'alb1', 'Old Title', ?, 'mp3')", testPath)

	editor := NewEditor(db)
	
	updates := map[string]interface{}{
		"title": "New Title",
		"year":  2024,
	}
	
	_, err := editor.UpdateTags("trk1", updates)
	if err != nil {
		t.Fatalf("UpdateTags failed: %v", err)
	}
	
	// Check DB
	var newTitle string
	db.Conn.QueryRow("SELECT title FROM tracks WHERE id = 'trk1'").Scan(&newTitle)
	if newTitle != "New Title" {
		t.Errorf("DB title not updated: %v", newTitle)
	}
	
	var newYear int
	db.Conn.QueryRow("SELECT year FROM albums WHERE id = 'alb1'").Scan(&newYear)
	if newYear != 2024 {
		t.Errorf("DB year not updated: %v", newYear)
	}
	
	// Check physical file
	tags, _ := ReadTagsFromFile(testPath)
	if tags["title"] != "New Title" {
		t.Errorf("File title not updated: %v", tags["title"])
	}
	if tags["year"] != "2024" {
		t.Errorf("File year not updated: %v", tags["year"])
	}
}

func TestUpdateTags_AlbumChange(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "song.mp3")
	createTestMP3(t, testPath)

	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('art1', 'Old Artist')")
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('alb1', 'art1', 'Old Album', 2000)")
	db.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, format) VALUES ('trk1', 'alb1', 'Song', ?, 'mp3')", testPath)

	editor := NewEditor(db)
	
	updates := map[string]interface{}{
		"album": "Brand New Album",
		"albumArtist": "Brand New Artist",
	}
	
	_, err := editor.UpdateTags("trk1", updates)
	if err != nil {
		t.Fatalf("UpdateTags failed: %v", err)
	}
	
	// Track should be associated with a new album
	var newAlbumID string
	db.Conn.QueryRow("SELECT album_id FROM tracks WHERE id = 'trk1'").Scan(&newAlbumID)
	
	if newAlbumID == "alb1" {
		t.Error("Track was not assigned to a new album")
	}
	
	var albumTitle, artistName string
	db.Conn.QueryRow("SELECT al.title, a.name FROM albums al JOIN artists a ON al.artist_id = a.id WHERE al.id = ?", newAlbumID).Scan(&albumTitle, &artistName)
	
	if albumTitle != "Brand New Album" || artistName != "Brand New Artist" {
		t.Errorf("New album/artist not created correctly. Got %s by %s", albumTitle, artistName)
	}
}

func TestUpdateTags_FileWriteFailure(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "song.mp3")
	createTestMP3(t, testPath)

	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('art1', 'Old Artist')")
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('alb1', 'art1', 'Old Album', 2000)")
	db.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, format) VALUES ('trk1', 'alb1', 'Old Title', ?, 'mp3')", testPath)

	// Hacer que el archivo falle quitando permisos al directorio (bloquea el .tmp)
	os.Chmod(tempDir, 0555)
	defer os.Chmod(tempDir, 0755)

	editor := NewEditor(db)
	
	updates := map[string]interface{}{
		"title": "Hacked Title",
	}
	
	_, err := editor.UpdateTags("trk1", updates)
	if err == nil {
		t.Fatal("Expected UpdateTags to fail due to file write failure, but it succeeded")
	}
	
	// Verificar que el DB NO se haya actualizado (Rollback lógico de la intención)
	var dbTitle string
	db.Conn.QueryRow("SELECT title FROM tracks WHERE id = 'trk1'").Scan(&dbTitle)
	if dbTitle == "Hacked Title" {
		t.Error("Database was updated despite physical file write failure")
	}
}

func TestUpdateTags_InvalidTrack(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()
	editor := NewEditor(db)
	
	_, err := editor.UpdateTags("fake_track", map[string]interface{}{"title": "A"})
	if err == nil || !strings.Contains(err.Error(), "could not find track id") {
		t.Errorf("Expected 'could not find track id' error, got %v", err)
	}
}

func TestUpdateTags_MaliciousInputTypes(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "song.mp3")
	createTestMP3(t, testPath)

	db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('art1', 'Old Artist')")
	db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('alb1', 'art1', 'Old Album', 2000)")
	db.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, path, format) VALUES ('trk1', 'alb1', 'Old Title', 1, ?, 'mp3')", testPath)

	editor := NewEditor(db)
	
	// Inject unexpected types
	updates := map[string]interface{}{
		"title":    nil,             // expected string
		"year":     "not a number",  // expected int or float64
		"trackNum": map[string]int{"a":1}, // completely unhandled
	}
	
	// Shouldn't panic
	_, err := editor.UpdateTags("trk1", updates)
	if err != nil {
		// Log but don't fail if it simply caught the error naturally, 
		// the point is no panic.
		t.Logf("Handled safely with error: %v", err)
	}
	
	var trackNum int
	db.Conn.QueryRow("SELECT track_num FROM tracks WHERE id = 'trk1'").Scan(&trackNum)
	
	if trackNum != 1 && trackNum != 0 {
		t.Errorf("trackNum changed to unexpected value: %v", trackNum)
	}
}
