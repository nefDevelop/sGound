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
		SELECT ar.id, ar.name, COUNT(DISTINCT al.id) as album_count, COALESCE(ar.image_url, '')
		FROM artists ar
		JOIN albums al ON ar.id = al.artist_id
		JOIN tracks t ON al.id = t.album_id
		%s
		GROUP BY ar.id, ar.name, ar.image_url
		ORDER BY ar.name`, whereClause)

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
	if strings.HasSuffix(r.URL.Path, "/albums") {
		artistID := s.extractID(r, "/api/artists/")
		s.handleGetArtistAlbums(artistID, w, r)
		return
	}
	if r.Method == http.MethodGet {
		artistID := s.extractID(r, "/api/artists/")
		s.handleGetArtist(artistID, w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetArtist(artistID string, w http.ResponseWriter, r *http.Request) {
	var name string
	var cachedImage, cachedBio, cachedTags, cachedSimilar sql.NullString
	err := s.getDB(r).Conn.QueryRow("SELECT name, image_url, bio, tags, similar FROM artists WHERE id = ?", artistID).Scan(&name, &cachedImage, &cachedBio, &cachedTags, &cachedSimilar)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var albumCount, trackCount int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT id) FROM albums WHERE artist_id = ?", artistID).Scan(&albumCount)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks t JOIN albums a ON t.album_id = a.id WHERE a.artist_id = ? AND t.status = 'active'", artistID).Scan(&trackCount)

	biography := cachedBio.String
	externalImage := cachedImage.String
	tags := strings.Split(cachedTags.String, ",")
	similar := strings.Split(cachedSimilar.String, ",")

	if cachedTags.String == "" {
		tags = []string{}
	}
	if cachedSimilar.String == "" {
		similar = []string{}
	}

	isDisambiguation := strings.Contains(biography, "puede referirse a") || strings.Contains(biography, "may refer to")
	if biography == "" || externalImage == "" || len(tags) == 0 || isDisambiguation {
		ext, _ := s.externalMeta.GetArtistInfo(name)
		if ext != nil {
			biography = ext.Biography
			externalImage = ext.Image
			tags = ext.Tags
			similar = ext.Similar
			tagsStr := strings.Join(tags, ",")
			similarStr := strings.Join(similar, ",")
			_, _ = s.getDB(r).Conn.Exec("UPDATE artists SET bio = ?, image_url = ?, tags = ?, similar = ? WHERE id = ?",
				biography, externalImage, tagsStr, similarStr, artistID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"id": artistID, "name": name, "albumCount": albumCount, "trackCount": trackCount,
		"biography": biography, "imageUrl": externalImage,
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

		// Normalización agresiva para detectar duplicados (ej: Aurora 2020 al final)
		normTitle := strings.ToLower(title)
		normTitle = strings.TrimSpace(normTitle)
		// Si el título termina en el nombre del artista o en un año de 4 dígitos, lo limpiamos para comparar
		artistLower := strings.ToLower(artist)
		if strings.HasSuffix(normTitle, artistLower) {
			normTitle = strings.TrimSpace(strings.TrimSuffix(normTitle, artistLower))
		}
		// Limpiar año al final (ej: "2020")
		words := strings.Fields(normTitle)
		if len(words) > 1 {
			last := words[len(words)-1]
			if len(last) == 4 {
				if _, err := strconv.Atoi(last); err == nil {
					normTitle = strings.TrimSpace(strings.TrimSuffix(normTitle, last))
				}
			}
		}

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
		       COALESCE(t.format, ''), COALESCE(ut.rating, 0) as rating, COALESCE(t.instrumental, 0)`
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
			var trackNum, year, rating, instrumental int
			var duration float64
			if err := rows.Scan(&id, &title, &trackNum, &duration, &genre, &artist, &album, &year, &format, &rating, &instrumental); err != nil {
				continue
			}
			enc.Encode(map[string]interface{}{
				"id": id, "title": title, "trackNum": trackNum, "duration": duration, "genre": genre,
				"artist": artist, "album": album, "year": year, "format": format, "rating": rating, "instrumental": instrumental == 1,
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
