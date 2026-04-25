package scanner

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"sgound/internal/db"
	"sgound/internal/tags"
)

type Scanner struct {
	DB          *db.MasterDatabase
	OnProgress  func(path string, count int)
	OnDiscovery func(count int)

	// Estado para API
	IsPaused  bool
	RAMUsedMB int
	muStatus  sync.Mutex

	// Canales de prioridad
	PriorityJobs chan string

	// Sistema de "Tickets de RAM" (4 slots fijos para workers)
	ramTickets chan struct{}

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
		ramTickets:       make(chan struct{}, 4),
		resolvedAlbums:   make(map[string]string),
		conflictedAlbums: make(map[string]struct{}),
		seenArtists:      make(map[string]struct{}),
		seenAlbums:       make(map[string]struct{}),
		dbFileCache:      make(map[string]dbFileInfo),
	}
}

func (s *Scanner) GetStatus() (isPaused bool, ramUsedMB int) {
	s.muStatus.Lock()
	defer s.muStatus.Unlock()
	return s.IsPaused, s.RAMUsedMB
}

// copyString fuerza una nueva asignación de memoria para romper vínculos con buffers grandes
func copyString(s string) string {
	if s == "" {
		return ""
	}
	return string([]byte(s))
}

func (s *Scanner) executeTransaction(fn func(tx *sql.Tx) error) error {
	var err error
	for i := 0; i < 5; i++ {
		tx, err := s.DB.Conn.Begin()
		if err != nil {
			if strings.Contains(err.Error(), "database is locked") {
				time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
				continue
			}
			return err
		}

		err = fn(tx)
		if err == nil {
			return tx.Commit()
		}

		tx.Rollback()
		if strings.Contains(err.Error(), "database is locked") {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("transaction failed after retries: %w", err)
}

func (s *Scanner) ParseAndSaveFile(path string) error {
	res, err := s.ParseFileWithCache(path, "active")
	if err != nil {
		return err
	}
	if res == nil {
		return nil
	}

	s.muDB.Lock()
	defer s.muDB.Unlock()

	return s.executeTransaction(func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", res.artistId, res.dbAlbumArtist); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO albums (id, artist_id, title, year, cover_width, cover_height, cover_size, cover_format) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET 
				year = CASE WHEN (year IS NULL OR year = 0) THEN excluded.year ELSE year END,
				cover_width = CASE WHEN (cover_width IS NULL OR cover_width = 0) THEN excluded.cover_width ELSE cover_width END,
				cover_height = CASE WHEN (cover_height IS NULL OR cover_height = 0) THEN excluded.cover_height ELSE cover_height END,
				cover_size = CASE WHEN (cover_size IS NULL OR cover_size = 0) THEN excluded.cover_size ELSE cover_size END,
				cover_format = CASE WHEN (cover_format IS NULL OR cover_format = '') THEN excluded.cover_format ELSE cover_format END
		`, res.albumId, res.artistId, res.albumName, res.year, res.coverWidth, res.coverHeight, res.coverSize, res.coverFormat); err != nil {
			return err
		}

		_, err = tx.Exec(
			`INSERT OR REPLACE INTO tracks 
			(id, album_id, artist, title, track_num, disc_num, path, duration, genre, size, format, composer, comment, bit_rate, sample_rate, status, instrumental, replay_gain, file_mtime, file_hash) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			res.trackId, res.albumId, res.artistName, res.title, res.trackNum, res.discNum, res.path, res.duration, res.genre, res.size, res.format, res.composer, res.comment, res.bitRate, res.sampleRate, res.status, res.instrumental, res.replayGain, res.fileMtime, res.hash,
		)
		return err
	})
}

func (s *Scanner) ScanDirectory(rootPath string) error {
	// ======= SALVAVIDAS PARA DOCKER =======
	// Bajamos el límite para forzar GC mucho antes de que el contenedor muera
	debug.SetMemoryLimit(150 * 1024 * 1024)
	// ======================================

	done := make(chan struct{})
	defer close(done)

	// ======= PERRO GUARDIÁN DE RAM (Watchdog con Backpressure) =======
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				// Calculamos la RAM "real" que el SO ve (Sys menos lo que ya marcamos como libre)
				realOSMB := (m.Sys - m.HeapReleased) / 1024 / 1024
				
				s.muStatus.Lock()
				s.RAMUsedMB = int(realOSMB)
				if realOSMB > 220 { // Bajamos el umbral de pánico
					if !s.IsPaused {
						slog.Warn("🚨 WATCHDOG: RAM crítica detectada (>220MB), pausando escáner...", "real_mb", realOSMB)
						s.IsPaused = true
					}
					debug.FreeOSMemory()
				} else if realOSMB < 120 && s.IsPaused {
					slog.Info("🟢 WATCHDOG: RAM estabilizada, reanudando escáner", "real_mb", realOSMB)
					s.IsPaused = false
				}
				s.muStatus.Unlock()
			case <-done:
				return
			}
		}
	}()
	// ===========================================

	slog.Info("Starting Pulse-Batch library scan", "path", rootPath)
	start := time.Now()

	s.muAlbums.Lock()
	s.resolvedAlbums = make(map[string]string)
	s.conflictedAlbums = make(map[string]struct{})
	s.seenArtists = make(map[string]struct{})
	s.seenAlbums = make(map[string]struct{})
	s.muAlbums.Unlock()

	// Indexar metadatos existentes
	rows, err := s.DB.Conn.Query("SELECT id, path, file_mtime, size, status FROM tracks")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, p, status string
			var m, sz int64
			if err := rows.Scan(&id, &p, &m, &sz, &status); err == nil {
				s.dbFileCache[p] = dbFileInfo{mtime: m, size: sz, id: id, status: status}
			}
		}
	}

	numWorkers := 3 // Reducimos a 3 por defecto para mayor estabilidad
	if envWorkers := os.Getenv("SGOUND_SCANNER_WORKERS"); envWorkers != "" {
		if val, err := strconv.Atoi(envWorkers); err == nil {
			numWorkers = val
		}
	}

	var (
		jobsMP3    = make(chan string, 500)
		jobsFLAC   = make(chan string, 100)
		results    = make(chan *trackResult, numWorkers*5)
		slowJobs   = make(chan *trackResult, numWorkers*2)
		wg         sync.WaitGroup
		slowWg     sync.WaitGroup
		writerDone = make(chan bool)
		mu         sync.Mutex
		count      int
		scanActive = true
	)

	// Aduana FLAC: Solo un worker a la vez puede procesar archivos pesados
	flacSem := make(chan struct{}, 1)

	// Clasificar y enviar a la cola correcta
	queueJob := func(path string) {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".flac" || ext == ".wav" {
			jobsFLAC <- path
		} else {
			jobsMP3 <- path
		}
	}

	// Helper para procesar una pista y despachar resultados
	processTrack := func(p string, wId int) {
		res, err := func() (*trackResult, error) {
			s.ramTickets <- struct{}{}
			defer func() { <-s.ramTickets }()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("🔥 Panic", "worker", wId, "file", filepath.Base(p), "err", r)
				}
			}()
			
			r, e := s.ParseFileWithCache(p, "pending")
			return r, e
		}()
		
		if err != nil || res == nil {
			return
		}
		if res.duration == 0 {
			slowJobs <- res
		} else {
			results <- res
		}
		mu.Lock()
		count++
		currentCount := count
		// Limpieza proactiva cada 100 archivos
		if currentCount % 100 == 0 {
			go func() {
				runtime.GC()
				debug.FreeOSMemory()
			}()
		}
		mu.Unlock()
		if s.OnProgress != nil {
			s.OnProgress(p, currentCount)
		}
	}

	// A. ESCRITOR DE DB
	go func() {
		batchSize := 50
		var batch []*trackResult
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		flush := func() {
			if len(batch) == 0 {
				return
			}
			s.muDB.Lock()
			err := s.executeTransaction(func(tx *sql.Tx) error {
				for _, r := range batch {
					if _, seen := s.seenArtists[r.artistId]; !seen {
						if _, err := tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", r.artistId, r.dbAlbumArtist); err != nil {
							return err
						}
						s.seenArtists[r.artistId] = struct{}{}
					}
					if _, seen := s.seenAlbums[r.albumId]; !seen {
						if _, err := tx.Exec(`INSERT INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET year = CASE WHEN (year IS NULL OR year = 0) THEN excluded.year ELSE year END`, r.albumId, r.artistId, r.albumName, r.year); err != nil {
							return err
						}
						s.seenAlbums[r.albumId] = struct{}{}
					}
					if _, err := tx.Exec(`UPDATE tracks SET id=?, album_id=?, artist=?, title=?, track_num=?, disc_num=?, duration=?, genre=?, size=?, format=?, composer=?, comment=?, bit_rate=?, sample_rate=?, status=?, instrumental=?, replay_gain=?, file_mtime=?, file_hash=? WHERE path=?`,
						r.trackId, r.albumId, r.artistName, r.title, r.trackNum, r.discNum, r.duration, r.genre, r.size, r.format, r.composer, r.comment, r.bitRate, r.sampleRate, "pending", r.instrumental, r.replayGain, r.fileMtime, r.hash, r.path); err != nil {
						return err
					}
				}
				return nil
			})
			s.muDB.Unlock()

			if err != nil {
				slog.Error("Failed to flush batch to DB", "error", err, "batchSize", len(batch))
			}
			
			batch = nil
			s.seenArtists = make(map[string]struct{})
			s.seenAlbums = make(map[string]struct{})
			s.muAlbums.Lock()
			s.resolvedAlbums = make(map[string]string)
			s.muAlbums.Unlock()

			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			sysMB := m.Sys / 1024 / 1024
			releasedMB := m.HeapReleased / 1024 / 1024
			realOSMB := sysMB - releasedMB

			slog.Info("🧠 RAM Status", 
				"used_mb", m.Alloc/1024/1024, 
				"sys_mb", sysMB, 
				"real_os_mb", realOSMB,
			)
		}
		for {
			select {
			case r, ok := <-results:
				if !ok {
					flush()
					writerDone <- true
					return
				}
				batch = append(batch, r)
				if len(batch) >= batchSize {
					flush()
				}
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
			if data, err := getAudioMetadataWithFFprobe(res.path); err == nil {
				res.duration = data.duration
				res.bitRate = data.bitRate
				res.sampleRate = data.sampleRate
				if data.replayGain != 0 {
					res.replayGain = data.replayGain
				}
			}
			res.status = "pending"
			results <- res
		}
	}()

	// C. ANALISTAS (BIBLIOTECARIOS)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		id := i + 1
		go func(wId int) {
			defer wg.Done()
			for {
				// Backpressure check
				for {
					s.muStatus.Lock()
					p := s.IsPaused
					s.muStatus.Unlock()
					if !p { break }
					time.Sleep(1 * time.Second)
				}

				var path string
				var ok bool

				// 1. Siempre mirar Prioridad primero
				select {
				case path, ok = <-s.PriorityJobs:
					if ok {
						slog.Info("👑 W"+strconv.Itoa(wId)+": PRIORITY", "file", filepath.Base(path))
						processTrack(path, wId)
						continue
					}
				default:
				}

				// 2. Intento FLAC (Aduana) - No bloqueante
				select {
				case flacSem <- struct{}{}:
					select {
					case path, ok = <-jobsFLAC:
						if ok {
							processTrack(path, wId)
						}
					default:
					}
					<-flacSem
					if path != "" {
						continue
					}
					if !ok {
						goto checkMP3
					}
				default:
				}

			checkMP3:
				select {
				case path, ok = <-jobsMP3:
					if ok {
						processTrack(path, wId)
						continue
					}
				default:
				}

				if !scanActive && len(jobsMP3) == 0 && len(jobsFLAC) == 0 && len(s.PriorityJobs) == 0 {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(id)
	}

	// D. EXPLORADOR (W-Explorer)
	slog.Info("W-Explorer: Mapping disk...")
	discoveryCount := 0

	// 'raw' existentes
	rowsRaw, err := s.DB.Conn.Query("SELECT path FROM tracks WHERE status = 'raw'")
	if err == nil && rowsRaw != nil {
		var rawPaths []string
		for rowsRaw.Next() {
			var p string
			if err := rowsRaw.Scan(&p); err == nil {
				rawPaths = append(rawPaths, p)
			}
		}
		rowsRaw.Close()
		go func() {
			for _, p := range rawPaths {
				queueJob(p)
			}
		}()
	}

	type rawInfo struct{ id, path string }
	var buffer []rawInfo
	flushDisc := func() {
		if len(buffer) == 0 {
			return
		}
		s.muDB.Lock()
		err := s.executeTransaction(func(tx *sql.Tx) error {
			for _, info := range buffer {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO tracks (id, album_id, artist, title, path, status, file_mtime, size) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					info.id, "raw_album", "Unknown", filepath.Base(info.path), info.path, "raw", 0, 0); err != nil {
					return err
				}
			}
			return nil
		})
		s.muDB.Unlock()

		if err != nil {
			slog.Error("Failed to flush raw discovery to DB", "error", err, "n", len(buffer))
		}
		
		slog.Debug("📂 W-Explorer: Syncing raw", "n", len(buffer))
		for _, info := range buffer {
			queueJob(info.path)
		}
		discoveryCount += len(buffer)
		if s.OnDiscovery != nil {
			s.OnDiscovery(discoveryCount)
		}
		buffer = nil
	}

	_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && IsAudioFile(d.Name()) == nil {
			if _, exists := s.dbFileCache[path]; !exists {
				buffer = append(buffer, rawInfo{GenerateID(path), path})
				if len(buffer) >= 50 {
					flushDisc()
				}
			}
		}
		return nil
	})
	flushDisc()

	scanActive = false
	wg.Wait()
	close(slowJobs)
	slowWg.Wait()
	close(results)
	<-writerDone

	slog.Info("Scan completed", "total", count, "duration", time.Since(start))
	s.muAlbums.Lock()
	for id := range s.conflictedAlbums {
		s.recalculateAlbumArtist(id)
	}
	s.muAlbums.Unlock()

	// Liberar memoria agresivamente al finalizar
	s.dbFileCache = make(map[string]dbFileInfo)
	s.seenArtists = nil
	s.seenAlbums = nil
	s.resolvedAlbums = nil
	s.conflictedAlbums = nil
	debug.FreeOSMemory()

	return nil
}

func (s *Scanner) ParseFileWithCache(path string, oldStatus string) (*trackResult, error) {
	fileStat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	meta, err := tags.Open(path)

	var artistName, albumName, title, genre, composer, comment, albumArtistTag string
	var trackNum, year, discNum int
	var coverWidth, coverHeight, coverSize int
	var coverFormat string

	if err != nil {
		slog.Warn("Could not read tags, using file info", "path", path, "err", err)
		artistName = "Unknown Artist"
		albumName = "Unknown Album"
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		genre = "Unknown"
	} else {
		defer meta.Close()
		title = copyString(meta.Title())
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		artistName = copyString(meta.Artist())
		if artistName == "" {
			artistName = "Unknown Artist"
		}
		albumName = copyString(meta.Album())
		if albumName == "" {
			albumName = "Unknown Album"
		}
		genre = copyString(meta.Genre())
		if genre == "" {
			genre = "Unknown"
		}

		composer = copyString(meta.Composer())
		comment = copyString(meta.Comment())
		albumArtistTag = copyString(meta.AlbumArtist())
		year = meta.Year()
		trackNum, _ = meta.Track()
		discNum, _ = meta.Disc()

		// Carátula: Obtener dimensiones y tamaño de forma eficiente (solo cabecera)
		if pic := meta.Picture(); pic != nil {
			coverSize = len(pic.Data)
			coverFormat = pic.MIME
			// image.DecodeConfig solo lee los metadatos de la imagen, NO los píxeles.
			// Esto evita picos de 30-100MB por imagen en archivos con carátulas 4K.
			cfg, _, err := image.DecodeConfig(bytes.NewReader(pic.Data))
			if err == nil {
				coverWidth = cfg.Width
				coverHeight = cfg.Height
			}
		}
	}

	trackId := tags.ReadPersistentID(path)
	if trackId == "" {
		trackId = GenerateID(path)
	}

	var bitRate, sampleRate int
	var duration float64
	lowerExt := strings.ToLower(filepath.Ext(path))
	switch lowerExt {
	case ".flac":
		duration, bitRate, sampleRate, _ = getFLACDurationNative(path)
	case ".mp3":
		duration, bitRate, sampleRate, _ = getMP3DurationNative(path)
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
	if dbAlbumArtist == "" {
		dbAlbumArtist = artistName
	}
	artistId := GenerateID(dbAlbumArtist)

	// ESTIMAR TAMAÑO DE RAM REAL (Bytes)
	mSize := len(artistName) + len(albumName) + len(title) + len(genre) + len(composer) + len(comment) + len(trackId) + len(path) + 1024

	return &trackResult{
		trackId: copyString(trackId), albumId: albumId, artistId: artistId,
		artistName: artistName, dbAlbumArtist: dbAlbumArtist, albumName: albumName,
		title: title, path: copyString(path), genre: genre, format: strings.TrimPrefix(filepath.Ext(path), "."),
		composer: composer, comment: comment, hash: "", status: oldStatus,
		trackNum: trackNum, discNum: discNum, year: year, duration: duration,
		bitRate: bitRate, sampleRate: sampleRate, size: fileStat.Size(),
		fileMtime:    fileStat.ModTime().Unix(),
		MetadataSize: mSize,
		coverWidth:   coverWidth,
		coverHeight:  coverHeight,
		coverSize:    coverSize,
		coverFormat:  coverFormat,
	}, nil
}

func getMP3DurationNative(path string) (float64, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	header := make([]byte, 10)
	if _, err := f.Read(header); err != nil {
		return 0, 0, 0, err
	}
	offset := int64(0)
	if string(header[:3]) == "ID3" {
		size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
		offset = 10 + size
	}
	f.Seek(offset, 0)
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	frameOffset := -1
	for i := 0; i < n-4; i++ {
		if buf[i] == 0xFF && (buf[i+1]&0xE0) == 0xE0 {
			frameOffset = i
			break
		}
	}
	if frameOffset == -1 {
		return 0, 0, 0, fmt.Errorf("no mp3")
	}
	b2 := buf[frameOffset+2]
	srIndex := (b2 & 0x0C) >> 2
	brIndex := (b2 & 0xF0) >> 4
	srTable := []int{44100, 48000, 32000, 0}
	brTable := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	sr := srTable[srIndex]
	br := brTable[brIndex] * 1000
	xingPos := frameOffset + 36
	if xingPos < n-8 && (string(buf[xingPos:xingPos+4]) == "Xing" || string(buf[xingPos:xingPos+4]) == "Info") {
		numFrames := int64(buf[xingPos+8])<<24 | int64(buf[xingPos+9])<<16 | int64(buf[xingPos+10])<<8 | int64(buf[xingPos+11])
		if numFrames > 0 {
			return float64(numFrames*1152) / float64(sr), br, sr, nil
		}
	}
	info, _ := os.Stat(path)
	if br > 0 {
		return float64((info.Size()-offset)*8) / float64(br), br, sr, nil
	}
	return 0, 0, 0, nil
}

func getFLACDurationNative(path string) (float64, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	header := make([]byte, 10)
	if _, err := f.Read(header); err != nil {
		return 0, 0, 0, err
	}
	offset := int64(0)
	if string(header[:3]) == "ID3" {
		size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
		offset = 10 + size
	}
	f.Seek(offset, 0)
	head := make([]byte, 4)
	if _, err := f.Read(head); err != nil || string(head) != "fLaC" {
		return 0, 0, 0, nil
	}
	blockHeader := make([]byte, 4)
	if _, err := f.Read(blockHeader); err != nil {
		return 0, 0, 0, err
	}
	if blockHeader[0]&0x7F != 0 {
		return 0, 0, 0, nil
	}
	data := make([]byte, 34)
	if _, err := f.Read(data); err != nil {
		return 0, 0, 0, err
	}
	sampleRate := int(data[10])<<12 | int(data[11])<<4 | int(data[12])>>4
	totalSamples := int64(data[13]&0x0F)<<32 | int64(data[14])<<24 | int64(data[15])<<16 | int64(data[16])<<8 | int64(data[17])
	if sampleRate > 0 {
		duration := float64(totalSamples) / float64(sampleRate)
		info, _ := os.Stat(path)
		return duration, int((float64(info.Size()) * 8) / duration), sampleRate, nil
	}
	return 0, 0, 0, nil
}

func IsAudioFile(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := map[string]bool{".mp3": true, ".flac": true, ".ogg": true, ".m4a": true, ".mp4": true, ".opus": true, ".wav": true}
	if !supported[ext] {
		return fmt.Errorf("no")
	}
	return nil
}

func getAudioMetadataWithFFprobe(path string) (audioMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration,bit_rate:stream=sample_rate:format_tags=REPLAYGAIN_TRACK_GAIN,replaygain_track_gain", "-of", "json", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return audioMetadata{}, err
	}
	if err := cmd.Start(); err != nil {
		return audioMetadata{}, err
	}
	defer func() { _, _ = io.Copy(io.Discard, stdout); cmd.Wait() }()
	var output struct {
		Format struct {
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			SampleRate string `json:"sample_rate"`
		} `json:"streams"`
	}
	if err := json.NewDecoder(stdout).Decode(&output); err != nil {
		return audioMetadata{}, err
	}
	data := audioMetadata{sampleRate: 44100}
	if output.Format.Duration != "" {
		data.duration, _ = strconv.ParseFloat(output.Format.Duration, 64)
	}
	if output.Format.BitRate != "" {
		data.bitRate, _ = strconv.Atoi(output.Format.BitRate)
	}
	if len(output.Streams) > 0 && output.Streams[0].SampleRate != "" {
		data.sampleRate, _ = strconv.Atoi(output.Streams[0].SampleRate)
	}
	for k, v := range output.Format.Tags {
		lk := strings.ToLower(k)
		if lk == "replaygain_track_gain" || lk == "REPLAYGAIN_TRACK_GAIN" {
			val := strings.TrimSuffix(v, " dB")
			data.replayGain, _ = strconv.ParseFloat(val, 64)
		}
	}
	return data, nil
}

type audioMetadata struct {
	duration, replayGain float64
	bitRate, sampleRate  int
}

func (s *Scanner) recalculateAlbumArtist(albumId string) {
	rows, _ := s.DB.Conn.Query(`SELECT artist, COUNT(*) as freq FROM tracks WHERE album_id = ? AND artist != 'Unknown Artist' GROUP BY artist ORDER BY freq DESC, artist ASC`, albumId)
	if rows == nil {
		return
	}
	defer rows.Close()
	var winnerName string
	var maxFreq int
	if rows.Next() {
		rows.Scan(&winnerName, &maxFreq)
	}
	if winnerName == "" {
		return
	}
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
	separators := []string{" feat.", " ft.", " feat ", " ft ", " with "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 {
			name = name[:idx]
		}
	}
	return strings.TrimSpace(name)
}

func NormalizeString(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer(".", "", ",", "", "-", "", "(", "", ")", "", "[", "", "]", "", "'", "", "\"", "")
	return strings.TrimSpace(r.Replace(s))
}

type trackResult struct {
	trackId, albumId, artistId, artistName, dbAlbumArtist, albumName, title, path, genre, format, composer, comment, hash, status string
	trackNum, discNum, year, bitRate, sampleRate                                                                                  int
	duration, replayGain                                                                                                          float64
	size, fileMtime                                                                                                               int64
	MetadataSize                                                                                                                  int
	instrumental                                                                                                                  bool
	coverWidth, coverHeight, coverSize                                                                                            int
	coverFormat                                                                                                                   string
}
