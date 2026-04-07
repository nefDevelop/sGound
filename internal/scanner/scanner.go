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
	"crypto/sha1"
	"encoding/hex"
)

type Scanner struct {
	DB         *db.Database
	OnProgress func(path string, count int)
}

func NewScanner(database *db.Database) *Scanner {
	return &Scanner{DB: database}
}

// ScanDirectory walks the given directory and processes audio files.
func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Info("Starting library scan", "path", rootPath)

	count := 0
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Error("Error walking path", "path", path, "error", err)
			return nil // Skip this path and continue
		}

		if !d.IsDir() {
			if err := isAudioFile(d.Name()); err == nil {
				slog.Info("Audio file found", "path", path)
				if parseErr := s.ParseAndSaveFile(path); parseErr != nil {
					slog.Error("Failed to parse file", "path", path, "error", parseErr)
				} else {
					count++
					if s.OnProgress != nil {
						s.OnProgress(path, count)
					}
				}
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

func isAudioFile(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := map[string]bool{
		".mp3":  true,
		".flac": true,
		".ogg":  true,
		".m4a":  true,
		".mp4":  true,
		".opus": true,
		".wav":  true,
	}
	if !supported[ext] {
		return fmt.Errorf("unsupported extension: %s", ext)
	}
	return nil
}

func (s *Scanner) ParseAndSaveFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fileStat, _ := f.Stat()
	fileSize := fileStat.Size()
	fileExt := strings.TrimPrefix(filepath.Ext(path), ".")

	metadata, err := tag.ReadFrom(f)
	
	var artistName, albumName, title string
	var trackNum, year, discNum int
	var genre, composer, comment string
	var replayGain float64

	if err != nil {
		slog.Warn("Tag parsing failed (corrupt metadata), fallback to paths", "path", path, "err", err)
		genre = "Unknown"
		artistName = "Unknown Artist"
		baseDir := filepath.Base(filepath.Dir(path))
		if baseDir == "." || baseDir == "/" {
			albumName = "Unknown Album"
		} else {
			albumName = baseDir
		}
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	} else {
		artistName = metadata.Artist()
		if artistName == "" {
			artistName = "Unknown Artist"
		}
		albumName = metadata.Album()
		if albumName == "" {
			albumName = "Unknown Album"
		}
		title = metadata.Title()
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		trackNum, _ = metadata.Track()
		discNum, _ = metadata.Disc()
		year = metadata.Year()
		genre = metadata.Genre()
		if genre == "" {
			genre = "Unknown"
		}
		composer = metadata.Composer()
		comment = metadata.Comment()
		
		// ReplayGain extraction
		if raw := metadata.Raw(); raw != nil {
			if rg, ok := raw["REPLAYGAIN_TRACK_GAIN"].(string); ok {
				fmt.Sscanf(rg, "%f", &replayGain)
			} else if rgArr, ok := raw["REPLAYGAIN_TRACK_GAIN"].([]string); ok && len(rgArr) > 0 {
				fmt.Sscanf(rgArr[0], "%f", &replayGain)
			}
		}
	}

	// 1. Get or Create Artist
	artistId := generateID(artistName)
	_, err = s.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistId, artistName)
	if err != nil {
		return err
	}

	// 2. Get or Create Album
	albumId := generateID(artistName + albumName)
	_, err = s.DB.Conn.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumId, artistId, albumName, year)
	if err != nil {
		return err
	}

	// 3. Create Track
	trackId := generateID(path) // Unique identifier for the file
	
	// Por ahora dejamos bitrate y samplerate en 0 o valores por defecto 
	// hasta que tengamos los decodificadores funcionando al 100% en este env
	bitrate := 0
	samplerate := 44100 
	duration := 0 // En segundos

	// Determinar el status inicial
	status := "active"
	var existingStatus string
	err = s.DB.Conn.QueryRow("SELECT status FROM tracks WHERE id = ?", trackId).Scan(&existingStatus)
	if err != nil {
		// Si no existe, es nueva -> "pending"
		status = "pending"
	} else {
		// Si ya existe, preservamos su status
		status = existingStatus
	}

	rating := 0 // Por ahora 0, luego implementaremos lectura de RATING tag

	_, err = s.DB.Conn.Exec(
		`INSERT OR REPLACE INTO tracks 
		(id, album_id, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, rating, replay_gain) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackId, albumId, title, trackNum, discNum, path, duration, genre, fileSize, fileExt, composer, comment, bitrate, samplerate, status, rating, replayGain,
	)
	if err != nil {
		return err
	}

	return nil
}

// naive id generator, in real app consider uuid or hashing
func generateID(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
