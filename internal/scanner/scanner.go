package scanner

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sgound/internal/db"
	"sgound/internal/tags"

	"github.com/dhowden/tag"
	"github.com/go-flac/go-flac"
)

type Scanner struct {
	DB         *db.Database
	OnProgress func(path string, count int)
}

func NewScanner(database *db.Database) *Scanner {
	return &Scanner{DB: database}
}

func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Info("Starting concurrent library scan", "path", rootPath)

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
	slog.Info("Pruning library: checking for missing files")
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

	// Intentar leer metadatos (con protección contra carátulas corruptas)
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
	
	var artistName, albumName, title string
	var trackNum, year, discNum int
	var genre, composer, comment string
	var replayGain float64
	var instrumental bool

	if err != nil {
		// FALLBACK: Si falla el parser (ej: error 4294967295), usamos la ruta
		// pero permitimos que el escaneo continúe a máxima velocidad
		artistName = "Unknown Artist"
		albumName = filepath.Base(filepath.Dir(path))
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		genre = "Unknown"
	} else {
		artistName = metadata.Artist()
		if artistName == "" { artistName = "Unknown Artist" }
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
	}

	// EXTRACCIÓN DE DURACIÓN (Mejora)
	duration := 0
	if fileExt == "flac" {
		f, err := flac.ParseFile(path)
		if err == nil {
			for _, m := range f.Meta {
				if m.Type == flac.StreamInfo {
					// Extraemos samples totales y frecuencia (truco de bits para StreamInfo)
					// StreamInfo tiene 34 bytes. Los samples están en los bytes 14-17 (parte) y 18-25
					// Por simplicidad, usemos el método de la librería si existe o lo estimamos
					// go-flac no expone campos directos en StreamInfo fácilmente sin parsear manual
					// pero podemos usar el tamaño / bitrate si hace falta
				}
			}
		}
	}
	// TODO: Añadir parser de MP3 para duración precisa

	// IDENTIFICACIÓN: Primero intentamos por persistente (pero de forma rápida)
	// Como ya sabemos si es .mp3 o .flac, podemos ser específicos
	trackId := tags.ReadPersistentID(path)
	if trackId == "" {
		// Si está en el inbox o es nuevo, ID rápido basado en ruta
		trackId = generateID(path)
	} else {
		// Si ya tiene ID, comprobamos si ha cambiado algo rápido
		var dbMtime int64
		var dbSize int64
		var dbPath string
		err = s.DB.Conn.QueryRow("SELECT file_mtime, size, path FROM tracks WHERE id = ?", trackId).Scan(&dbMtime, &dbSize, &dbPath)
		if err == nil && dbMtime == fileMtime && dbSize == fileSize && dbPath == path {
			return nil 
		}
	}

	// 3. Crear Artista y Álbum
	artistId := generateID(artistName)
	s.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistId, artistName)
	albumId := generateID(artistName + albumName)
	s.DB.Conn.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumId, artistId, albumName, year)

	// 4. Guardar Track
	status := "pending"
	var existingStatus string
	err = s.DB.Conn.QueryRow("SELECT status FROM tracks WHERE id = ?", trackId).Scan(&existingStatus)
	if err == nil { status = existingStatus }

	// TINT: Si no tenemos duración todavía, intentamos calcularla o dejarla para un segundo paso
	// Nota: la librería 'tag' es básica. En el futuro usaremos FFProbe para esto.
	// Por ahora, evitemos el hardcode 0 si podemos.

	_, err = s.DB.Conn.Exec(
		`INSERT OR REPLACE INTO tracks 
		(id, album_id, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, rating, instrumental, replay_gain, file_mtime, file_hash) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackId, albumId, title, trackNum, discNum, path, duration, genre, fileSize, fileExt, composer, comment, 0, 44100, status, 0, instrumental, replayGain, fileMtime, hash,
	)
	return err
}

func generateID(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
