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

	// Step 1: Walk and update/add
	count := 0
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Error("Error walking path", "path", path, "error", err)
			return nil // Skip this path and continue
		}

		if !d.IsDir() {
			if err := isAudioFile(d.Name()); err == nil {
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

	// Step 2: Prune missing files
	slog.Info("Pruning library: checking for missing files")
	orphans, err := s.DB.GetOrphanedTracks()
	if err == nil && len(orphans) > 0 {
		var ids []string
		for _, o := range orphans {
			ids = append(ids, o.ID)
		}
		if deleteErr := s.DB.DeleteTracks(ids); deleteErr != nil {
			slog.Error("Failed to delete orphaned tracks", "error", deleteErr)
		} else {
			slog.Info("Pruned missing tracks", "count", len(ids))
		}
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
	fileStat, err := os.Stat(path)
	if err != nil {
		return err
	}
	fileSize := fileStat.Size()
	fileMtime := fileStat.ModTime().Unix()

	trackId := generateID(path)
	
	// Check if already in DB and unchanged
	var dbMtime int64
	var dbSize int64
	err = s.DB.Conn.QueryRow("SELECT file_mtime, size FROM tracks WHERE id = ?", trackId).Scan(&dbMtime, &dbSize)
	if err == nil && dbMtime == fileMtime && dbSize == fileSize {
		// slog.Debug("File unchanged, skipping", "path", path)
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fileExt := strings.TrimPrefix(filepath.Ext(path), ".")

	metadata, err := tag.ReadFrom(f)
	
	var artistName, albumName, title string
	var trackNum, year, discNum int
	var genre, composer, comment string
	var replayGain float64
	var instrumental bool

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
		
		// Metadata extraction (Instrumental & ReplayGain)
		if raw := metadata.Raw(); raw != nil {
			// Instrumental detection
			for _, key := range []string{"INSTRUMENTAL", "instrumental"} {
				if val, ok := raw[key].(string); ok && (val == "1" || strings.ToLower(val) == "true") {
					instrumental = true
				} else if valArr, ok := raw[key].([]string); ok && len(valArr) > 0 && (valArr[0] == "1" || strings.ToLower(valArr[0]) == "true") {
					instrumental = true
				}
			}

			// ReplayGain
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
		(id, album_id, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, rating, instrumental, replay_gain, file_mtime) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackId, albumId, title, trackNum, discNum, path, duration, genre, fileSize, fileExt, composer, comment, bitrate, samplerate, status, rating, instrumental, replayGain, fileMtime,
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
