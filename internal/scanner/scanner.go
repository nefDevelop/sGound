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

	// Caché para optimización de agrupación de álbumes
	muAlbums         sync.Mutex
	resolvedAlbums   map[string]string   // albumId -> artista principal
	conflictedAlbums map[string]struct{} // albumId -> requiere votación

	// Caché para optimización de inserciones SQL
	seenArtists map[string]struct{}
	seenAlbums  map[string]struct{}
}

func NewScanner(database *db.MasterDatabase) *Scanner {
	return &Scanner{
		DB:               database,
		resolvedAlbums:   make(map[string]string),
		conflictedAlbums: make(map[string]struct{}),
		seenArtists:      make(map[string]struct{}),
		seenAlbums:       make(map[string]struct{}),
	}
}

func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Info("Starting Turbo library scan", "path", rootPath)

	s.muAlbums.Lock()
	s.resolvedAlbums = make(map[string]string)
	s.conflictedAlbums = make(map[string]struct{})
	s.seenArtists = make(map[string]struct{})
	s.seenAlbums = make(map[string]struct{})
	s.muAlbums.Unlock()

	jobs := make(chan string, 200)
	results := make(chan *trackResult, 200)
	var wg sync.WaitGroup
	var mu sync.Mutex
	count := 0

	// DB WRITER (Batch Transactions para máxima velocidad)
	writerDone := make(chan bool)
	go func() {
		batchSize := 200
		var batch []*trackResult
		
		flush := func() {
			if len(batch) == 0 { return }
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

	// 16 Workers (Como el código que pasaste)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
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

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return nil }
		if !d.IsDir() && isAudioFile(d.Name()) == nil { jobs <- path }
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

	// Pruning
	orphans, err := s.DB.GetOrphanedTracks()
	if err == nil && len(orphans) > 0 {
		var ids []string
		for _, o := range orphans { ids = append(ids, o.ID) }
		s.DB.DeleteTracks(ids)
	}

	slog.Info("Completed library scan", "total", count)
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

	// FAST PATH: Si ya está en la DB y no cambió, ni siquiera abrimos el archivo
	var trackId, currentAlbumId, status string
	var dbMtime, dbSize int64
	err = s.DB.Conn.QueryRow("SELECT id, file_mtime, size, album_id, status FROM tracks WHERE path = ?", path).
		Scan(&trackId, &dbMtime, &dbSize, &currentAlbumId, &status)
	
	if err == nil && dbMtime == fileMtime && dbSize == fileSize {
		return nil, nil 
	}

	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()

	// Metadatos
	metadata, err := tag.ReadFrom(f)
	var artistName, albumName, title string
	var trackNum, year, discNum, duration int
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

	// ffprobe solo si es necesario (sin duración)
	var bitRate, sampleRate int
	if duration == 0 {
		if data, err := getAudioMetadataWithFFprobe(path); err == nil {
			duration = data.duration
			bitRate = data.bitRate
			sampleRate = data.sampleRate
			if data.replayGain != 0 { replayGain = data.replayGain }
		}
	}
	if sampleRate == 0 { sampleRate = 44100 }

	// ID Persistente
	if trackId == "" {
		trackId = tags.ReadPersistentID(path)
		if trackId == "" { trackId = GenerateID(path) }
	}

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

	if status == "" { status = "pending" }

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
	trackNum, discNum, year, duration, bitRate, sampleRate int
	size, fileMtime int64
	replayGain float64
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
		}
	}
	return data, nil
}

type audioMetadata struct {
	duration, bitRate, sampleRate int
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
