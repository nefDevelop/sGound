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

	ic := s.inboxCount.Load()
	rc := s.rawCount.Load()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scanning":   scanning,
		"processed":  processed,
		"rawCount":    rc,
		"inboxCount": ic,
		"isPaused":   isPaused,
		"ramUsedMB":  ramUsed,
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

	approvedCount := 0
	failedCount := 0
	dbase := s.getDB(r)

	for _, oldID := range data.IDs {
		var path string
		err := dbase.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", oldID).Scan(&path)
		if err != nil {
			failedCount++
			continue
		}

		if s.Watcher != nil {
			s.Watcher.MutePath(path, 15*time.Second)
		}

		newID := tags.GenerateUUID()
		editor := tags.NewEditor(dbase)

		// Deep Scan for covers during approval
		func(p string, tid string, albID string) {
			metadata, _ := tags.Open(p)
			var imgData []byte
			var mime string

			// 1. Try to find cover if missing
			if metadata == nil || metadata.Picture() == nil {
				dir := filepath.Dir(p)
				common := []string{"cover", "folder", "front", "album", "artwork"}
				entries, _ := os.ReadDir(dir)
				for _, ent := range entries {
					name := strings.ToLower(ent.Name())
					ext := filepath.Ext(name)
					if ext != ".jpg" && ext != ".jpeg" && ext != ".png" { continue }
					base := strings.TrimSuffix(name, ext)
					found := false
					for _, c := range common {
						if strings.HasPrefix(base, c) { found = true; break }
					}
					if found {
						imgData, _ = os.ReadFile(filepath.Join(dir, ent.Name()))
						mime = "image/jpeg"
						if ext == ".png" { mime = "image/png" }
						editor.EmbedCover(tid, imgData, mime)
						break
					}
				}
			}

			// 2. REGARDLESS of whether we just embedded it or it was already there, 
			// RE-READ dimensions to sync database
			metadata, _ = tags.Open(p) // Re-open to get fresh state
			if metadata != nil && metadata.Picture() != nil {
				pic := metadata.Picture()
				src, _, err := image.Decode(bytes.NewReader(pic.Data))
				if err == nil {
					w := src.Bounds().Dx()
					h := src.Bounds().Dy()
					s.MasterDB.Conn.Exec(`
						UPDATE albums 
						SET cover_width = ?, cover_height = ?, cover_size = ?, cover_format = ?
						WHERE id = (SELECT album_id FROM tracks WHERE id = ?)`, 
						w, h, len(pic.Data), pic.MIME, tid)
					slog.Info("📸 Cover metadata synchronized during approval", "track", tid, "dims", fmt.Sprintf("%dx%d", w, h))
				}
			}
		}(path, oldID, "")

		if err := tags.WritePersistentID(path, newID); err != nil {
			slog.Error("⚠️ Non-fatal: Failed to write physical SGOUND_ID, but approving in DB anyway", "path", path, "err", err)
		}

		dbSuccess := false
		for i := 0; i < 5; i++ {
			errTx := s.executeTransaction(func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE tracks SET id = ?, status = 'active' WHERE id = ?", newID, oldID)
				return err
			})
			if errTx == nil {
				dbSuccess = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if dbSuccess {
			approvedCount++
			if s.Config.AutoRename {
				editor := tags.NewEditor(dbase)
				editor.OrganizeFile(newID, s.Config.MusicPath, s.Config.RenamePattern)
			}
		} else {
			slog.Error("❌ Critical: Failed to update track status in DB", "path", path)
			failedCount++
		}
	}

	var totalActive int
	s.MasterDB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'active'").Scan(&totalActive)
	slog.Info("✅ Batch approval completed", "approved", approvedCount, "failed", failedCount, "total_active", totalActive)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "approved":%d, "total":%d}`, approvedCount, totalActive)))
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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
	s.updateInboxCount()
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
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM albums WHERE cover_width IS NULL OR cover_width = 0").Scan(&missingCovers)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM albums WHERE cover_width > 0 AND (cover_width < 300 OR cover_height < 300)").Scan(&smallCovers)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM albums WHERE cover_width > 1500 OR cover_height > 1500 OR cover_size > 2000000").Scan(&hugeCovers)
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
		WHERE al.cover_width IS NULL OR al.cover_width = 0
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
		WHERE al.cover_width > 0 AND (al.cover_width < 300 OR al.cover_height < 300)
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
		WHERE al.cover_width > 1500 OR al.cover_height > 1500 OR al.cover_size > 2000000
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
		query := `
			SELECT DISTINCT id, name 
			FROM artists 
			WHERE (image_url IS NULL OR image_url = '') OR (bio IS NULL OR bio = '')`
		rows, err := s.MasterDB.Conn.Query(query)
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
			slog.Info("✨ All artists are already enriched")
			return
		}

		// Use a worker pool of 3 to speed things up
		jobs := make(chan artistTask, len(tasks))
		var wg sync.WaitGroup
		numWorkers := 3
		if len(tasks) < numWorkers {
			numWorkers = len(tasks)
		}

		for w := 1; w <= numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for a := range jobs {
					lowerName := strings.ToLower(a.Name)
					if lowerName == "various artists" || lowerName == "unknown artist" || lowerName == "unknown" {
						continue
					}
					
					ext, err := s.externalMeta.GetArtistInfo(a.Name)
					if err == nil && ext != nil {
						tagsStr := strings.Join(ext.Tags, ",")
						similarStr := strings.Join(ext.Similar, ",")
						err = s.MasterDB.Conn.QueryRow("UPDATE artists SET image_url = ?, bio = ?, tags = ?, similar = ? WHERE id = ? RETURNING name", 
							ext.Image, ext.Biography, tagsStr, similarStr, a.ID).Scan(&a.Name)
						if err == nil {
							slog.Info("✨ Artist enriched successfully", "name", a.Name, "hasImage", ext.Image != "")
						} else {
							// Fallback if RETURNING is not supported or fails
							_, _ = s.MasterDB.Conn.Exec("UPDATE artists SET image_url = ?, bio = ?, tags = ?, similar = ? WHERE id = ?", 
								ext.Image, ext.Biography, tagsStr, similarStr, a.ID)
							slog.Info("✨ Artist enriched successfully", "id", a.ID, "hasImage", ext.Image != "")
						}
					}
					// Still sleep a bit to avoid aggressive rate-limiting
					time.Sleep(250 * time.Millisecond)
				}
			}()
		}

		for _, task := range tasks {
			slog.Debug("⏳ Queuing artist for enrichment", "name", task.Name)
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
