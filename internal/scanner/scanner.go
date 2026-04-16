package scanner

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"sgound/internal/db"
	"sgound/internal/tags"

	"github.com/dhowden/tag"
)

type Scanner struct {
	DB         *db.MasterDatabase
	OnProgress func(path string, count int)
}

func NewScanner(database *db.MasterDatabase) *Scanner {
	return &Scanner{DB: database}
}

func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Debug("Starting concurrent library scan", "path", rootPath)

	jobs := make(chan string, 100)
	var wg sync.WaitGroup
	var mu sync.Mutex
	count := 0

	// Pool de 16 workers para procesar archivos en paralelo (el ritmo de "15 en 15")
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if err := s.ParseAndSaveFile(path); err != nil {
					slog.Error("Failed to parse file", "path", path, "error", err)
				}
				mu.Lock()
				count++
				currentCount := count
				mu.Unlock()
				if s.OnProgress != nil {
					s.OnProgress(path, currentCount)
				}
			}
		}()
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return nil }
		if !d.IsDir() && isAudioFile(d.Name()) == nil {
			jobs <- path
		}
		return nil
	})

	close(jobs)
	wg.Wait()

	// Pruning de archivos desaparecidos
	slog.Debug("Pruning library: checking for missing files")
	orphans, err := s.DB.GetOrphanedTracks()
	if err == nil && len(orphans) > 0 {
		var ids []string
		for _, o := range orphans { ids = append(ids, o.ID) }
		s.DB.DeleteTracks(ids)
	}

	slog.Info("Completed library scan", "total", count)
	return err
}

func isAudioFile(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := map[string]bool{".mp3": true, ".flac": true, ".ogg": true, ".m4a": true, ".mp4": true, ".opus": true, ".wav": true}
	if !supported[ext] { return fmt.Errorf("unsupported") }
	return nil
}

func (s *Scanner) ParseAndSaveFile(path string) error {
	fileStat, err := os.Stat(path)
	if err != nil { return err }
	fileSize := fileStat.Size()
	fileMtime := fileStat.ModTime().Unix()
	fileExt := strings.TrimPrefix(filepath.Ext(path), ".")

	// ABRIR EL ARCHIVO SOLO UNA VEZ
	f, err := os.Open(path)
	if err != nil { return err }
	defer f.Close()

	// FAST HASH (1MB)
	hash := ""
	buf := make([]byte, 1024*1024)
	n, _ := f.ReadAt(buf, 0)
	if n > 0 {
		h := sha1.New()
		h.Write(buf[:n])
		hash = hex.EncodeToString(h.Sum(nil))
	}

	metadata, err := tag.ReadFrom(f)
	
	// DECLARACIÓN DE VARIABLES (Scope raíz de la función)
	var artistName, albumArtist, albumName, title, albumId string
	var trackNum, year, discNum, duration int
	var genre, composer, comment string
	var replayGain float64
	var instrumental bool

	if err != nil {
		// FALLBACK: Si falla el parser, usamos la ruta
		artistName = "Unknown Artist"
		albumArtist = "Unknown Artist"
		albumName = filepath.Base(filepath.Dir(path))
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		genre = "Unknown"
		albumId = GenerateID(filepath.Dir(path) + albumName)
	} else {
		artistName = metadata.Artist()
		if artistName == "" { artistName = "Unknown Artist" }
		
		albumArtist = artistName 
		raw := metadata.Raw()
		if val, ok := raw["albumartist"].(string); ok && val != "" {
			albumArtist = val
		} else if val, ok := raw["TPE2"].(string); ok && val != "" { 
			albumArtist = val
		} else if val, ok := raw["ALBUMARTIST"].(string); ok && val != "" {
			albumArtist = val
		}

		albumName = metadata.Album()
		if albumName == "" { albumName = "Unknown Album" }
		title = metadata.Title()
		if title == "" { title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) }
		trackNum, _ = metadata.Track()
		discNum, _ = metadata.Disc()
		year = metadata.Year()
		genre = metadata.Genre()
		if genre == "" { genre = "Unknown" }
		composer = metadata.Composer()
		comment = metadata.Comment()

		// EXTRACCIÓN DE REPLAYGAIN DESDE TAGS (Opcional, FFprobe es más fiable)
		for k, v := range raw {
			key := strings.ToUpper(k)
			if strings.Contains(key, "REPLAYGAIN_TRACK_GAIN") {
				if val, ok := v.(string); ok {
					cleanVal := strings.TrimSuffix(strings.ToLower(val), " db")
					fmt.Sscanf(cleanVal, "%f", &replayGain)
					break
				}
			}
		}

		albumGroupingKey := NormalizeArtist(albumArtist)
		albumId = GenerateID(albumGroupingKey + albumName)
	}

	var bitRate, sampleRate int
	// EXTRACCIÓN DE DURACIÓN Y REPLAYGAIN CON FFPROBE (Prioridad sobre tags básicos)
	if data, err := getAudioMetadataWithFFprobe(path); err == nil {
		duration = data.duration
		bitRate = data.bitRate
		sampleRate = data.sampleRate
		if data.replayGain != 0 {
			replayGain = data.replayGain
		}
	}

	if sampleRate == 0 { sampleRate = 44100 }

	trackId := tags.ReadPersistentID(path)
	if trackId == "" {
		trackId = GenerateID(path)
	} else {
		var dbMtime int64
		var dbSize int64
		var dbPath string
		var dbBitrate int
		err = s.DB.Conn.QueryRow("SELECT file_mtime, size, path, bit_rate FROM tracks WHERE id = ?", trackId).Scan(&dbMtime, &dbSize, &dbPath, &dbBitrate)
		if err == nil && dbBitrate > 0 && dbMtime == fileMtime && dbSize == fileSize && dbPath == path {
			return nil 
		}
	}

	artistId := GenerateID(albumArtist)
	s.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistId, albumArtist)

	if albumId == "" {
		albumGroupingKey := albumArtist
		if strings.Contains(albumArtist, "Unknown") {
			albumGroupingKey = filepath.Dir(path)
		}
		albumId = GenerateID(albumGroupingKey + albumName)
	}
	s.DB.Conn.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumId, artistId, albumName, year)

	status := "pending"
	var existingStatus string
	err = s.DB.Conn.QueryRow("SELECT status FROM tracks WHERE id = ?", trackId).Scan(&existingStatus)
	if err == nil { status = existingStatus }

	// Evitar duplicados por ruta si ha cambiado el ID (ej: de basado en ruta a PersistentID)
	var existingId string
	var existingIdStatus string
	err = s.DB.Conn.QueryRow("SELECT id, status FROM tracks WHERE path = ?", path).Scan(&existingId, &existingIdStatus)
	if err == nil && existingId != trackId {
		slog.Warn("Path collision detected: migrating track to new ID", "path", path, "old", existingId, "new", trackId)
		// Preservar el estado (muy importante para evitar que vuelvan al Inbox)
		status = existingIdStatus
		// Migrar datos de usuario e historiales
		s.DB.Conn.Exec("UPDATE OR IGNORE user_tracks SET track_id = ? WHERE track_id = ?", trackId, existingId)
		s.DB.Conn.Exec("UPDATE OR IGNORE playlist_tracks SET track_id = ? WHERE track_id = ?", trackId, existingId)
		s.DB.Conn.Exec("UPDATE OR IGNORE playback_history SET track_id = ? WHERE track_id = ?", trackId, existingId)
		// Eliminar el antiguo para evitar duplicados en la vista de biblioteca
		s.DB.Conn.Exec("DELETE FROM tracks WHERE id = ?", existingId)
	}

	_, err = s.DB.Conn.Exec(
		`INSERT OR REPLACE INTO tracks 
		(id, album_id, artist, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, instrumental, replay_gain, file_mtime, file_hash) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackId, albumId, artistName, title, trackNum, discNum, path, duration, genre, fileSize, fileExt, composer, comment, bitRate, sampleRate, status, instrumental, replayGain, fileMtime, hash,
	)
	return err
}

type audioMetadata struct {
	duration   int
	bitRate    int
	sampleRate int
	replayGain float64
}

func getAudioMetadataWithFFprobe(path string) (audioMetadata, error) {
	// Usamos nokey=0 para identificar cada campo correctamente
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration,bit_rate:stream=sample_rate:format_tags=REPLAYGAIN_TRACK_GAIN,replaygain_track_gain", "-of", "default=noprint_wrappers=1:nokey=0", path)
	out, err := cmd.Output()
	if err != nil {
		return audioMetadata{}, err
	}
	
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	data := audioMetadata{sampleRate: 44100} // Default sensible
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "duration=") {
			if val, err := strconv.ParseFloat(strings.TrimPrefix(line, "duration="), 64); err == nil {
				data.duration = int(val)
			}
		} else if strings.HasPrefix(line, "bit_rate=") {
			if val, err := strconv.Atoi(strings.TrimPrefix(line, "bit_rate=")); err == nil {
				data.bitRate = val
			}
		} else if strings.HasPrefix(line, "sample_rate=") {
			if val, err := strconv.Atoi(strings.TrimPrefix(line, "sample_rate=")); err == nil {
				data.sampleRate = val
			}
		} else if strings.Contains(line, "replaygain_track_gain=") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				valStr := strings.TrimSuffix(strings.ToLower(parts[1]), " db")
				if val, err := strconv.ParseFloat(valStr, 64); err == nil {
					data.replayGain = val
				}
			}
		} else if strings.HasSuffix(line, " dB") {
			// Fallback para el formato anterior si FFprobe lo saca sin key en los tags
			cleanVal := strings.TrimSuffix(strings.ToLower(line), " db")
			if val, err := strconv.ParseFloat(cleanVal, 64); err == nil {
				data.replayGain = val
			}
		}
	}
	
	return data, nil
}

func GenerateID(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func NormalizeArtist(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	// Eliminar ft., feat., with, etc para agrupar bajo el artista principal
	separators := []string{" feat.", " ft.", " feat ", " with ", " & ", " / ", " vs. "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 {
			name = name[:idx]
		}
	}
	return strings.TrimSpace(name)
}
