package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"sgound/internal/db"
	"sgound/internal/models"
	"sgound/internal/utils"
)

func (s *Server) handleGetArtists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	user, _ := r.Context().Value(userKey).(*db.User)
	whereClause := "WHERE t.status = 'active'"
	args := []interface{}{}

	pathFilter, pathArgs := s.getPathFilter(user)
	if pathFilter != "" {
		whereClause += " AND " + pathFilter
		args = append(args, pathArgs...)
	}

	// AUTO-SYNC (Moved to background to avoid blocking initial list load)
	dbase := s.getDB(r)
	go func(db *db.MasterDatabase) {
		repairQuery := `
			SELECT DISTINCT artist
			FROM tracks
			WHERE artist IS NOT NULL AND artist != '' AND artist != 'Unknown'
			  AND artist NOT IN (SELECT name FROM artists)`
		rows, err := db.Conn.Query(repairQuery)
		if err != nil {
			return
		}
		var missing []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				missing = append(missing, name)
			}
		}
		rows.Close()
		for _, name := range missing {
			artistID := utils.GenerateID(utils.NormalizeArtist(name))
			_, _ = db.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistID, name)
			slog.Info("🔧 Background repaired missing artist entry", "name", name)
		}
	}(dbase)

	// Pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 0
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	query := `
		SELECT 
			ar.id, 
			ar.name, 
			(SELECT COUNT(*) FROM albums WHERE artist_id = ar.id) as album_count,
			COALESCE(ar.image_url, '')
		FROM artists ar
		WHERE EXISTS (
			SELECT 1 FROM tracks t 
			JOIN albums al ON t.album_id = al.id 
			WHERE al.artist_id = ar.id AND t.status IN ('active', 'pending', 'raw')
		) OR EXISTS (
			/* Caso especial para canciones sin álbum formal aún */
			SELECT 1 FROM tracks t WHERE t.artist = ar.name AND t.status IN ('active', 'pending', 'raw')
		)
		ORDER BY ar.name`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	rows, err := s.getDB(r).Conn.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("["))
	enc := json.NewEncoder(w)
	first := true

	for rows.Next() {
		if !first {
			w.Write([]byte(","))
		}
		first = false
		var a models.Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.ImageURL); err != nil {
			continue
		}
		enc.Encode(a)
	}
	w.Write([]byte("]"))
}

func (s *Server) handleArtistActions(w http.ResponseWriter, r *http.Request) {
	// Extraer ID base sin sufijos
	rawID := s.extractID(r, "/api/artists/")

	if strings.HasSuffix(r.URL.Path, "/albums") {
		artistID := strings.TrimSuffix(rawID, "/albums")
		s.handleGetArtistAlbums(artistID, w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/image") {
		artistID := strings.TrimSuffix(rawID, "/image")
		s.handleGetArtistImageWithID(artistID, w, r)
		return
	}
	if r.Method == http.MethodGet {
		s.handleGetArtist(rawID, w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetArtist(artistID string, w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	var name string
	var cachedImage, cachedSource, cachedBio, cachedTags, cachedSimilar sql.NullString
	err := s.getDB(r).Conn.QueryRow("SELECT name, image_url, image_source, bio, tags, similar FROM artists WHERE id = ?", artistID).Scan(&name, &cachedImage, &cachedSource, &cachedBio, &cachedTags, &cachedSimilar)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	biography := cachedBio.String
	externalImage := cachedImage.String
	imageSource := cachedSource.String
	tags := strings.Split(cachedTags.String, ",")
	similar := strings.Split(cachedSimilar.String, ",")

	if cachedTags.String == "" {
		tags = []string{}
	}
	if cachedSimilar.String == "" {
		similar = []string{}
	}

	isDisambiguation := strings.Contains(biography, "puede referirse a") || strings.Contains(biography, "may refer to")
	if biography == "" || externalImage == "" || len(tags) == 0 || isDisambiguation || imageSource == "" {
		// Launch enrichment in background to avoid blocking the UI
		go func(id, n string, db *db.MasterDatabase) {
			ext, _ := s.externalMeta.GetArtistInfo(n)
			if ext != nil {
				tagsStr := strings.Join(ext.Tags, ",")
				similarStr := strings.Join(ext.Similar, ",")
				_, _ = db.Conn.Exec("UPDATE artists SET bio = ?, image_url = ?, image_source = ?, tags = ?, similar = ? WHERE id = ?",
					ext.Biography, ext.Image, ext.Source, tagsStr, similarStr, id)
				slog.Info("✨ Artist metadata enriched in background", "name", n)
			}
		}(artistID, name, s.getDB(r))
	}

	var trackCount int
	_ = s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT t.id) FROM tracks t JOIN albums a ON t.album_id = a.id WHERE a.artist_id = ? AND t.status = 'active'", artistID).Scan(&trackCount)

	// 3. GET ALBUM COUNT (Optimized: Count distinct normalized titles)
	whereClause := "WHERE al.artist_id = ? AND t.status = 'active'"
	args := []interface{}{artistID}
	pathFilter, pathArgs := s.getPathFilter(user)
	if pathFilter != "" {
		whereClause += " AND " + pathFilter
		args = append(args, pathArgs...)
	}

	var albumCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT LOWER(TRIM(al.title)))
		FROM albums al
		JOIN tracks t ON al.id = t.album_id
		%s`, whereClause)
	_ = s.getDB(r).Conn.QueryRow(countQuery, args...).Scan(&albumCount)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"id": artistID, "name": name, "albumCount": albumCount, "trackCount": trackCount,
		"biography": biography, "imageUrl": externalImage, "imageSource": imageSource,
		"tags": tags, "similar": similar,
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetArtistAlbums(artistID string, w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	whereClause := "WHERE al.artist_id = ? AND t.status = 'active'"
	args := []interface{}{artistID}

	pathFilter, pathArgs := s.getPathFilter(user)
	if pathFilter != "" {
		whereClause += " AND " + pathFilter
		args = append(args, pathArgs...)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT al.id, al.title, COALESCE(al.year, 0), ar.name as artist, COALESCE(al.cover_id, '') as cover_id
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON al.id = t.album_id
		%s
		ORDER BY al.year DESC`, whereClause)

	rows, err := s.getDB(r).Conn.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type albumGroup struct {
		best map[string]interface{}
	}
	groups := make(map[string]*albumGroup)

	for rows.Next() {
		var id, title, artist, coverId string
		var year int
		if err := rows.Scan(&id, &title, &year, &artist, &coverId); err != nil {
			continue
		}
		
		// Fallback for coverId: use first track if album cover is missing
		if coverId == "" {
			_ = s.getDB(r).Conn.QueryRow("SELECT id FROM tracks WHERE album_id = ? LIMIT 1", id).Scan(&coverId)
		}

		// Normalización agresiva para detectar duplicados
		normTitle := s.normalizeAlbumTitle(title, artist)
		albumData := map[string]interface{}{
			"id": id, "title": title, "year": year, "artist": artist, "coverId": coverId,
		}

		if existing, ok := groups[normTitle]; ok {
			// Si el nuevo tiene año y el viejo no, o si el nuevo tiene un título más corto (limpio), preferirlo
			existingYear := existing.best["year"].(int)
			if (year > 0 && existingYear == 0) || (len(title) < len(existing.best["title"].(string))) {
				existing.best = albumData
			}
		} else {
			groups[normTitle] = &albumGroup{best: albumData}
		}
	}

	var albums []map[string]interface{}
	for _, g := range groups {
		albums = append(albums, g.best)
	}

	// Volver a ordenar por año DESC después de la consolidación
	sort.Slice(albums, func(i, j int) bool {
		yi := albums[i]["year"].(int)
		yj := albums[j]["year"].(int)
		if yi != yj { return yi > yj }
		return albums[i]["title"].(string) < albums[j]["title"].(string)
	})

	if len(albums) == 0 {
		albums = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleGetAlbums(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	whereClause := "WHERE t.status = 'active'"
	args := []interface{}{}

	pathFilter, pathArgs := s.getPathFilter(user)
	if pathFilter != "" {
		whereClause += " AND " + pathFilter
		args = append(args, pathArgs...)
	}

	// Pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 0
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT al.id, al.title, COALESCE(al.year, 0), ar.name as artist, 
		       (SELECT t2.id FROM tracks t2 WHERE t2.album_id = al.id AND t2.status = 'active' LIMIT 1) as sample_track_id
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON al.id = t.album_id
		%s
		ORDER BY al.title`, whereClause)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	rows, err := s.getDB(r).Conn.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var albums []map[string]interface{}
	for rows.Next() {
		var id, title, artist, sampleTrackID string
		var year int
		if err := rows.Scan(&id, &title, &year, &artist, &sampleTrackID); err != nil {
			continue
		}
		albums = append(albums, map[string]interface{}{
			"id": id, "title": title, "year": year, "artist": artist, "coverId": sampleTrackID,
		})
	}

	if len(albums) == 0 {
		albums = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleAlbumActions(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/tracks") {
		albumID := s.extractID(r, "/api/albums/")
		s.handleGetAlbumTracks(albumID, w, r)
		return
	}
	if r.Method == http.MethodGet {
		albumID := s.extractID(r, "/api/albums/")
		s.handleGetAlbum(albumID, w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetAlbum(albumID string, w http.ResponseWriter, r *http.Request) {
	var title, artist string
	var year int
	err := s.getDB(r).Conn.QueryRow(`
		SELECT a.title, ar.name, COALESCE(a.year, 0) 
		FROM albums a 
		JOIN artists ar ON a.artist_id = ar.id 
		WHERE a.id = ?`, albumID).Scan(&title, &artist, &year)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var coverId string
	s.getDB(r).Conn.QueryRow("SELECT id FROM tracks WHERE album_id = ? LIMIT 1", albumID).Scan(&coverId)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": albumID, "title": title, "artist": artist, "year": year, "coverId": coverId,
	})
}

func (s *Server) handleGetAlbumTracks(albumID string, w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	whereClause := "WHERE t.album_id = ? AND t.status = 'active'"
	args := []interface{}{albumID}

	pathFilter, pathArgs := s.getPathFilter(user)
	if pathFilter != "" {
		whereClause += " AND " + pathFilter
		args = append(args, pathArgs...)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.track_num, t.duration, t.genre, COALESCE(t.artist, a.name) as artist, al.title as album
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		%s
		ORDER BY t.track_num`, whereClause)

	rows, err := s.getDB(r).Conn.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tracks []map[string]interface{}
	for rows.Next() {
		var id, title, genre, artist, album string
		var trackNum int
		var duration float64
		if err := rows.Scan(&id, &title, &trackNum, &duration, &genre, &artist, &album); err != nil {
			slog.Error("Scan failed in handleGetAlbumTracks", "error", err)
			continue
		}
		tracks = append(tracks, map[string]interface{}{
			"id": id, "title": title, "trackNum": trackNum, "duration": duration, "genre": genre, "artist": artist, "album": album,
		})
	}

	if len(tracks) == 0 {
		tracks = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleGetTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	user, _ := r.Context().Value(userKey).(*db.User)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 1000 
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	minimal := r.URL.Query().Get("minimal") == "true"

	whereClause := "WHERE 1=1"
	args := []interface{}{}

	pathFilter, pathArgs := s.getPathFilter(user)
	if pathFilter != "" {
		whereClause += " AND " + pathFilter
		args = append(args, pathArgs...)
	}

	if status != "all" {
		statusList := strings.Split(status, ",")
		if len(statusList) > 1 {
			placeholders := make([]string, len(statusList))
			for i := range statusList {
				placeholders[i] = "?"
				args = append(args, statusList[i])
			}
			whereClause += " AND t.status IN (" + strings.Join(placeholders, ",") + ")"
		} else {
			whereClause += " AND t.status = ?"
			args = append(args, status)
		}
	}

	genre := r.URL.Query().Get("genre")
	if genre != "" {
		whereClause += " AND t.genre = ?"
		args = append(args, genre)
	}

	artistId := r.URL.Query().Get("artistId")
	if artistId != "" {
		whereClause += " AND a.id = ?"
		args = append(args, artistId)
	}

	trackId := r.URL.Query().Get("id")
	if trackId != "" {
		whereClause += " AND t.id = ?"
		args = append(args, trackId)
	}

	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM tracks t
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN artists a ON al.artist_id = a.id
		%s`, whereClause)
	_ = s.getDB(r).Conn.QueryRow(countQuery, args...).Scan(&totalCount)

	selectFields := `t.id, t.title, COALESCE(t.track_num, 0), COALESCE(t.disc_num, 0), COALESCE(t.duration, 0), COALESCE(t.genre, ''), 
		       COALESCE(t.artist, a.name, 'Artist Desconocido') as artist, COALESCE(al.title, 'Álbum Desconocido') as album, COALESCE(al.year, 0) as year,
		       COALESCE(t.size, 0), COALESCE(t.format, ''), COALESCE(t.bit_rate, 0), COALESCE(t.sample_rate, 0),
		       COALESCE(t.composer, ''), COALESCE(t.comment, ''), t.status, 
		       COALESCE(ut.rating, 0) as rating, 
		       COALESCE(t.instrumental, 0), COALESCE(t.track_total, 0), COALESCE(t.disc_total, 0),
		       COALESCE(ut.play_count, 0) as play_count`

	if minimal {
		selectFields = `t.id, t.title, COALESCE(t.track_num, 0), COALESCE(t.duration, 0), COALESCE(t.genre, ''), 
		       COALESCE(t.artist, a.name, 'Artist Desconocido') as artist, COALESCE(al.title, 'Álbum Desconocido') as album, COALESCE(al.year, 0) as year,
		       COALESCE(t.format, ''), COALESCE(ut.rating, 0) as rating, COALESCE(t.instrumental, 0),
		       COALESCE(t.bit_rate, 0), COALESCE(t.sample_rate, 0)`
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM tracks t
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN artists a ON al.artist_id = a.id
		LEFT JOIN user_tracks ut ON t.id = ut.track_id AND ut.username = ?
		%s
		ORDER BY t.path ASC, t.track_num ASC, t.id ASC
		LIMIT ? OFFSET ?`, selectFields, whereClause)

	finalArgs := append([]interface{}{user.Username}, args...)
	finalArgs = append(finalArgs, limit, offset)

	rows, err := s.getDB(r).Conn.Query(query, finalArgs...)
	if err != nil {
		slog.Error("Database query failed in handleGetTracks", "error", err, "query", query)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("X-Total-Count", strconv.Itoa(totalCount))
	w.Header().Set("Content-Type", "application/json")
	
	// Start JSON array
	w.Write([]byte("["))
	
	enc := json.NewEncoder(w)
	first := true

	for rows.Next() {
		if !first {
			w.Write([]byte(","))
		}
		first = false

		if minimal {
			var id, title, genre, artist, album, format string
			var trackNum, year, rating, instrumental, bitRate, sampleRate int
			var duration float64
			if err := rows.Scan(&id, &title, &trackNum, &duration, &genre, &artist, &album, &year, &format, &rating, &instrumental, &bitRate, &sampleRate); err != nil {
				continue
			}
			enc.Encode(map[string]interface{}{
				"id": id, "title": title, "trackNum": trackNum, "duration": duration, "genre": genre,
				"artist": artist, "album": album, "year": year, "format": format, "rating": rating, "instrumental": instrumental == 1,
				"bit_rate": bitRate, "sample_rate": sampleRate,
			})
		} else {
			var id, title, genre, artist, album, format, composer, comment, tStatus string
			var trackNum, discNum, year, bitRate, sampleRate, rating, instrumental, trackTotal, discTotal, playCount int
			var duration float64
			var size int64
			if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment, &tStatus, &rating, &instrumental, &trackTotal, &discTotal, &playCount); err != nil {
				continue
			}
			enc.Encode(map[string]interface{}{
				"id": id, "title": title, "trackNum": trackNum, "discNum": discNum,
				"duration": duration, "genre": genre, "artist": artist, "album": album, "year": year,
				"size": size, "format": format, "bitrate": bitRate, "sampleRate": sampleRate,
				"composer": composer, "comment": comment, "status": tStatus, "rating": rating,
				"instrumental": instrumental == 1, "trackTotal": trackTotal, "discTotal": discTotal,
				"play_count": playCount,
			})
		}
	}
	
	// Close JSON array
	w.Write([]byte("]"))
}

func (s *Server) handleLibraryMetadata(w http.ResponseWriter, r *http.Request) {
	decadeRows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT (al.year / 10) * 10 as decade 
		FROM albums al 
		JOIN tracks t ON al.id = t.album_id
		WHERE al.year > 0 AND t.status = 'active'
		ORDER BY decade DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer decadeRows.Close()

	var decades []int
	for decadeRows.Next() {
		var d int
		decadeRows.Scan(&d)
		decades = append(decades, d)
	}

	genreRows, err := s.getDB(r).Conn.Query("SELECT DISTINCT genre FROM tracks WHERE genre IS NOT NULL AND genre != '' AND status = 'active' ORDER BY genre ASC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer genreRows.Close()

	var genres []string
	for genreRows.Next() {
		var g string
		genreRows.Scan(&g)
		genres = append(genres, g)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decades": decades,
		"genres":  genres,
	})
}

func (s *Server) normalizeAlbumTitle(title, artist string) string {
	norm := strings.ToLower(title)
	artistLower := strings.ToLower(artist)

	for {
		changed := false
		norm = strings.TrimSpace(norm)
		// Quitar puntuación común al final (-, _, .)
		norm = strings.TrimRight(norm, " -_.")

		// 1. Quitar año al final (4 dígitos)
		words := strings.Fields(norm)
		if len(words) > 1 {
			last := words[len(words)-1]
			if len(last) == 4 {
				if _, err := strconv.Atoi(last); err == nil {
					norm = strings.TrimSpace(strings.TrimSuffix(norm, last))
					changed = true
				}
			}
		}

		// 2. Quitar nombre del artista al final (con artista vacío
		// HasSuffix siempre es true y el bucle no termina).
		if artistLower != "" && strings.HasSuffix(norm, artistLower) {
			norm = strings.TrimSpace(strings.TrimSuffix(norm, artistLower))
			changed = true
		}

		if !changed {
			break
		}
	}
	return strings.TrimSpace(norm)
}
