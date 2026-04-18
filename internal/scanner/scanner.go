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
	"time"

	"sgound/internal/db"
	"sgound/internal/tags"

	"github.com/dhowden/tag"
)

type Scanner struct {
	DB         *db.MasterDatabase
	OnProgress func(path string, count int)

	// Caché para optimización de agrupación de álbumes
	muAlbums         sync.Mutex
	resolvedAlbums   map[string]string   // albumId -> artista principal
	conflictedAlbums map[string]struct{} // albumId -> requiere votación

	// Caché para optimización de inserciones SQL
	seenArtists map[string]struct{}
	seenAlbums  map[string]struct{}
	
	// Fast Path Cache: path -> {mtime, size}
	dbFileCache map[string]dbFileInfo
}

type dbFileInfo struct {
	mtime int64
	size  int64
}

func NewScanner(database *db.MasterDatabase) *Scanner {
	return &Scanner{
		DB:               database,
		resolvedAlbums:   make(map[string]string),
		conflictedAlbums: make(map[string]struct{}),
		seenArtists:      make(map[string]struct{}),
		seenAlbums:       make(map[string]struct{}),
		dbFileCache:      make(map[string]dbFileInfo),
	}
}

func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Info("Starting Optimized Turbo library scan", "path", rootPath)
	start := time.Now()

	s.muAlbums.Lock()
	s.resolvedAlbums = make(map[string]string)
	s.conflictedAlbums = make(map[string]struct{})
	s.seenArtists = make(map[string]struct{})
	s.seenAlbums = make(map[string]struct{})
	s.muAlbums.Unlock()

	// 1. Pre-cargar metadatos de la DB en RAM (FAST PATH OPTIMIZATION)
	slog.Info("Indexing existing library metadata...")
	rows, err := s.DB.Conn.Query("SELECT path, file_mtime, size FROM tracks")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			var m, sz int64
			if err := rows.Scan(&p, &m, &sz); err == nil {
				s.dbFileCache[p] = dbFileInfo{mtime: m, size: sz}
			}
		}
	}

	// Optimizar para discos con latencia (HDD). 2 workers suele ser el sweet spot.
	numWorkers := 2 
	if numWorkers > 16 { numWorkers = 16 } // Cap para no saturar I/O de disco
	
	jobs := make(chan string, 500)
	results := make(chan *trackResult, 500)
	var wg sync.WaitGroup
	var mu sync.Mutex
	count := 0

	// DB WRITER (Batch Transactions)
	writerDone := make(chan bool)
	go func() {
		batchSize := 50
		var batch []*trackResult
		
		flush := func() {
			if len(batch) == 0 { return }
			slog.Info("Flushing scan batch to database", "count", len(batch))
			tx, err := s.DB.Conn.Begin()
			if err != nil {
				slog.Error("Failed to start transaction", "error", err)
				return
			}
			
			for _, r := range batch {
				if _, seen := s.seenArtists[r.artistId]; !seen {
					tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", r.artistId, r.dbAlbumArtist)
					s.seenArtists[r.artistId] = struct{}{}
				}
				if _, seen := s.seenAlbums[r.albumId]; !seen {
					tx.Exec(`
						INSERT INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)
						ON CONFLICT(id) DO UPDATE SET 
							year = CASE WHEN (year IS NULL OR year = 0) THEN excluded.year ELSE year END
					`, r.albumId, r.artistId, r.albumName, r.year)
					s.seenAlbums[r.albumId] = struct{}{}
				}
				tx.Exec(
					`INSERT OR REPLACE INTO tracks 
					(id, album_id, artist, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, instrumental, replay_gain, file_mtime, file_hash) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					r.trackId, r.albumId, r.artistName, r.title, r.trackNum, r.discNum, r.path, r.duration, r.genre, r.size, r.format, r.composer, r.comment, r.bitRate, r.sampleRate, r.status, r.instrumental, r.replayGain, r.fileMtime, r.hash,
				)

				s.muAlbums.Lock()
				resolved, exists := s.resolvedAlbums[r.albumId]
				if !exists {
					s.resolvedAlbums[r.albumId] = r.dbAlbumArtist
				} else if resolved != r.dbAlbumArtist {
					s.conflictedAlbums[r.albumId] = struct{}{}
				}
				s.muAlbums.Unlock()
			}
			tx.Commit()
			batch = nil
		}

		for r := range results {
			batch = append(batch, r)
			if len(batch) >= batchSize { flush() }
		}
		flush()
		writerDone <- true
	}()

	// Workers Dinámicos
	slog.Info("Starting workers", "count", numWorkers)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				// FAST PATH: Check RAM cache instead of DB
				fileStat, err := os.Stat(path)
				if err != nil { continue }
				
				info, exists := s.dbFileCache[path]
				if exists && info.mtime == fileStat.ModTime().Unix() && info.size == fileStat.Size() {
					mu.Lock()
					count++
					mu.Unlock()
					continue // Skip file, no changes
				}

				slog.Info("Scanning file", "path", path)
				res, err := s.ParseFile(path)
				if err != nil {
					slog.Error("Failed to process file", "path", path, "error", err)
					continue
				}
				if res != nil { results <- res }
				mu.Lock()
				count++
				currentCount := count
				mu.Unlock()
				if s.OnProgress != nil { s.OnProgress(path, currentCount) }
			}
		}()
	}

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return nil }

		// Skip hidden folders and files (like .Trash-1000, .git, etc)
		name := d.Name()
		if name != "." && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() && isAudioFile(name) == nil { jobs <- path }
		return nil
	})

	close(jobs)
	wg.Wait()
	close(results)
	<-writerDone

	// Votación de artistas de álbum
	s.muAlbums.Lock()
	for id := range s.conflictedAlbums {
		s.recalculateAlbumArtist(id)
	}
	s.muAlbums.Unlock()

	// Pruning (Limpieza de tracks eliminados físicamente)
	orphans, err := s.DB.GetOrphanedTracks()
	if err == nil && len(orphans) > 0 {
		var ids []string
		for _, o := range orphans { ids = append(ids, o.ID) }
		s.DB.DeleteTracks(ids)
		slog.Info("Pruned orphaned tracks from DB", "count", len(ids))
	}

	slog.Info("Completed library scan", "total", count, "duration", time.Since(start))
	return err
}

func (s *Scanner) ParseAndSaveFile(path string) error {
	res, err := s.ParseFile(path)
	if err != nil { return err }
	if res == nil { return nil }

	tx, err := s.DB.Conn.Begin()
	if err != nil { return err }

	tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", res.artistId, res.dbAlbumArtist)
	tx.Exec(`
		INSERT INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			year = CASE WHEN (year IS NULL OR year = 0) THEN excluded.year ELSE year END
	`, res.albumId, res.artistId, res.albumName, res.year)

	_, err = tx.Exec(
		`INSERT OR REPLACE INTO tracks 
		(id, album_id, artist, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, instrumental, replay_gain, file_mtime, file_hash) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res.trackId, res.albumId, res.artistName, res.title, res.trackNum, res.discNum, res.path, res.duration, res.genre, res.size, res.format, res.composer, res.comment, res.bitRate, res.sampleRate, res.status, res.instrumental, res.replayGain, res.fileMtime, res.hash,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Scanner) ParseFile(path string) (*trackResult, error) {
	fileStat, err := os.Stat(path)
	if err != nil { return nil, err }
	fileSize := fileStat.Size()
	fileMtime := fileStat.ModTime().Unix()
	fileExt := strings.TrimPrefix(filepath.Ext(path), ".")

	// ID Persistente
	trackId := tags.ReadPersistentID(path)
	if trackId == "" { 
		// Check DB if we have it by path to recover ID
		s.DB.Conn.QueryRow("SELECT id FROM tracks WHERE path = ?", path).Scan(&trackId)
		if trackId == "" {
			trackId = GenerateID(path) 
		}
	}

	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()

	// Metadatos
	metadata, err := tag.ReadFrom(f)
	var artistName, albumName, title string
	var trackNum, year, discNum, bitRate, sampleRate int
	var duration float64
	var genre, composer, comment string
	var replayGain float64
	var instrumental bool

	if err != nil {
		artistName = "Unknown Artist"
		albumName = "Unknown Album"
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

	// Optimización: Intentar sacar duración de FLAC nativamente (SIN ffprobe)
	if strings.ToLower(filepath.Ext(path)) == ".flac" {
		if d, br, sr, err := getFLACDurationNative(path); err == nil && d > 0 {
			duration = d
			bitRate = br
			sampleRate = sr
		}
	}

	// ffprobe solo si es necesario (sin duración)
	if duration == 0 {
		slog.Info("File has no duration tag, spawning ffprobe", "path", path)
		if data, err := getAudioMetadataWithFFprobe(path); err == nil {
			duration = data.duration
			bitRate = data.bitRate
			sampleRate = data.sampleRate
			if data.replayGain != 0 { replayGain = data.replayGain }
		}
	}
	if sampleRate == 0 { sampleRate = 44100 }

	albumArtistTag := ""
	if metadata != nil {
		raw := metadata.Raw()
		for _, tagKey := range []string{"albumartist", "ALBUMARTIST", "TPE2"} {
			if val, ok := raw[tagKey].(string); ok && val != "" {
				albumArtistTag = val
				break
			}
		}
	}

	var groupingKey string
	if albumArtistTag != "" {
		groupingKey = "AA:" + NormalizeArtist(albumArtistTag)
	} else {
		dir := filepath.Dir(path)
		base := strings.ToLower(filepath.Base(dir))
		if strings.HasPrefix(base, "cd") || strings.HasPrefix(base, "disc") || strings.HasPrefix(base, "disk") {
			dir = filepath.Dir(dir)
		}
		groupingKey = "DIR:" + dir
	}

	albumId := GenerateID(groupingKey + "|" + NormalizeString(albumName))
	dbAlbumArtist := albumArtistTag
	if dbAlbumArtist == "" { dbAlbumArtist = artistName }
	artistId := GenerateID(dbAlbumArtist)

	status := "pending"
	// Check if track was already active in DB to keep status
	s.DB.Conn.QueryRow("SELECT status FROM tracks WHERE id = ?", trackId).Scan(&status)

	return &trackResult{
		trackId: trackId, albumId: albumId, artistId: artistId, 
		artistName: artistName, dbAlbumArtist: dbAlbumArtist, albumName: albumName, 
		title: title, path: path, genre: genre, format: fileExt, 
		composer: composer, comment: comment, hash: "", status: status,
		trackNum: trackNum, discNum: discNum, year: year, duration: duration, 
		bitRate: bitRate, sampleRate: sampleRate, size: fileSize, 
		fileMtime: fileMtime, replayGain: replayGain, instrumental: instrumental,
	}, nil
}

type trackResult struct {
	trackId, albumId, artistId, artistName, dbAlbumArtist, albumName, title, path, genre, format, composer, comment, hash, status string
	trackNum, discNum, year, bitRate, sampleRate int
	duration, replayGain float64
	size, fileMtime int64
	instrumental bool
}

func isAudioFile(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := map[string]bool{".mp3": true, ".flac": true, ".ogg": true, ".m4a": true, ".mp4": true, ".opus": true, ".wav": true}
	if !supported[ext] { return fmt.Errorf("unsupported") }
	return nil
}

func getAudioMetadataWithFFprobe(path string) (audioMetadata, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration,bit_rate:stream=sample_rate:format_tags=REPLAYGAIN_TRACK_GAIN,replaygain_track_gain", "-of", "default=noprint_wrappers=1:nokey=0", path)
	out, err := cmd.Output()
	if err != nil { return audioMetadata{}, err }
	
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	data := audioMetadata{sampleRate: 44100}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "duration=") {
			if val, err := strconv.ParseFloat(strings.TrimPrefix(line, "duration="), 64); err == nil {
				data.duration = val
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
		}
	}
	return data, nil
}

type audioMetadata struct {
	duration   float64
	bitRate    int
	sampleRate int
	replayGain float64
}

func (s *Scanner) recalculateAlbumArtist(albumId string) {
	rows, err := s.DB.Conn.Query(`
		SELECT artist, COUNT(*) as freq 
		FROM tracks 
		WHERE album_id = ? AND artist != 'Unknown Artist'
		GROUP BY artist 
		ORDER BY freq DESC, artist ASC
	`, albumId)
	if err != nil { return }
	defer rows.Close()

	var winnerName string
	var maxFreq int
	if rows.Next() { rows.Scan(&winnerName, &maxFreq) }
	if winnerName == "" { return }

	winnerArtistId := GenerateID(winnerName)
	s.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", winnerArtistId, winnerName)
	s.DB.Conn.Exec("UPDATE albums SET artist_id = ? WHERE id = ?", winnerArtistId, albumId)
}

func GenerateID(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func NormalizeArtist(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	separators := []string{" feat.", " ft.", " feat ", " ft ", " with "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 { name = name[:idx] }
	}
	return strings.TrimSpace(name)
}

func NormalizeString(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer(".", "", ",", "", "-", "", "(", "", ")", "", "[", "", "]", "", "'", "", "\"", "")
	s = r.Replace(s)
	return strings.TrimSpace(s)
}

// Extraer info de FLAC usando STREAMINFO (muy rápido)
func getFLACDurationNative(path string) (float64, int, int, error) {
	f, err := os.Open(path)
	if err != nil { return 0, 0, 0, err }
	defer f.Close()

	// Leer encabezado "fLaC"
	head := make([]byte, 4)
	if _, err := f.Read(head); err != nil || string(head) != "fLaC" {
		return 0, 0, 0, fmt.Errorf("not a flac file")
	}

	// El bloque STREAMINFO es el primero y tiene un tamaño fijo de 34 bytes (más 4 de cabecera de bloque)
	// Cabecera de bloque: 1 bit (isLast), 7 bits (type), 24 bits (length)
	blockHeader := make([]byte, 4)
	if _, err := f.Read(blockHeader); err != nil { return 0, 0, 0, err }
	
	blockType := blockHeader[0] & 0x7F
	if blockType != 0 { // 0 es STREAMINFO
		return 0, 0, 0, fmt.Errorf("STREAMINFO not first block")
	}

	data := make([]byte, 34)
	if _, err := f.Read(data); err != nil { return 0, 0, 0, err }

	// STREAMINFO structure:
	// min_block_size: 16 bits
	// max_block_size: 16 bits
	// min_frame_size: 24 bits
	// max_frame_size: 24 bits
	// sample_rate: 20 bits
	// num_channels: 3 bits
	// bits_per_sample: 5 bits
	// total_samples: 36 bits
	
	sampleRate := int(data[10])<<12 | int(data[11])<<4 | int(data[12])>>4
	
	totalSamples := int64(data[13]&0x0F)<<32 | int64(data[14])<<24 | int64(data[15])<<16 | int64(data[16])<<8 | int64(data[17])
	
	if sampleRate > 0 {
		duration := float64(totalSamples) / float64(sampleRate)
		
		// Bitrate aproximado: (tamaño archivo * 8) / duración
		info, _ := os.Stat(path)
		br := 0
		if duration > 0 {
			br = int((float64(info.Size()) * 8) / duration)
		}
		
		return duration, br, sampleRate, nil
	}

	return 0, 0, 0, fmt.Errorf("could not parse STREAMINFO")
}
