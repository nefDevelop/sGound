package api

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"sgound/internal/db"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing search query", http.StatusBadRequest)
		return
	}

	searchTerm := query + "*"
	likeTerm := query + "%"
	results := make(map[string]interface{})

	rows, err := s.getDB(r).Conn.Query(`
		SELECT id, title, artist, album
		FROM search_idx
		WHERE search_idx MATCH ?
		ORDER BY rank
		LIMIT 50`, searchTerm)

	tracks := []map[string]interface{}{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, artist, album string
			rows.Scan(&id, &title, &artist, &album)
			tracks = append(tracks, map[string]interface{}{"id": id, "title": title, "artist": artist, "album": album})
		}
	}
	results["tracks"] = tracks
	results["artists"] = []string{} 
	results["albums"] = []string{}

	genreRows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT genre
		FROM tracks
		WHERE genre LIKE ? AND status = 'active'
		LIMIT 10
	`, likeTerm)

	genres := []string{}
	if err == nil {
		defer genreRows.Close()
		for genreRows.Next() {
			var g string
			genreRows.Scan(&g)
			if g != "" { genres = append(genres, g) }
		}
	}
	results["genres"] = genres

	playlistRows, err := s.getDB(r).Conn.Query(`
		SELECT id, name
		FROM playlists
		WHERE name LIKE ?
		LIMIT 10
	`, likeTerm)

	playlists := []map[string]interface{}{}
	if err == nil {
		defer playlistRows.Close()
		for playlistRows.Next() {
			var id, name string
			playlistRows.Scan(&id, &name)
			playlists = append(playlists, map[string]interface{}{"id": id, "name": name})
		}
	}
	results["playlists"] = playlists

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleAutocomplete(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	q := r.URL.Query().Get("q")

	if field == "" || q == "" {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	var query string
	switch field {
	case "artist":
		query = "SELECT DISTINCT ar.name FROM artists ar JOIN albums al ON ar.id = al.artist_id JOIN tracks t ON al.id = t.album_id WHERE ar.name LIKE ? ORDER BY CASE WHEN ar.name LIKE ? THEN 0 ELSE 1 END, ar.name LIMIT 10"
	case "genre":
		query = "SELECT DISTINCT genre FROM tracks WHERE genre LIKE ? ORDER BY CASE WHEN genre LIKE ? THEN 0 ELSE 1 END, genre LIMIT 10"
	case "album":
		query = "SELECT DISTINCT al.title FROM albums al JOIN tracks t ON al.id = t.album_id WHERE al.title LIKE ? ORDER BY CASE WHEN al.title LIKE ? THEN 0 ELSE 1 END, al.title LIMIT 10"
	case "composer":
		query = "SELECT DISTINCT composer FROM tracks WHERE composer LIKE ? ORDER BY CASE WHEN composer LIKE ? THEN 0 ELSE 1 END, composer LIMIT 10"
	default:
		json.NewEncoder(w).Encode([]string{})
		return
	}

	rows, err := s.getDB(r).Conn.Query(query, "%"+q+"%", q+"%")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []string{}
	for rows.Next() {
		var val string
		rows.Scan(&val)
		results = append(results, val)
	}
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	stats := make(map[string]interface{})

	var tTracks, tAlbums, tArtists, tPlaylists, tGenres int
	pathFilter, pathArgs := s.getPathFilter(user)
	whereActive := "WHERE status = 'active'"
	if pathFilter != "" {
		whereActive += " AND " + pathFilter
	}

	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks t "+whereActive, pathArgs...).Scan(&tTracks)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT album_id) FROM tracks t "+whereActive, pathArgs...).Scan(&tAlbums)

	artistQuery := fmt.Sprintf(`SELECT COUNT(DISTINCT ar.id) FROM artists ar JOIN albums al ON ar.id = al.artist_id JOIN tracks t ON al.id = t.album_id %s AND t.status = 'active'`, "WHERE 1=1")
	if pathFilter != "" {
		artistQuery = fmt.Sprintf(`SELECT COUNT(DISTINCT ar.id) FROM artists ar JOIN albums al ON ar.id = al.artist_id JOIN tracks t ON al.id = t.album_id WHERE %s AND t.status = 'active'`, pathFilter)
	}
	s.getDB(r).Conn.QueryRow(artistQuery, pathArgs...).Scan(&tArtists)

	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM playlists WHERE username = ?", user.Username).Scan(&tPlaylists)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT genre) FROM tracks t "+whereActive+" AND genre != ''", pathArgs...).Scan(&tGenres)
	stats["totalTracks"] = tTracks
	stats["totalAlbums"] = tAlbums
	stats["totalArtists"] = tArtists
	stats["totalPlaylists"] = tPlaylists
	stats["totalGenres"] = tGenres

	var pendingCount int
	wherePending := "WHERE status = 'pending'"
	if pathFilter != "" {
		wherePending += " AND " + pathFilter
	}
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks t "+wherePending, pathArgs...).Scan(&pendingCount)
	stats["inboxCount"] = pendingCount

	gems := make([]map[string]interface{}, 0)
	whereGems := "WHERE t.status = 'active'"
	if pathFilter != "" {
		whereGems += " AND " + pathFilter
	}
	rows, err := s.getDB(r).Conn.Query(fmt.Sprintf(`
		SELECT MIN(t.id) as id, a.title, art.name as artist 
		FROM tracks t
		JOIN albums a ON t.album_id = a.id
		JOIN artists art ON a.artist_id = art.id
		LEFT JOIN user_tracks ut ON t.id = ut.track_id AND ut.username = ?
		%s
		GROUP BY a.id 
		ORDER BY COALESCE(ut.play_count, 0) ASC LIMIT 100
	`, whereGems), append([]interface{}{user.Username}, pathArgs...)...)
	if err == nil {
		defer rows.Close()
		var candidates []map[string]interface{}
		for rows.Next() {
			var id, title, artist string
			rows.Scan(&id, &title, &artist)
			candidates = append(candidates, map[string]interface{}{"id": id, "title": title, "artist": artist})
		}

		if len(candidates) > 0 {
			rand.Shuffle(len(candidates), func(i, j int) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			})
			if len(candidates) > 9 {
				gems = candidates[:9]
			} else {
				gems = candidates
			}
		}
	}
	stats["forgottenGems"] = gems

	topPlays := make([]map[string]interface{}, 0)
	rows2, err := s.getDB(r).Conn.Query(`
		SELECT t.id, t.title, a.name as artist, ut.play_count 
		FROM tracks t 
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		JOIN user_tracks ut ON t.id = ut.track_id
		WHERE ut.username = ? AND ut.play_count > 0 AND t.status = 'active'
		ORDER BY ut.play_count DESC LIMIT 10
	`, user.Username)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id, title, artist string
			var count int
			rows2.Scan(&id, &title, &artist, &count)
			topPlays = append(topPlays, map[string]interface{}{"id": id, "title": title, "artist": artist, "playCount": count})
		}
	}
	stats["mostPlayed"] = topPlays

	recentAlbums := make([]map[string]interface{}, 0)
	whereRecent := "WHERE t.status = 'active'"
	if pathFilter != "" {
		whereRecent += " AND " + pathFilter
	}
	rows3, err := s.getDB(r).Conn.Query(fmt.Sprintf(`
		SELECT a.id, a.title, art.name, MAX(t.created_at) as added_at, 
		       COALESCE(MAX(t.genre), '') as genre, MAX(t.bit_rate) as bitrate,
		       COALESCE((SELECT ut.play_count FROM user_tracks ut WHERE ut.track_id = t.id AND ut.username = ?), 0) as pc,
		       MAX(a.year) as year
		FROM albums a
		JOIN artists art ON a.artist_id = art.id
		JOIN tracks t ON a.id = t.album_id
		%s
		GROUP BY a.id, a.title, art.name
		ORDER BY added_at DESC LIMIT 6
	`, whereRecent), append([]interface{}{user.Username}, pathArgs...)...)
	if err == nil {
		defer rows3.Close()
		currentYear := time.Now().Year()
		for rows3.Next() {
			var id, title, artist, addedAt, genre string
			var br, pc, year int
			rows3.Scan(&id, &title, &artist, &addedAt, &genre, &br, &pc, &year)

			tag := "Recuerdo"
			color := "tertiary"
			g := strings.ToLower(genre)

			if pc > 30 {
				tag = "Top Hit"; color = "primary"
			} else if br > 1000000 {
				tag = "Premium"; color = "primary"
			} else if year > 0 && year < currentYear-20 {
				tag = "Nostalgia"; color = "tertiary"
			} else if strings.Contains(g, "rock") || strings.Contains(g, "pop") || strings.Contains(g, "electronic") || strings.Contains(g, "metal") {
				tag = "Energía"; color = "primary"
			} else if strings.Contains(g, "jazz") || strings.Contains(g, "classical") {
				tag = "Inspiración"; color = "secondary"
			} else if strings.Contains(g, "chill") || strings.Contains(g, "ambient") || strings.Contains(g, "lo-fi") || strings.Contains(g, "blues") {
				tag = "Chill"; color = "tertiary"
			} else if pc == 0 {
				tag = "Descubrir"; color = "secondary"
			}

			recentAlbums = append(recentAlbums, map[string]interface{}{
				"id": id, "title": title, "artist": artist, "addedAt": addedAt, "tag": tag, "tagColor": color,
			})
		}
	}
	stats["discovery"] = recentAlbums

	trends := make([]map[string]interface{}, 0)
	var totalActiveWithGenre int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'active' AND genre != ''").Scan(&totalActiveWithGenre)

	if totalActiveWithGenre > 0 {
		rows4, err := s.getDB(r).Conn.Query(`
			SELECT genre, COUNT(*) as count 
			FROM tracks 
			WHERE status = 'active' AND genre != '' 
			GROUP BY genre 
			ORDER BY count DESC LIMIT 3
		`)
		if err == nil {
			defer rows4.Close()
			for rows4.Next() {
				var g string
				var c int
				rows4.Scan(&g, &c)
				percentage := (c * 100) / totalActiveWithGenre
				trends = append(trends, map[string]interface{}{ "genre": g, "percentage": percentage })
			}
		}
	}
	stats["trends"] = trends

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
