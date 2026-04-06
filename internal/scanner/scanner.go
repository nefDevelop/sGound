package scanner

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"sgound/internal/db"

	"github.com/dhowden/tag"
)

type Scanner struct {
	DB *db.Database
}

func NewScanner(database *db.Database) *Scanner {
	return &Scanner{DB: database}
}

// ScanDirectory walks the given directory and processes audio files.
func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Info("Starting library scan", "path", rootPath)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Error("Error walking path", "path", path, "error", err)
			return nil // Skip this path and continue
		}

		if !d.IsDir() && isAudioFile(d.Name()) {
			if parseErr := s.ParseAndSaveFile(path); parseErr != nil {
				slog.Error("Failed to parse file", "path", path, "error", parseErr)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	slog.Info("Completed library scan")
	return nil
}

func isAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".mp3" || ext == ".flac" || ext == ".ogg" || ext == ".m4a"
}

func (s *Scanner) ParseAndSaveFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	metadata, err := tag.ReadFrom(f)
	if err != nil {
		return fmt.Errorf("failed to read tags from %s: %w", path, err)
	}

	artistName := metadata.Artist()
	if artistName == "" {
		artistName = "Unknown Artist"
	}
	albumName := metadata.Album()
	if albumName == "" {
		albumName = "Unknown Album"
	}
	title := metadata.Title()
	if title == "" {
		title = filepath.Base(path)
	}
	trackNum, _ := metadata.Track()

	// 1. Get or Create Artist
	artistId := generateID(artistName)
	_, err = s.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistId, artistName)
	if err != nil {
		return err
	}

	// 2. Get or Create Album
	albumId := generateID(artistName + albumName)
	_, err = s.DB.Conn.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumId, artistId, albumName, metadata.Year())
	if err != nil {
		return err
	}

	// 3. Create Track
	trackId := generateID(path) // Unique identifier for the file
	_, err = s.DB.Conn.Exec(
		"INSERT OR REPLACE INTO tracks (id, album_id, title, track_num, path, duration) VALUES (?, ?, ?, ?, ?, ?)",
		trackId, albumId, title, trackNum, path, 0, // 0 for duration until we parse headers deeper
	)
	if err != nil {
		return err
	}

	return nil
}

// naive id generator, in real app consider uuid or hashing
func generateID(input string) string {
	return strings.ReplaceAll(strings.ToLower(input), " ", "-") // VERY naive, for POC
}
