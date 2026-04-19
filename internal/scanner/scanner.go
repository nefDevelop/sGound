package scanner

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
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

	// Canales de prioridad
	PriorityJobs chan string

	// Mutexes para sincronización
	muDB             sync.Mutex          // Bloqueo de escritura en DB
	muAlbums         sync.Mutex          // Bloqueo de caché de álbumes
	resolvedAlbums   map[string]string   // albumId -> artista principal
	conflictedAlbums map[string]struct{} // albumId -> requiere votación

	// Caché para optimización de inserciones SQL
	seenArtists map[string]struct{}
	seenAlbums  map[string]struct{}

	// Fast Path Cache: path -> {mtime, size}
	dbFileCache map[string]dbFileInfo
}

type dbFileInfo struct {
	mtime  int64
	size   int64
	id     string
	status string
}

func NewScanner(database *db.MasterDatabase) *Scanner {
	return &Scanner{
		DB:               database,
		PriorityJobs:     make(chan string, 100),
		resolvedAlbums:   make(map[string]string),
		conflictedAlbums: make(map[string]struct{}),
		seenArtists:      make(map[string]struct{}),
		seenAlbums:       make(map[string]struct{}),
		dbFileCache:      make(map[string]dbFileInfo),
	}
}

func (s *Scanner) ParseAndSaveFile(path string) error {
	res, err := s.ParseFileWithCache(path, "active")
	if err != nil { return err }
	if res == nil { return nil }

	s.muDB.Lock()
	defer s.muDB.Unlock()

	tx, err := s.DB.Conn.Begin()
	if err != nil { return err }

	tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", res.artistId, res.dbAlbumArtist)
	tx.Exec(`
		INSERT INTO albums (id, artist_id, title, year) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			year = CASE WHEN (year IS NULL OR year = 0) THEN excluded.year ELSE year END
	`, res.albumId, res.artistId, res.albumName)

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

func (s *Scanner) ScanDirectory(rootPath string) error {
	slog.Info("Starting Pulse-Batch library scan", "path", rootPath)
	start := time.Now()

	s.muAlbums.Lock()
	s.resolvedAlbums = make(map[string]string)
	s.conflictedAlbums = make(map[string]struct{})
	s.seenArtists = make(map[string]struct{})
	s.seenAlbums = make(map[string]struct{})
	s.muAlbums.Unlock()

	// 1. Pre-cargar metadatos de la DB
	slog.Info("Indexing existing library metadata...")
	rows, err := s.DB.Conn.Query("SELECT id, path, file_mtime, size, status FROM tracks")
	if err == nil {
		defer rows.Close()
		cacheCount := 0
		for rows.Next() {
			var id, p, status string
			var m, sz int64
			if err := rows.Scan(&id, &p, &m, &sz, &status); err == nil {
				s.dbFileCache[p] = dbFileInfo{mtime: m, size: sz, id: id, status: status}
				cacheCount++
			}
		}
		slog.Info("Metadata cache loaded", "count", cacheCount)
	}

	numWorkers := 4
	if envWorkers := os.Getenv("SGOUND_SCANNER_WORKERS"); envWorkers != "" {
		if val, err := strconv.Atoi(envWorkers); err == nil { numWorkers = val }
	}

	debug.SetGCPercent(20)
	var (
		jobs       = make(chan string, 1000)
		results    = make(chan *trackResult, numWorkers*2)
		slowJobs   = make(chan *trackResult, numWorkers*2)
		wg         sync.WaitGroup
		slowWg     sync.WaitGroup
		writerDone = make(chan bool)
		mu         sync.Mutex
		count      int
	)
	tagSem := make(chan struct{}, 1)

	// A. ESCRITOR DE DB (Lotes de 50)
	go func() {
		batchSize := 50 
		var batch []*trackResult
		ticker := time.NewTicker(10 * time.Second) // Lotes por tiempo más largos
		defer ticker.Stop()
		flush := func() {
			if len(batch) == 0 { return }
			s.muDB.Lock()
			slog.Debug("💾 W-Writer: Syncing", "n", len(batch))
			tx, _ := s.DB.Conn.Begin()
			for _, r := range batch {
				if _, seen := s.seenArtists[r.artistId]; !seen {
					tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", r.artistId, r.dbAlbumArtist)
					s.seenArtists[r.artistId] = struct{}{}
				}
				if _, seen := s.seenAlbums[r.albumId]; !seen {
					tx.Exec(`INSERT INTO albums (id, artist_id, title, year) VALUES (?, ?, ?) ON CONFLICT(id) DO UPDATE SET year = CASE WHEN (year IS NULL OR year = 0) THEN excluded.year ELSE year END`, r.albumId, r.artistId, r.albumName)
					s.seenAlbums[r.albumId] = struct{}{}
				}
				tx.Exec(`UPDATE tracks SET id=?, album_id=?, artist=?, title=?, track_num=?, disc_num=?, duration=?, genre=?, size=?, format=?, composer=?, comment=?, bit_rate=?, sample_rate=?, status=?, instrumental=?, replay_gain=?, file_mtime=?, file_hash=? WHERE path=?`,
					r.trackId, r.albumId, r.artistName, r.title, r.trackNum, r.discNum, r.duration, r.genre, r.size, r.format, r.composer, r.comment, r.bitRate, r.sampleRate, "pending", r.instrumental, r.replayGain, r.fileMtime, r.hash, r.path)
			}
			tx.Commit()
			s.muDB.Unlock()
			time.Sleep(200 * time.Millisecond) // SILENCIO DE RADIO
			batch = nil
			s.seenArtists = make(map[string]struct{})
			s.seenAlbums = make(map[string]struct{})
		}
		for {
			select {
			case r, ok := <-results:
				if !ok { flush(); writerDone <- true; return }
				batch = append(batch, r)
				if len(batch) >= batchSize { flush() }
			case <-ticker.C:
				flush()
			}
		}
	}()

	// B. CARRIL LENTO (ffprobe)
	slowWg.Add(1)
	go func() {
		defer slowWg.Done()
		for res := range slowJobs {
			slog.Debug("🐢 W-Slow: ffprobe", "file", filepath.Base(res.path))
			if data, err := getAudioMetadataWithFFprobe(res.path); err == nil {
				res.duration = data.duration
				res.bitRate = data.bitRate
				res.sampleRate = data.sampleRate
				if data.replayGain != 0 { res.replayGain = data.replayGain }
			}
			res.status = "pending"
			results <- res
		}
	}()

	// C. ANALISTAS
	slog.Info("Starting analysts", "count", numWorkers)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		id := i + 1
		go func(wId int) {
			defer wg.Done()
			for {
				var path string
				var ok bool
				select {
				case path, ok = <-s.PriorityJobs:
					if ok { slog.Debug("👑 W"+strconv.Itoa(wId)+": PRIORITY", "file", filepath.Base(path)) }
				default:
					select {
					case path, ok = <-s.PriorityJobs:
						if ok { slog.Debug("👑 W"+strconv.Itoa(wId)+": PRIORITY", "file", filepath.Base(path)) }
					case path, ok = <-jobs:
					}
				}
				if !ok { return }
				res, err := func() (*trackResult, error) {
					defer func() { if r := recover(); r != nil { slog.Error("🔥 Panic", "worker", wId, "file", filepath.Base(path), "err", r) } }()
					tagSem <- struct{}{}
					defer func() { <-tagSem }()
					slog.Debug("📖 W"+strconv.Itoa(wId), "file", filepath.Base(path))
					return s.ParseFileWithCache(path, "pending")
				}()
				if err != nil || res == nil { continue }
				if res.duration == 0 { slowJobs <- res } else { results <- res }
				mu.Lock()
				count++
				currentCount := count
				if currentCount % 10 == 0 {
					runtime.GC(); debug.FreeOSMemory()
					var m runtime.MemStats; runtime.ReadMemStats(&m)
					slog.Info("🧠 RAM", "used_mb", m.Alloc/1024/1024, "sys_mb", m.Sys/1024/1024)
				}
				mu.Unlock()
				if s.OnProgress != nil { s.OnProgress(path, currentCount) }
			}
		}(id)
	}

	// D. EXPLORADOR (Lotes de 200)
	slog.Info("W-Explorer: Mapping disk...")
	discoveryCount := 0
	
	// 'raw' existentes
	rowsRaw, err := s.DB.Conn.Query("SELECT path FROM tracks WHERE status = 'raw'")
	if err == nil && rowsRaw != nil {
		for rowsRaw.Next() {
			var p string
			if err := rowsRaw.Scan(&p); err == nil { jobs <- p }
		}
		rowsRaw.Close()
	}

	type rawInfo struct { id, path string }
	var buffer []rawInfo
	flushDisc := func() {
		if len(buffer) == 0 { return }
		s.muDB.Lock()
		slog.Debug("📂 W-Explorer: Syncing", "n", len(buffer))
		tx, _ := s.DB.Conn.Begin()
		for _, info := range buffer {
			tx.Exec(`INSERT OR IGNORE INTO tracks (id, album_id, artist, title, path, status, file_mtime, size) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				info.id, "raw_album", "Unknown", filepath.Base(info.path), info.path, "raw", 0, 0)
		}
		tx.Commit()
		s.muDB.Unlock()

		// ENVIAR A COLA FUERA DEL BLOQUEO DE DB (Evita Deadlock)
		for _, info := range buffer {
			jobs <- info.path
		}

		time.Sleep(100 * time.Millisecond) // RESPIRO
		discoveryCount += len(buffer)
		slog.Info("➜ Discovered", "total", discoveryCount)
		buffer = nil
	}

	_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return nil }
		if !d.IsDir() && isAudioFile(d.Name()) == nil {
			_, exists := s.dbFileCache[path]
			if !exists {
				tId := GenerateID(path) // Rápido sin disk I/O
				buffer = append(buffer, rawInfo{tId, path})
				if len(buffer) >= 200 { flushDisc() }
			}
		}
		return nil
	})
	flushDisc()
	slog.Info("W-Explorer: Finished", "total", discoveryCount)

	close(jobs)
	wg.Wait()
	close(slowJobs)
	slowWg.Wait()
	close(results)
	<-writerDone

	slog.Info("Scan completed", "total", count, "duration", time.Since(start))
	s.muAlbums.Lock()
	for id := range s.conflictedAlbums { s.recalculateAlbumArtist(id) }
	s.muAlbums.Unlock()
	return nil
}

func (s *Scanner) ParseFileWithCache(path string, oldStatus string) (*trackResult, error) {
	fileStat, err := os.Stat(path)
	if err != nil { return nil, err }
	trackId := tags.ReadPersistentID(path)
	if trackId == "" { trackId = GenerateID(path) }
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	metadata, err := tag.ReadFrom(f)
	var artistName, albumName, title string
	var trackNum, year, discNum, bitRate, sampleRate int
	var duration float64
	var genre, composer, comment string
	if err != nil {
		artistName = "Unknown Artist"; albumName = "Unknown Album"
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		genre = "Unknown"
	} else {
		artistName = metadata.Artist(); if artistName == "" { artistName = "Unknown Artist" }
		albumName = metadata.Album(); if albumName == "" { albumName = "Unknown Album" }
		title = metadata.Title(); if title == "" { title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) }
		trackNum, _ = metadata.Track(); discNum, _ = metadata.Disc(); year = metadata.Year()
		genre = metadata.Genre(); if genre == "" { genre = "Unknown" }
		composer = metadata.Composer(); comment = metadata.Comment()
	}
	lowerExt := strings.ToLower(filepath.Ext(path))
	if lowerExt == ".flac" { duration, bitRate, sampleRate, _ = getFLACDurationNative(path)
	} else if lowerExt == ".mp3" { duration, bitRate, sampleRate, _ = getMP3DurationNative(path)
	}
	albumArtistTag := ""
	if metadata != nil {
		raw := metadata.Raw()
		for _, tagKey := range []string{"albumartist", "ALBUMARTIST", "TPE2"} {
			if val, ok := raw[tagKey].(string); ok && val != "" { albumArtistTag = val; break }
		}
	}
	var groupingKey string
	if albumArtistTag != "" { groupingKey = "AA:" + NormalizeArtist(albumArtistTag)
	} else {
		dir := filepath.Dir(path); base := strings.ToLower(filepath.Base(dir))
		if strings.HasPrefix(base, "cd") || strings.HasPrefix(base, "disc") || strings.HasPrefix(base, "disk") { dir = filepath.Dir(dir) }
		groupingKey = "DIR:" + dir
	}
	albumId := GenerateID(groupingKey + "|" + NormalizeString(albumName))
	dbAlbumArtist := albumArtistTag; if dbAlbumArtist == "" { dbAlbumArtist = artistName }
	artistId := GenerateID(dbAlbumArtist)
	return &trackResult{
		trackId: trackId, albumId: albumId, artistId: artistId,
		artistName: artistName, dbAlbumArtist: dbAlbumArtist, albumName: albumName,
		title: title, path: path, genre: genre, format: strings.TrimPrefix(filepath.Ext(path), "."),
		composer: composer, comment: comment, hash: "", status: oldStatus,
		trackNum: trackNum, discNum: discNum, year: year, duration: duration,
		bitRate: bitRate, sampleRate: sampleRate, size: fileStat.Size(),
		fileMtime: fileStat.ModTime().Unix(), instrumental: false,
	}, nil
}

func getMP3DurationNative(path string) (float64, int, int, error) {
	f, err := os.Open(path); if err != nil { return 0, 0, 0, err }; defer f.Close()
	header := make([]byte, 10); if _, err := f.Read(header); err != nil { return 0, 0, 0, err }
	offset := int64(0); if string(header[:3]) == "ID3" {
		size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
		offset = 10 + size
	}
	f.Seek(offset, 0); buf := make([]byte, 4096); n, _ := f.Read(buf)
	frameOffset := -1; for i := 0; i < n-4; i++ {
		if buf[i] == 0xFF && (buf[i+1]&0xE0) == 0xE0 { frameOffset = i; break }
	}
	if frameOffset == -1 { return 0, 0, 0, fmt.Errorf("no mp3") }
	b2 := buf[frameOffset+2]; srIndex := (b2 & 0x0C) >> 2; brIndex := (b2 & 0xF0) >> 4
	sr := []int{44100, 48000, 32000, 0}[srIndex]; br := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}[brIndex] * 1000
	xingPos := frameOffset + 36
	if xingPos < n-8 && (string(buf[xingPos:xingPos+4]) == "Xing" || string(buf[xingPos:xingPos+4]) == "Info") {
		numFrames := int64(buf[xingPos+8])<<24 | int64(buf[xingPos+9])<<16 | int64(buf[xingPos+10])<<8 | int64(buf[xingPos+11])
		if numFrames > 0 { return float64(numFrames*1152) / float64(sr), br, sr, nil }
	}
	info, _ := os.Stat(path); if br > 0 { return float64((info.Size()-offset)*8) / float64(br), br, sr, nil }
	return 0, 0, 0, nil
}

func getFLACDurationNative(path string) (float64, int, int, error) {
	f, err := os.Open(path); if err != nil { return 0, 0, 0, err }; defer f.Close()
	header := make([]byte, 10); if _, err := f.Read(header); err != nil { return 0, 0, 0, err }
	offset := int64(0); if string(header[:3]) == "ID3" {
		size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
		offset = 10 + size
	}
	f.Seek(offset, 0); head := make([]byte, 4); if _, err := f.Read(head); err != nil || string(head) != "fLaC" { return 0, 0, 0, nil }
	blockHeader := make([]byte, 4); if _, err := f.Read(blockHeader); err != nil { return 0, 0, 0, err }
	if blockHeader[0]&0x7F != 0 { return 0, 0, 0, nil }
	data := make([]byte, 34); if _, err := f.Read(data); err != nil { return 0, 0, 0, err }
	sampleRate := int(data[10])<<12 | int(data[11])<<4 | int(data[12])>>4
	totalSamples := int64(data[13]&0x0F)<<32 | int64(data[14])<<24 | int64(data[15])<<16 | int64(data[16])<<8 | int64(data[17])
	if sampleRate > 0 {
		duration := float64(totalSamples) / float64(sampleRate); info, _ := os.Stat(path)
		return duration, int((float64(info.Size()) * 8) / duration), sampleRate, nil
	}
	return 0, 0, 0, nil
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
	if !supported[ext] { return fmt.Errorf("no") }
	return nil
}

func getAudioMetadataWithFFprobe(path string) (audioMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration,bit_rate:stream=sample_rate:format_tags=REPLAYGAIN_TRACK_GAIN,replaygain_track_gain", "-of", "default=nokey=0", path)
	out, _ := cmd.Output(); lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	data := audioMetadata{sampleRate: 44100}
	for _, line := range lines {
		if strings.HasPrefix(line, "duration=") { data.duration, _ = strconv.ParseFloat(strings.TrimPrefix(line, "duration="), 64)
		} else if strings.HasPrefix(line, "bit_rate=") { data.bitRate, _ = strconv.Atoi(strings.TrimPrefix(line, "bit_rate="))
		} else if strings.HasPrefix(line, "sample_rate=") { data.sampleRate, _ = strconv.Atoi(strings.TrimPrefix(line, "sample_rate="))
		}
	}
	return data, nil
}

type audioMetadata struct {
	duration, replayGain float64
	bitRate, sampleRate int
}

func (s *Scanner) recalculateAlbumArtist(albumId string) {
	rows, _ := s.DB.Conn.Query(`SELECT artist, COUNT(*) as freq FROM tracks WHERE album_id = ? AND artist != 'Unknown Artist' GROUP BY artist ORDER BY freq DESC, artist ASC`, albumId)
	if rows == nil { return }
	defer rows.Close()
	var winnerName string; var maxFreq int
	if rows.Next() { rows.Scan(&winnerName, &maxFreq) }
	if winnerName == "" { return }
	winnerArtistId := GenerateID(winnerName)
	s.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", winnerArtistId, winnerName)
	s.DB.Conn.Exec("UPDATE albums SET artist_id = ? WHERE id = ?", winnerArtistId, albumId)
}

func GenerateID(input string) string {
	h := sha1.New(); h.Write([]byte(input)); return hex.EncodeToString(h.Sum(nil))
}

func NormalizeArtist(name string) string {
	name = strings.ToLower(name)
	separators := []string{" feat.", " ft.", " feat ", " ft ", " with "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 { name = name[:idx] }
	}
	return strings.TrimSpace(name)
}

func NormalizeString(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer(".", "", ",", "", "-", "", "(", "", ")", "", "[", "", "]", "", "'", "", "\"", "")
	return strings.TrimSpace(r.Replace(s))
}
