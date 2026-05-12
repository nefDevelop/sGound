package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"sgound/internal/db"
)

func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	if r.Method == http.MethodPost {
		var body struct {
			Name    string `json:"name"`
			IsSmart bool   `json:"isSmart"`
			Rules   struct {
				Genre          string `json:"genre"`
				MinRating      int    `json:"minRating"`
				LastPlayedDays int    `json:"lastPlayedDays"`
				Instrumental   *bool  `json:"instrumental"`
			} `json:"rules"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		id := strings.ToLower(strings.ReplaceAll(body.Name, " ", "-")) + "-" + fmt.Sprintf("%d", os.Getpid())
		isSmart := 0
		if body.IsSmart { isSmart = 1 }

		ruleSQL := ""
		if body.IsSmart { ruleSQL = s.generateRuleSQL(body.Rules) }

		err := s.MasterDB.CreatePlaylist(user.Username, id, body.Name, "", isSmart, ruleSQL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	rows, err := s.getDB(r).Conn.Query("SELECT id, name, is_smart, rule_sql FROM playlists WHERE username = ?", user.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, name, ruleSQL string
		var isSmart int
		rows.Scan(&id, &name, &isSmart, &ruleSQL)

		count := 0
		if isSmart == 1 && ruleSQL != "" {
			s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM (" + ruleSQL + ")").Scan(&count)
		} else {
			s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?", id).Scan(&count)
		}

		list = append(list, map[string]interface{}{
			"id": id, "name": name, "isSmart": isSmart == 1, "trackCount": count,
		})
	}
	if list == nil { list = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) generateRuleSQL(rules struct {
	Genre          string `json:"genre"`
	MinRating      int    `json:"minRating"`
	LastPlayedDays int    `json:"lastPlayedDays"`
	Instrumental   *bool  `json:"instrumental"`
}) string {
	var parts []string
	if rules.Genre != "" {
		parts = append(parts, fmt.Sprintf("genre = '%s'", strings.ReplaceAll(rules.Genre, "'", "''")))
	}
	if rules.MinRating > 0 {
		parts = append(parts, fmt.Sprintf("rating >= %d", rules.MinRating))
	}
	if rules.LastPlayedDays > 0 {
		parts = append(parts, fmt.Sprintf("(last_played IS NULL OR last_played < datetime('now', '-%d days'))", rules.LastPlayedDays))
	}
	if rules.Instrumental != nil {
		val := 0
		if *rules.Instrumental { val = 1 }
		parts = append(parts, fmt.Sprintf("instrumental = %d", val))
	}

	if len(parts) == 0 {
		return "SELECT id FROM tracks WHERE status = 'active'"
	}

	return "SELECT id FROM tracks WHERE status = 'active' AND " + strings.Join(parts, " AND ")
}

func (s *Server) handleSmartPlaylists(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	if user == nil {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	
	typeStr := s.extractID(r, "/api/smart-playlists/")

	baseQuery := `
		SELECT t.id, t.title, t.track_num, t.disc_num, t.duration, COALESCE(t.genre, ''),
		       a.name as artist, al.title as album, COALESCE(al.year, 0) as year,
		       t.size, t.format, t.bit_rate, t.sample_rate,
		       t.composer, t.comment, t.instrumental, COALESCE(t.track_total, 0), COALESCE(t.disc_total, 0)
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id`

	whereClause := "WHERE t.status = 'active'"
	var orderClause string
	var args []interface{}

	switch typeStr {
	case "recently-played":
		baseQuery += " JOIN user_tracks ut ON t.id = ut.track_id AND ut.username = ?"
		args = append(args, user.Username)
		whereClause += " AND ut.last_played IS NOT NULL"
		orderClause = "ORDER BY ut.last_played DESC LIMIT 50"
	case "high-quality":
		whereClause += " AND (t.bit_rate >= 320000 OR t.format = 'flac' OR t.format = 'wav')"
		orderClause = "ORDER BY t.bit_rate DESC"
	case "recently-added":
		orderClause = "ORDER BY t.created_at DESC LIMIT 50"
	case "decade":
		decade := r.URL.Query().Get("value")
		if decade != "" {
			dInt, _ := strconv.Atoi(decade)
			whereClause += " AND al.year >= ? AND al.year <= ?"
			args = append(args, dInt, dInt+9)
		}
		orderClause = "ORDER BY al.year DESC, al.title, t.track_num"
	case "genre":
		genre := r.URL.Query().Get("value")
		if genre != "" {
			whereClause += " AND t.genre = ?"
			args = append(args, genre)
		}
		orderClause = "ORDER BY a.name, al.title, t.track_num"
	default:
		http.Error(w, "Unknown smart playlist type", http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf("%s %s %s", baseQuery, whereClause, orderClause)
	rows, err := s.getDB(r).Conn.Query(query, args...)
	if err != nil {
		slog.Error("SQL Error in handleSmartPlaylists", "type", typeStr, "error", err, "query", query, "args", args)
		http.Error(w, "Error executing query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tracks []map[string]interface{}
	for rows.Next() {
		var id, title, genre, artist, album, format, composer, comment string
		var trackNum, discNum, year, bitRate, sampleRate, instrumental, trackTotal, discTotal int
		var duration float64
		var size int64
		
		// Usamos Scan con punteros o COALESCE en el SQL para evitar errores con NULLs
		if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment, &instrumental, &trackTotal, &discTotal); err != nil {
			slog.Error("Scan error in handleSmartPlaylists", "err", err)
			continue
		}
		
		tracks = append(tracks, map[string]interface{}{
			"id": id, "title": title, "trackNum": trackNum, "discNum": discNum,
			"duration": duration, "genre": genre, "artist": artist, "album": album, "year": year,
			"size": size, "format": format, "bitrate": bitRate, "sampleRate": sampleRate,
			"composer": composer, "comment": comment, "instrumental": instrumental == 1,
			"trackTotal": trackTotal, "discTotal": discTotal,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handlePlaylistActions(w http.ResponseWriter, r *http.Request) {
	id := s.extractID(r, "/api/playlists/")

	if strings.HasSuffix(r.URL.Path, "/tracks") {
		switch r.Method {
		case http.MethodGet:
			s.handleGetPlaylistTracks(w, r)
		case http.MethodPost:
			var body struct {
				TrackID string `json:"trackId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				s.getDB(r).AddToPlaylist(id, body.TrackID)
			}
			w.WriteHeader(http.StatusOK)
		}
		return
	}

	if r.Method == http.MethodDelete {
		user, _ := r.Context().Value(userKey).(*db.User)
		s.getDB(r).DeletePlaylist(id, user.Username)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
		return
	}

	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func (s *Server) handleGetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/playlists/")
	id = strings.TrimSuffix(id, "/tracks")
	user, _ := r.Context().Value(userKey).(*db.User)

	var isSmart int
	var ruleSQL string
	err := s.getDB(r).Conn.QueryRow("SELECT is_smart, rule_sql FROM playlists WHERE id = ? AND username = ?", id, user.Username).Scan(&isSmart, &ruleSQL)
	if err != nil {
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	var rows *sql.Rows
	fields := "t.id, t.title, t.track_num, t.disc_num, t.duration, COALESCE(t.genre, ''), a.name, al.title, COALESCE(al.year, 0), COALESCE(ut.rating, 0), t.composer, t.comment, t.instrumental, COALESCE(t.track_total, 0), COALESCE(t.disc_total, 0)"

	if isSmart == 1 && ruleSQL != "" {
		rows, err = s.getDB(r).Conn.Query(`
			SELECT `+fields+`
			FROM tracks t
			JOIN albums al ON t.album_id = al.id
			JOIN artists a ON al.artist_id = a.id
			LEFT JOIN user_tracks ut ON t.id = ut.track_id AND ut.username = ?
			WHERE t.id IN (`+ruleSQL+`)
			ORDER BY a.name, al.title, t.track_num
		`, user.Username)
	} else {
		rows, err = s.getDB(r).Conn.Query(`
			SELECT `+fields+`
			FROM tracks t
			JOIN playlist_tracks pt ON t.id = pt.track_id
			JOIN albums al ON t.album_id = al.id
			JOIN artists a ON al.artist_id = a.id
			LEFT JOIN user_tracks ut ON t.id = ut.track_id AND ut.username = ?
			WHERE pt.playlist_id = ? AND t.status = 'active'
			ORDER BY pt.position ASC
		`, user.Username, id)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var tid, title, genre, artist, album, composer, comment string
		var trackNum, discNum, year, rating, instrumental, trackTotal, discTotal int
		var duration float64
		err := rows.Scan(&tid, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &rating, &composer, &comment, &instrumental, &trackTotal, &discTotal)
		if err != nil {
			slog.Error("Scan error in handleGetPlaylistTracks", "err", err)
			continue
		}
		list = append(list, map[string]interface{}{
			"id": tid, "title": title, "trackNum": trackNum, "discNum": discNum,
			"duration": duration, "genre": genre, "artist": artist,
			"album": album, "year": year, "rating": rating,
			"composer": composer, "comment": comment, "instrumental": instrumental == 1,
			"trackTotal": trackTotal, "discTotal": discTotal,
		})
	}
	json.NewEncoder(w).Encode(list)
}
