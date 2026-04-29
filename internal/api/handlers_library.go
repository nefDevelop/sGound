package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"sgound/internal/db"
	"sgound/internal/models"
	"sgound/internal/scanner"
	"sgound/internal/tags"
	"sgound/internal/utils"
)

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if val, ok := s.scanStatuses.Load("global"); ok {
		if val.(*ScanStatus).Scanning {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"already scanning"}`))
			return
		}
	}

	status := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store("global", status)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"global scan started"}`))

	go func() {
		s.updateInboxCount()
		sc := scanner.NewScanner(s.MasterDB)
		s.activeScanner = sc 

		sc.OnDiscovery = func(count int) {
			s.updateInboxCount()
		}

		sc.OnProgress = func(path string, count int) {
			if count%5 == 0 {
				status.Processed = count
				s.scanStatuses.Store("global", status)
				s.updateInboxCount()
			}
		}

		defer func() {
			status.Scanning = false
			s.scanStatuses.Store("global", status)
			s.activeScanner = nil 
			s.updateInboxCount()
			s.EnrichArtists()
		}()
		if err := sc.ScanDirectory(s.Config.MusicPath); err != nil {
			slog.Error("Global scan failed", "error", err)
		}
	}()
}

func (s *Server) handleScanStatus(w http.ResponseWriter, r *http.Request) {
	scanning := false
	processed := 0
	if val, ok := s.scanStatuses.Load("global"); ok {
		sStatus := val.(*ScanStatus)
		scanning = sStatus.Scanning
		processed = sStatus.Processed
	}

	isPaused := false
	ramUsed := 0
	if s.activeScanner != nil {
		isPaused, ramUsed = s.activeScanner.GetStatus()
	}

	// Re-check counts directly from DB to ensure real-time accuracy in the UI
	var ic, rc int64
	err1 := s.MasterDB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'pending'").Scan(&ic)
	err2 := s.MasterDB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'raw'").Scan(&rc)

	if err1 != nil || err2 != nil {
		slog.Error("Database error fetching real-time counts", "err1", err1, "err2", err2)
		// Fallback to cached values if DB is locked
		ic = s.inboxCount.Load()
		rc = s.rawCount.Load()
	} else {
		// Sincronizar los contadores atómicos internos
		s.inboxCount.Store(ic)
		s.rawCount.Store(rc)
	}

	// También chequear si hay aprobación en curso
	approvalProcessed := 0
	approvalScanning := false
	if val, ok := s.scanStatuses.Load("approval"); ok {
		sStatus := val.(*ScanStatus)
		approvalScanning = sStatus.Scanning
		approvalProcessed = sStatus.Processed
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scanning":          scanning || approvalScanning,
		"processed":         processed,
		"approvalProcessed": approvalProcessed,
		"rawCount":          rc,
		"inboxCount":        ic,
		"isPaused":          isPaused,
		"ramUsedMB":         ramUsed,
	})
}

func (s *Server) handlePrioritizeTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	trackID := r.URL.Query().Get("id")
	if trackID == "" {
		http.Error(w, "Missing track ID", http.StatusBadRequest)
		return
	}

	var path string
	err := s.MasterDB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	if s.activeScanner != nil {
		select {
		case s.activeScanner.PriorityJobs <- path:
			slog.Debug("👑 Track prioritized", "id", trackID)
		default:
		}
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"prioritized"}`))
}

func (s *Server) handleGetInbox(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("status", "pending,raw")
	r.URL.RawQuery = q.Encode()
	s.handleGetTracks(w, r)
}

func (s *Server) handleApproveAllTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verificar si ya hay un proceso en curso
	if val, ok := s.scanStatuses.Load("approval"); ok {
		if val.(*ScanStatus).Scanning {
			http.Error(w, "Approval already in progress", http.StatusConflict)
			return
		}
	}

	// Obtener todos los IDs de una vez para procesar
	rows, err := s.MasterDB.Conn.Query("SELECT id FROM tracks WHERE status IN ('pending', 'raw')")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success", "message":"No tracks to approve"}`))
		return
	}

	status := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store("approval", status)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf(`{"status":"started", "total":%d}`, len(ids))))

	go func(ids []string) {
		defer func() {
			status.Scanning = false
			s.scanStatuses.Store("approval", status)
			s.updateInboxCount()
		}()

		batchSize := 100
		for i := 0; i < len(ids); i += batchSize {
			end := i + batchSize
			if end > len(ids) {
				end = len(ids)
			}
			batch := ids[i:end]

			// Para no duplicar lógica, extraemos el núcleo de handleApproveTracks o lo llamamos internamente
			// Pero como handleApproveTracks requiere http.ResponseWriter, mejor refactorizamos el núcleo
			s.internalApproveBatch(batch)

			status.Processed = end
			s.scanStatuses.Store("approval", status)
		}
	}(ids)
}

func (s *Server) internalApproveBatch(ids []string) {
	dbase := s.MasterDB
	processedAlbums := make(map[string]bool)

	for _, oldID := range ids {
		var path, albumID, artistName, title, albumTitle string
		err := dbase.Conn.QueryRow(`
			SELECT t.path, t.album_id, t.artist, t.title, al.title 
			FROM tracks t 
			JOIN albums al ON t.album_id = al.id 
			WHERE t.id = ?`, oldID).Scan(&path, &albumID, &artistName, &title, &albumTitle)
		if err != nil {
			slog.Error("Failed to fetch track for approval", "id", oldID, "error", err)
			continue
		}

		if s.Watcher != nil {
			s.Watcher.MutePath(path, 15*time.Second)
		}

		editor := tags.NewEditor(dbase)

		// 1. Asegurar consistencia de Artista y Álbum
		// Durante el escaneo 'raw' o 'pending', estos registros podrían ser genéricos.
		// Al aprobar, los consolidamos.
		artistID := utils.GenerateID(utils.NormalizeArtist(artistName))
		_ , _ = dbase.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistID, artistName)
		
		// Regenerar albumID basado en el artista real y título para evitar colisiones genéricas
		groupingKey := "AA:" + utils.NormalizeArtist(artistName)
		newAlbumID := utils.GenerateID(groupingKey + "|" + utils.NormalizeString(albumTitle))
		
		if newAlbumID != albumID {
			_ , _ = dbase.Conn.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title) VALUES (?, ?, ?)", newAlbumID, artistID, albumTitle)
			_ , _ = dbase.Conn.Exec("UPDATE tracks SET album_id = ? WHERE id = ?", newAlbumID, oldID)
			albumID = newAlbumID
		}

		// 2. Procesar Carátula (solo una vez por álbum en el lote)
		if !processedAlbums[albumID] {
			s.ensureAlbumMetadata(path, oldID, albumID, editor)
			processedAlbums[albumID] = true
		}

		// 3. Persistencia física del ID
		// Intentamos escribir el ID en el archivo para que sea "inmune" a movimientos manuales futuros
		if err := tags.WritePersistentID(path, oldID); err != nil {
			slog.Warn("Could not write physical SGOUND_ID", "path", path, "error", err)
		}

		// 4. Activación en BD
		dbSuccess := false
		for i := 0; i < 3; i++ {
			errTx := s.executeTransaction(func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE tracks SET status = 'active' WHERE id = ?", oldID)
				return err
			})
			if errTx == nil {
				dbSuccess = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// 5. Renombrado y Organización
		if dbSuccess && s.Config.AutoRename {
			newPath, err := editor.OrganizeFile(oldID, s.Config.MusicPath, s.Config.RenamePattern)
			if err != nil {
				slog.Error("Failed to organize file during approval", "id", oldID, "error", err)
			} else if newPath != "" {
				slog.Debug("File organized", "old", path, "new", newPath)
			}
		}

		s.updateInboxCount()
	}
}

func (s *Server) ensureAlbumMetadata(p string, tid string, albID string, editor *tags.Editor) {
	metadata, _ := tags.Open(p)
	var imgData []byte
	var mime string

	// 1. Intentar encontrar carátula si el archivo no tiene pero la carpeta sí
	if metadata == nil || metadata.Picture() == nil {
		dir := filepath.Dir(p)
		common := []string{"cover", "folder", "front", "album", "artwork"}
		entries, _ := os.ReadDir(dir)
		for _, ent := range entries {
			name := strings.ToLower(ent.Name())
			ext := filepath.Ext(name)
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
				continue
			}
			base := strings.TrimSuffix(name, ext)
			found := false
			for _, c := range common {
				if strings.HasPrefix(base, c) {
					found = true
					break
				}
			}
			if found {
				imgData, _ = os.ReadFile(filepath.Join(dir, ent.Name()))
				mime = "image/jpeg"
				if ext == ".png" { mime = "image/png" }
				_ = editor.EmbedCover(tid, imgData, mime)
				break
			}
		}
	}

	// 2. Sincronizar dimensiones en la base de datos
	metadata, _ = tags.Open(p)
	if metadata != nil && metadata.Picture() != nil {
		pic := metadata.Picture()
		src, _, err := image.Decode(bytes.NewReader(pic.Data))
		if err == nil {
			w := src.Bounds().Dx()
			h := src.Bounds().Dy()
			s.MasterDB.Conn.Exec(`
				UPDATE albums 
				SET cover_width = ?, cover_height = ?, cover_size = ?, cover_format = ?
				WHERE id = ?`,
				w, h, len(pic.Data), pic.MIME, albID)
		}
	}
}

func (s *Server) handleApproveTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	s.internalApproveBatch(data.IDs)

	var totalActive int
	s.MasterDB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'active'").Scan(&totalActive)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "approved":%d, "total":%d}`, len(data.IDs), totalActive)))
	s.updateInboxCount()
}


func (s *Server) handleRejectTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	for _, id := range data.IDs {
		_, err := s.getDB(r).Conn.Exec("UPDATE tracks SET status = 'rejected' WHERE id = ?", id)
		if err != nil {
			slog.Error("Failed to reject track", "id", id, "error", err)
		}
	}

	s.updateInboxCount()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleAnalyzeInbox(w http.ResponseWriter, r *http.Request) {
	dbase := s.getDB(r)
	rows, err := dbase.Conn.Query(`
		SELECT t.id, t.artist, al.title, COALESCE(al.year, 0)
		FROM tracks t 
		JOIN albums al ON t.album_id = al.id 
		WHERE t.status = 'pending'
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type PendingTrack struct {
		ID     string
		Artist string
		Album  string
		Year   int
	}
	var tracks []PendingTrack
	for rows.Next() {
		var t PendingTrack
		if err := rows.Scan(&t.ID, &t.Artist, &t.Album, &t.Year); err == nil {
			tracks = append(tracks, t)
		}
	}

	type Suggestion struct {
		TrackID string `json:"trackId"`
		Artist  string `json:"artist"`
		Album   string `json:"album"`
		Year    int    `json:"year"`
	}
	var suggestions []Suggestion = make([]Suggestion, 0)

	type groupKey struct {
		normArtist string
		albumTitle string
	}
	groups := make(map[groupKey][]string) 
	trackMeta := make(map[string]PendingTrack)

	for _, t := range tracks {
		norm := utils.NormalizeArtist(t.Artist)
		key := groupKey{norm, t.Album}
		groups[key] = append(groups[key], t.ID)
		trackMeta[t.ID] = t
	}

	albumTitles := make([]string, 0)
	for key := range groups {
		isGeneric := len(key.albumTitle) < 3
		if !isGeneric {
			if _, err := strconv.Atoi(key.albumTitle); err == nil { isGeneric = true }
		}
		if !isGeneric { albumTitles = append(albumTitles, key.albumTitle) }
	}

	consensusData := make(map[string][]struct {
		Artist string
		Year   int
	})
	if len(albumTitles) > 0 {
		placeholders := make([]string, len(albumTitles))
		args := make([]interface{}, len(albumTitles))
		for i, t := range albumTitles {
			placeholders[i] = "?"
			args[i] = t
		}

		query := fmt.Sprintf(`
			SELECT t.artist, al.title, COALESCE(al.year, 0)
			FROM tracks t
			JOIN albums al ON t.album_id = al.id
			WHERE t.status = 'active' AND al.title IN (%s)
		`, strings.Join(placeholders, ","))

		cRows, err := dbase.Conn.Query(query, args...)
		if err == nil {
			defer cRows.Close()
			for cRows.Next() {
				var a, t string
				var y int
				if err := cRows.Scan(&a, &t, &y); err == nil {
					consensusData[t] = append(consensusData[t], struct {
						Artist string
						Year   int
					}{a, y})
				}
			}
		}
	}

	for key, ids := range groups {
		data, exists := consensusData[key.albumTitle]
		if !exists { continue }

		counts := make(map[string]int)
		years := make(map[int]int)

		for _, d := range data {
			dNorm := utils.NormalizeArtist(d.Artist)
			if strings.Contains(dNorm, key.normArtist) || strings.Contains(key.normArtist, dNorm) {
				counts[d.Artist]++
				if d.Year > 0 { years[d.Year]++ }
			}
		}

		if len(counts) == 0 { continue }
		winnerArtist := ""; maxCount := -1
		for a, c := range counts {
			if c > maxCount { maxCount = c; winnerArtist = a }
		}
		winnerYear := 0; maxYearCount := -1
		for y, c := range years {
			if c > maxYearCount { maxYearCount = c; winnerYear = y }
		}

		if winnerArtist != "" {
			winnerNorm := utils.NormalizeArtist(winnerArtist)
			for _, id := range ids {
				t := trackMeta[id]
				currentNorm := utils.NormalizeArtist(t.Artist)
				isSameArtistRoot := winnerNorm == currentNorm

				if isSameArtistRoot && (t.Artist != winnerArtist || (winnerYear > 0 && t.Year != winnerYear)) {
					suggestions = append(suggestions, Suggestion{TrackID: id, Artist: winnerArtist, Album: t.Album, Year: winnerYear})
				} else if !isSameArtistRoot && (winnerYear > 0 && t.Year != winnerYear) {
					suggestions = append(suggestions, Suggestion{TrackID: id, Artist: t.Artist, Album: t.Album, Year: winnerYear})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "suggestions": suggestions})
}

func (s *Server) handleApplyInboxSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Suggestions []struct {
			TrackID string `json:"trackId"`
			Artist  string `json:"artist"`
			Album   string `json:"album"`
			Year    int    `json:"year"`
		} `json:"suggestions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	dbase := s.getDB(r)
	
	var err error
	for i := 0; i < 5; i++ {
		tx, errTx := dbase.Conn.Begin()
		if errTx != nil {
			if strings.Contains(errTx.Error(), "database is locked") {
				time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
				continue
			}
			http.Error(w, errTx.Error(), http.StatusInternalServerError)
			return
		}

		success := true
		for _, sug := range data.Suggestions {
			artistId := utils.GenerateID(sug.Artist)
			if _, err = tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistId, sug.Artist); err != nil { success = false; break }

			groupingKey := "AA:" + utils.NormalizeArtist(sug.Artist)
			albumId := utils.GenerateID(groupingKey + "|" + utils.NormalizeString(sug.Album))
			if _, err = tx.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumId, artistId, sug.Album, sug.Year); err != nil { success = false; break }

			if _, err = tx.Exec("UPDATE tracks SET album_id = ?, artist = ? WHERE id = ?", albumId, sug.Artist, sug.TrackID); err != nil { success = false; break }
		}

		if !success {
			tx.Rollback()
			if strings.Contains(err.Error(), "database is locked") { time.Sleep(time.Duration(100*(i+1)) * time.Millisecond); continue }
			http.Error(w, err.Error(), http.StatusInternalServerError); return
		}

		if err = tx.Commit(); err != nil {
			if strings.Contains(err.Error(), "database is locked") { time.Sleep(time.Duration(100*(i+1)) * time.Millisecond); continue }
			http.Error(w, err.Error(), http.StatusInternalServerError); return
		}
		break
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
	s.updateInboxCount()
}

func (s *Server) handleLibraryHealth(w http.ResponseWriter, r *http.Request) {
	health := make(map[string]interface{})
	var incomplete int
	s.getDB(r).Conn.QueryRow(`
		SELECT COUNT(*) FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists ar ON al.artist_id = ar.id
		WHERE status = 'active' AND (
			(t.genre IS NULL OR t.genre = '' OR t.genre = 'Unknown') OR
			(ar.name = 'Unknown Artist') OR
			(al.title = 'Unknown Album')
		)`).Scan(&incomplete)
	health["missingTags"] = incomplete
	dups, _ := s.getDB(r).GetDuplicates()
	health["duplicateCount"] = len(dups)
	orphans, _ := s.getDB(r).GetOrphanedTracks()
	health["orphanCount"] = len(orphans)
	lowQual, _ := s.getDB(r).GetLowQualityTracks(128)
	health["lowQualityCount"] = len(lowQual)
	var missingCovers, smallCovers, hugeCovers int
	s.getDB(r).Conn.QueryRow(`
		SELECT COUNT(DISTINCT al.id) 
		FROM albums al
		JOIN tracks t ON t.album_id = al.id
		WHERE t.status = 'active' AND (al.cover_width IS NULL OR al.cover_width = 0)`).Scan(&missingCovers)
	
	s.getDB(r).Conn.QueryRow(`
		SELECT COUNT(DISTINCT al.id) 
		FROM albums al
		JOIN tracks t ON t.album_id = al.id
		WHERE t.status = 'active' AND al.cover_width > 0 AND (al.cover_width < 300 OR al.cover_height < 300)`).Scan(&smallCovers)
	
	s.getDB(r).Conn.QueryRow(`
		SELECT COUNT(DISTINCT al.id) 
		FROM albums al
		JOIN tracks t ON t.album_id = al.id
		WHERE t.status = 'active' AND (al.cover_width > 1500 OR al.cover_height > 1500 OR al.cover_size > 2000000)`).Scan(&hugeCovers)
	
	health["missingCovers"] = missingCovers
	health["smallCovers"] = smallCovers
	health["hugeCovers"] = hugeCovers
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleRateTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	user, _ := r.Context().Value(userKey).(*db.User)
	var data struct {
		ID     string `json:"id"`
		Rating int    `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	err := s.MasterDB.UpdateTrackUserData(user.Username, data.ID, data.Rating, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	editor := tags.NewEditor(s.getDB(r))
	tagData := map[string]interface{}{ "rating": float64(data.Rating) }
	if _, err := editor.UpdateTags(data.ID, tagData); err != nil {
		slog.Error("Failed to update tags", "track", data.ID, "err", err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
	s.updateInboxCount()
}

func (s *Server) handleLibraryCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	orphans, err := s.getDB(r).GetOrphanedTracks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var ids []string
	for _, o := range orphans { ids = append(ids, o.ID) }
	if err := s.getDB(r).DeleteTracks(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "removed": %d}`, len(ids))))
}

func (s *Server) handleLibraryResyncIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	
	dbase := s.getDB(r)
	
	// 1. REPARACIÓN PROFUNDA DE ARTISTAS (Limpieza de corrupción y re-vinculación)
	slog.Info("🛠️ Starting Deep Artist Metadata Repair...")
	rows, err := dbase.Conn.Query("SELECT id, artist FROM tracks")
	if err == nil {
		defer rows.Close()
		count := 0
		for rows.Next() {
			var trackID, rawArtist string
			if err := rows.Scan(&trackID, &rawArtist); err == nil {
				// Limpiar nombre y generar ID normalizado
				cleanArtist := utils.SanitizeString(rawArtist)
				artistID := utils.GenerateID(utils.NormalizeArtist(cleanArtist))
				
				// Asegurar que existe el artista con el nombre limpio
				_, _ = dbase.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistID, cleanArtist)
				
				// Actualizar la canción para que apunte al ID correcto y tenga el nombre limpio
				_, _ = dbase.Conn.Exec("UPDATE tracks SET artist = ? WHERE id = ?", cleanArtist, trackID)
				count++
			}
		}
		slog.Info("✅ Deep Artist Repair completed", "processed_tracks", count)
	}

	// 2. RECONSTRUCCIÓN DEL ÍNDICE DE BÚSQUEDA (FTS5 usa sintaxis especial)
	_, err = dbase.Conn.Exec("INSERT INTO search_idx(search_idx) VALUES('rebuild')")

	if err != nil {
		slog.Error("Library resync failed", "error", err)
		http.Error(w, "Resync failed: " + err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success", "message":"Library metadata repaired and index rebuilt"}`))
}

func (s *Server) handleLibraryCleanPlaylists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	
	dbase := s.getDB(r)
	res, err := dbase.Conn.Exec("DELETE FROM playlist_tracks WHERE track_id NOT IN (SELECT id FROM tracks)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	removed, _ := res.RowsAffected()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "removed":%d}`, removed)))
}

func (s *Server) handleLibraryDuplicates(w http.ResponseWriter, r *http.Request) {
	dups, err := s.getDB(r).GetDuplicates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	groups := make(map[string][]models.Track)
	for _, t := range dups {
		key := fmt.Sprintf("%s|%s|%s", strings.ToLower(t.Artist), strings.ToLower(t.Album), strings.ToLower(t.Title))
		if t.Artist == "" && t.Album == "" && t.Title == "" && t.FileHash != "" { key = "hash:" + t.FileHash }
		groups[key] = append(groups[key], t)
	}
	var result [][]models.Track
	for _, g := range groups {
		if len(g) > 1 { result = append(result, g) }
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleLibraryOrphans(w http.ResponseWriter, r *http.Request) {
	orphans, err := s.getDB(r).GetOrphanedTracks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orphans)
}

func (s *Server) handleLibraryLowQuality(w http.ResponseWriter, r *http.Request) {
	thresholdStr := r.URL.Query().Get("bitrate")
	threshold := 128
	if thresholdStr != "" {
		if val, err := strconv.Atoi(thresholdStr); err == nil { threshold = val }
	}
	tracks, err := s.getDB(r).GetLowQualityTracks(threshold)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleLibraryMissingTags(w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT t.id, t.title, COALESCE(t.genre, ''), ar.name as artist, al.title as album, al.year
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists ar ON al.artist_id = ar.id
		WHERE t.status = 'active' AND (
			(t.genre IS NULL OR t.genre = '' OR t.genre = 'Unknown') OR
			(ar.name = 'Unknown Artist') OR
			(al.title = 'Unknown Album')
		)
		LIMIT 500
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Genre, &t.Artist, &t.Album, &t.Year); err == nil { tracks = append(tracks, t) }
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleLibraryAllCovers(w http.ResponseWriter, r *http.Request) {
	dbase := s.getDB(r)
	var result []map[string]interface{}

	// 1. Missing covers
	rows, _ := dbase.Conn.Query(`
		SELECT al.id, al.title, ar.name as artist, t.path, 0, 0, 0
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON t.album_id = al.id
		WHERE t.status = 'active' AND (al.cover_width IS NULL OR al.cover_width = 0)
		GROUP BY al.id`)
	for rows != nil && rows.Next() {
		var id, title, artist, path string
		var wd, ht, sz int
		rows.Scan(&id, &title, &artist, &path, &wd, &ht, &sz)
		result = append(result, map[string]interface{}{"id": id, "title": title, "artist": artist, "path": path, "type": "missing"})
	}
	if rows != nil { rows.Close() }

	// 2. Small covers
	rows, _ = dbase.Conn.Query(`
		SELECT al.id, al.title, ar.name as artist, t.path, al.cover_width, al.cover_height, al.cover_size
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON t.album_id = al.id
		WHERE t.status = 'active' AND al.cover_width > 0 AND (al.cover_width < 300 OR al.cover_height < 300)
		GROUP BY al.id`)
	for rows != nil && rows.Next() {
		var id, title, artist, path string
		var wd, ht, sz int
		rows.Scan(&id, &title, &artist, &path, &wd, &ht, &sz)
		result = append(result, map[string]interface{}{"id": id, "title": title, "artist": artist, "path": path, "width": wd, "height": ht, "size": sz, "type": "small"})
	}
	if rows != nil { rows.Close() }

	// 3. Huge covers
	rows, _ = dbase.Conn.Query(`
		SELECT al.id, al.title, ar.name as artist, t.path, al.cover_width, al.cover_height, al.cover_size
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON t.album_id = al.id
		WHERE t.status = 'active' AND (al.cover_width > 1500 OR al.cover_height > 1500 OR al.cover_size > 2000000)
		GROUP BY al.id`)
	for rows != nil && rows.Next() {
		var id, title, artist, path string
		var wd, ht, sz int
		rows.Scan(&id, &title, &artist, &path, &wd, &ht, &sz)
		result = append(result, map[string]interface{}{"id": id, "title": title, "artist": artist, "path": path, "width": wd, "height": ht, "size": sz, "type": "huge"})
	}
	if rows != nil { rows.Close() }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleOrganizeLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	user, _ := r.Context().Value(userKey).(*db.User)
	if status, ok := s.organizeStatuses.Load(user.Username); ok && status.(*OrganizeStatus).IsRunning {
		http.Error(w, "Organization already in progress", http.StatusConflict)
		return
	}
	dbase := s.getDB(r)
	var total int
	dbase.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&total)
	status := &OrganizeStatus{ IsRunning: true, Total: total, Processed: 0, Current: "Starting...", Errors: []string{} }
	s.organizeStatuses.Store(user.Username, status)
	go func() {
		rows, err := dbase.Conn.Query("SELECT id, title, path FROM tracks")
		if err != nil { status.IsRunning = false; return }
		defer rows.Close()
		editor := tags.NewEditor(dbase)
		for rows.Next() {
			var id, title, path string
			if err := rows.Scan(&id, &title, &path); err == nil {
				status.Current = title
				_, err := editor.OrganizeFile(id, s.Config.MusicPath, s.Config.RenamePattern)
				if err != nil { status.Errors = append(status.Errors, fmt.Sprintf("Failed to organize %s: %v", title, err)) }
				status.Processed++
			}
		}
		status.IsRunning = false; status.Current = "Done"
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleOrganizeLibraryPreview(w http.ResponseWriter, r *http.Request) {
	dbase := s.getDB(r)
	rows, _ := dbase.Conn.Query("SELECT id, title, path FROM tracks LIMIT 50") 
	defer rows.Close()
	previews := []map[string]string{}
	editor := tags.NewEditor(dbase)
	for rows.Next() {
		var id, title, oldPath string
		if err := rows.Scan(&id, &title, &oldPath); err == nil {
			newPath, _ := editor.OrganizeFilePreview(id, s.Config.MusicPath, s.Config.RenamePattern)
			previews = append(previews, map[string]string{"id": id, "title": title, "oldPath": oldPath, "newPath": newPath})
		}
	}
	json.NewEncoder(w).Encode(previews)
}

func (s *Server) handleOrganizeStatus(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	status, ok := s.organizeStatuses.Load(user.Username)
	if !ok {
		json.NewEncoder(w).Encode(OrganizeStatus{IsRunning: false})
		return
	}
	json.NewEncoder(w).Encode(status)
}

func (s *Server) EnrichArtists() {
	go func() {
		slog.Info("🚀 Background artist enrichment started")
		now := time.Now().Unix()
		dayAgo := now - 86400

		query := `
			SELECT id, name 
			FROM artists 
			WHERE ((image_url IS NULL OR image_url = '') OR (bio IS NULL OR bio = ''))
			  AND (last_enrich_attempt < ?)`
		
		rows, err := s.MasterDB.Conn.Query(query, dayAgo)
		if err != nil {
			slog.Error("Failed to query artists for enrichment", "error", err)
			return
		}
		defer rows.Close()

		type artistTask struct{ ID, Name string }
		var tasks []artistTask
		for rows.Next() {
			var a artistTask
			if err := rows.Scan(&a.ID, &a.Name); err == nil {
				tasks = append(tasks, a)
			}
		}

		if len(tasks) == 0 {
			slog.Info("✨ No artists need enrichment at this moment")
			return
		}

		jobs := make(chan artistTask, len(tasks))
		var wg sync.WaitGroup
		numWorkers := 5
		if len(tasks) < numWorkers { numWorkers = len(tasks) }

		for w := 1; w <= numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for a := range jobs {
					lowerName := strings.ToLower(a.Name)
					if lowerName == "various artists" || lowerName == "unknown artist" || lowerName == "unknown" {
						continue
					}
					
					_, _ = s.MasterDB.Conn.Exec("UPDATE artists SET last_enrich_attempt = ? WHERE id = ?", now, a.ID)

					ext, err := s.externalMeta.GetArtistInfo(a.Name)
					if err == nil && ext != nil {
						tagsStr := strings.Join(ext.Tags, ",")
						similarStr := strings.Join(ext.Similar, ",")
						_, err = s.MasterDB.Conn.Exec("UPDATE artists SET image_url = ?, bio = ?, tags = ?, similar = ?, last_enrich_attempt = ? WHERE id = ?", 
							ext.Image, ext.Biography, tagsStr, similarStr, now, a.ID)
						
						if err == nil {
							slog.Info("✨ Artist enriched successfully", "name", a.Name, "hasImage", ext.Image != "")
						}
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}

		for _, task := range tasks {
			jobs <- task
		}
		close(jobs)
		wg.Wait()

		slog.Info("✅ Background artist enrichment completed", "processed", len(tasks))
		utils.PurgeMemory(s.MasterDB.Conn)
	}()
}

func (s *Server) HandleFileEvent(path string) {
	if _, err := os.Stat(path); err != nil { return }
	sc := scanner.NewScanner(s.MasterDB)
	sc.ParseAndSaveFile(path)
	s.updateInboxCount()
}

func (s *Server) HandleFileDelete(path string) {
	s.MasterDB.Conn.Exec("DELETE FROM tracks WHERE path = ?", path)
	s.updateInboxCount()
}

func (s *Server) TriggerGlobalScan() {
	status := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store("global", status)
	sc := scanner.NewScanner(s.MasterDB)
	sc.OnProgress = func(path string, count int) {
		status.Processed = count
		s.scanStatuses.Store("global", status)
	}
	go func() {
		defer func() {
			status.Scanning = false
			s.scanStatuses.Store("global", status)
		}()
		sc.ScanDirectory(s.Config.MusicPath)
	}()
}
