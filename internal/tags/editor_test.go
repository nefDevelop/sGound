package tags

import (
	"os"
	"path/filepath"
	"sgound/internal/db"
	"testing"
	"time"
)

func setupTestDB() *db.Database {
	database, _ := db.Connect(":memory:")
	database.Conn.Exec("CREATE TABLE IF NOT EXISTS artists (id TEXT PRIMARY KEY, name TEXT NOT NULL)")
	database.Conn.Exec("CREATE TABLE IF NOT EXISTS albums (id TEXT PRIMARY KEY, artist_id TEXT NOT NULL, title TEXT NOT NULL, year INTEGER)")
	database.Conn.Exec(`CREATE TABLE IF NOT EXISTS tracks (
		id TEXT PRIMARY KEY, album_id TEXT NOT NULL, title TEXT NOT NULL, 
		track_num INTEGER, disc_num INTEGER, path TEXT NOT NULL, 
		format TEXT, genre TEXT, bit_rate INTEGER, file_mtime INTEGER
	)`)
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

	// Esperar un momento a que la goroutine de limpieza actúe
	time.Sleep(100 * time.Millisecond)

	// Verificar que old_folder/sub ha desaparecido
	if _, err := os.Stat(oldFolder); !os.IsNotExist(err) {
		t.Error("Old folder should have been deleted")
	}
	
	parentOfOld := filepath.Join(tempDir, "old_folder")
	if _, err := os.Stat(parentOfOld); !os.IsNotExist(err) {
		t.Error("Old parent folder should have been deleted")
	}
}
