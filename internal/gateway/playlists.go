package gateway

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sgound/internal/utils"
)

type playlistPayload struct {
	XMLName   xml.Name      `xml:"playlist" json:"-"`
	ID        string        `xml:"id,attr" json:"id"`
	Name      string        `xml:"name,attr" json:"name"`
	Owner     string        `xml:"owner,attr,omitempty" json:"owner,omitempty"`
	SongCount int           `xml:"songCount,attr" json:"songCount"`
	Duration  int           `xml:"duration,attr" json:"duration"`
	Entries   []songPayload `xml:"entry" json:"entry,omitempty"`
}

type playlistsResp struct {
	XMLName   xml.Name          `xml:"playlists" json:"-"`
	Playlists []playlistPayload `xml:"playlist" json:"playlist"`
}

// playlistEntries devuelve las canciones: manuales por rowid o smart por ruleSQL.
func (g *SubsonicGateway) playlistEntries(username, playlistID string, isSmart int, ruleSQL string) ([]songPayload, error) {
	if isSmart == 1 && ruleSQL != "" {
		return g.querySongs(`WHERE t.id IN (`+ruleSQL+`) AND t.status = 'active'`,
			`ORDER BY t.title LIMIT 500`)
	}
	return g.querySongs(`WHERE t.id IN (
			SELECT track_id FROM playlist_tracks WHERE playlist_id = ?
		) AND t.status = 'active'`,
		`ORDER BY (SELECT rowid FROM playlist_tracks WHERE playlist_id = ? AND track_id = t.id)`, playlistID, playlistID)
}

func (g *SubsonicGateway) playlistSummary(username, id string) (playlistPayload, error) {
	var p playlistPayload
	var isSmart int
	var ruleSQL string
	err := g.DB.Conn.QueryRow(
		`SELECT id, name, username, is_smart, COALESCE(rule_sql, '')
		 FROM playlists WHERE id = ? AND username = ?`, id, username).
		Scan(&p.ID, &p.Name, &p.Owner, &isSmart, &ruleSQL)
	if err != nil {
		return p, err
	}
	entries, err := g.playlistEntries(username, id, isSmart, ruleSQL)
	if err != nil {
		return p, err
	}
	p.Entries = entries
	if p.Entries == nil {
		p.Entries = []songPayload{}
	}
	p.SongCount = len(entries)
	for _, s := range entries {
		p.Duration += s.Duration
	}
	return p, nil
}

func (g *SubsonicGateway) handleGetPlaylists(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	rows, err := g.DB.Conn.Query(
		`SELECT id, name, username, is_smart, COALESCE(rule_sql, '')
		 FROM playlists WHERE username = ? ORDER BY name`, user.Username)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	defer rows.Close()
	type meta struct {
		id      string
		name    string
		owner   string
		isSmart int
		ruleSQL string
	}
	var list []meta
	for rows.Next() {
		var m meta
		if err := rows.Scan(&m.id, &m.name, &m.owner, &m.isSmart, &m.ruleSQL); err != nil {
			continue
		}
		list = append(list, m)
	}
	rows.Close()

	resp := playlistsResp{}
	for _, m := range list {
		entries, err := g.playlistEntries(user.Username, m.id, m.isSmart, m.ruleSQL)
		if err != nil {
			writeErr(w, r, errGeneric, "Error de base de datos.")
			return
		}
		var p playlistPayload
		p.ID, p.Name, p.Owner = m.id, m.name, m.owner
		p.SongCount = len(entries)
		for _, s := range entries {
			p.Duration += s.Duration
		}
		resp.Playlists = append(resp.Playlists, p)
	}
	if resp.Playlists == nil {
		resp.Playlists = []playlistPayload{}
	}
	writeOK(w, r, "playlists", resp, resp)
}

func (g *SubsonicGateway) handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	p, err := g.playlistSummary(user.Username, id)
	if err != nil {
		writeErr(w, r, errNotFound, "Playlist no encontrada.")
		return
	}
	writeOK(w, r, "playlist", p, p)
}

func appendSongs(playlistID string, ids []string, exec func(string, ...any) error) error {
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := exec(`INSERT OR IGNORE INTO playlist_tracks (playlist_id, track_id) VALUES (?, ?)`, playlistID, id); err != nil {
			return err
		}
	}
	return nil
}

func (g *SubsonicGateway) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	// Sin playlistId es creación; con playlistId es actualización (append).
	if q.Get("playlistId") == "" {
		name := strings.TrimSpace(q.Get("name"))
		if name == "" {
			writeErr(w, r, errMissingParam, "Falta el nombre.")
			return
		}
		id := strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "-" + strconv.FormatInt(time.Now().Unix(), 10)
		if err := g.DB.Conn.QueryRow(`SELECT id FROM playlists WHERE id = ?`, id).Scan(new(string)); err == nil {
			id += "-" + utils.GenerateID(name+time.Now().String())[:8]
		}
		if _, err := g.DB.Conn.Exec(
			`INSERT INTO playlists (id, username, name, description, is_smart, rule_sql) VALUES (?, ?, ?, '', 0, '')`,
			id, user.Username, name); err != nil {
			writeErr(w, r, errGeneric, "No se pudo crear.")
			return
		}
		exec := func(query string, args ...any) error {
			_, err := g.DB.Conn.Exec(query, args...)
			return err
		}
		ids := append(q["songId"], q["songIdToAdd"]...)
		if err := appendSongs(id, ids, exec); err != nil {
			writeErr(w, r, errGeneric, "No se pudieron añadir canciones.")
			return
		}
		p, err := g.playlistSummary(user.Username, id)
		if err != nil {
			writeErr(w, r, errGeneric, "Creada pero no legible.")
			return
		}
		writeOK(w, r, "playlist", p, p)
		return
	}
	g.handleUpdatePlaylist(w, r)
}

func (g *SubsonicGateway) handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	id := q.Get("playlistId")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta playlistId.")
		return
	}
	var owner string
	if err := g.DB.Conn.QueryRow(
		`SELECT username FROM playlists WHERE id = ?`, id).Scan(&owner); err != nil {
		writeErr(w, r, errNotFound, "Playlist no encontrada.")
		return
	}
	if owner != user.Username {
		writeErr(w, r, errNotAuthorized, "No es tu playlist.")
		return
	}
	if name := strings.TrimSpace(q.Get("name")); name != "" {
		if _, err := g.DB.Conn.Exec(`UPDATE playlists SET name = ? WHERE id = ?`, name, id); err != nil {
			writeErr(w, r, errGeneric, "No se pudo renombrar.")
			return
		}
	}
	if comment := q.Get("comment"); comment != "" {
		_, _ = g.DB.Conn.Exec(`UPDATE playlists SET description = ? WHERE id = ?`, comment, id)
	}
	exec := func(query string, args ...any) error {
		_, err := g.DB.Conn.Exec(query, args...)
		return err
	}
	ids := append(q["songId"], q["songIdToAdd"]...)
	if err := appendSongs(id, ids, exec); err != nil {
		writeErr(w, r, errGeneric, "No se pudieron añadir canciones.")
		return
	}
	for _, idxStr := range q["songIndexToRemove"] {
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx < 0 {
			continue
		}
		var trackID string
		err = g.DB.Conn.QueryRow(`
			SELECT track_id FROM playlist_tracks WHERE playlist_id = ?
			ORDER BY rowid LIMIT 1 OFFSET ?`, id, idx).Scan(&trackID)
		if err != nil {
			continue
		}
		_, _ = g.DB.Conn.Exec(
			`DELETE FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?`, id, trackID)
	}
	writeOK(w, r, "", nil, nil)
}

func (g *SubsonicGateway) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	res, err := g.DB.Conn.Exec(`DELETE FROM playlists WHERE id = ? AND username = ?`, id, user.Username)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, r, errNotFound, "Playlist no encontrada.")
		return
	}
	_, _ = g.DB.Conn.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, id)
	writeOK(w, r, "", nil, nil)
}

// setStarred marca/desmarca. Solo tracks de momento (id de álbum/artista -> 70).
func (g *SubsonicGateway) setStarred(w http.ResponseWriter, r *http.Request, value int) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	ids := append(append([]string{}, q["id"]...), q["albumId"]...)
	ids = append(ids, q["artistId"]...)
	if len(ids) == 0 {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	for _, id := range ids {
		var trackID string
		err := g.DB.Conn.QueryRow(
			`SELECT id FROM tracks WHERE id = ? AND status = 'active'`, id).Scan(&trackID)
		if err != nil {
			writeErr(w, r, errNotFound, fmt.Sprintf("Id no soportado o inexistente: %s (solo tracks).", id))
			return
		}
		if _, err := g.DB.Conn.Exec(`
			INSERT INTO user_tracks (username, track_id, rating, starred)
			VALUES (?, ?, 0, ?)
			ON CONFLICT(username, track_id) DO UPDATE SET starred = excluded.starred`,
			user.Username, trackID, value); err != nil {
			writeErr(w, r, errGeneric, "Error de base de datos.")
			return
		}
	}
	writeOK(w, r, "", nil, nil)
}

func (g *SubsonicGateway) handleStar(w http.ResponseWriter, r *http.Request) {
	g.setStarred(w, r, 1)
}

func (g *SubsonicGateway) handleUnstar(w http.ResponseWriter, r *http.Request) {
	g.setStarred(w, r, 0)
}

type starredResp struct {
	XMLName xml.Name      `xml:"starred" json:"-"`
	Artists []artistPayload `xml:"artist" json:"artist"`
	Albums  []albumPayload  `xml:"album" json:"album"`
	Songs   []songPayload   `xml:"song" json:"song"`
}

func (g *SubsonicGateway) handleGetStarred(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	songs, err := g.querySongs(`JOIN user_tracks ut ON ut.track_id = t.id
		WHERE ut.username = ? AND ut.starred = 1 AND t.status = 'active'`,
		`ORDER BY t.title LIMIT 500`, user.Username)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	resp := starredResp{
		Artists: []artistPayload{},
		Albums:  []albumPayload{},
		Songs:   songs,
	}
	if resp.Songs == nil {
		resp.Songs = []songPayload{}
	}
	writeOK(w, r, "starred", resp, resp)
}

func (g *SubsonicGateway) handleSetRating(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	id := q.Get("id")
	rating, err := strconv.Atoi(q.Get("rating"))
	if id == "" || err != nil || rating < 0 || rating > 5 {
		writeErr(w, r, errMissingParam, "Faltan id o rating (0-5).")
		return
	}
	var trackID string
	if err := g.DB.Conn.QueryRow(
		`SELECT id FROM tracks WHERE id = ? AND status = 'active'`, id).Scan(&trackID); err != nil {
		writeErr(w, r, errNotFound, "Canción no encontrada.")
		return
	}
	if _, err := g.DB.Conn.Exec(`
		INSERT INTO user_tracks (username, track_id, rating)
		VALUES (?, ?, ?)
		ON CONFLICT(username, track_id) DO UPDATE SET rating = excluded.rating`,
		user.Username, trackID, rating); err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	writeOK(w, r, "", nil, nil)
}

func (g *SubsonicGateway) handleScrobble(w http.ResponseWriter, r *http.Request) {
	user, ok := g.authenticate(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	id := q.Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	var trackID string
	if err := g.DB.Conn.QueryRow(
		`SELECT id FROM tracks WHERE id = ? AND status = 'active'`, id).Scan(&trackID); err != nil {
		writeErr(w, r, errNotFound, "Canción no encontrada.")
		return
	}
	// submission=false (o sin submission ni time) es solo "now playing".
	if q.Get("submission") == "false" || (q.Get("submission") == "" && q.Get("time") == "") {
		writeOK(w, r, "", nil, nil)
		return
	}
	if _, err := g.DB.Conn.Exec(`
		INSERT INTO user_tracks (username, track_id, rating, play_count, last_played)
		VALUES (?, ?, 0, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(username, track_id) DO UPDATE SET
			play_count = play_count + 1, last_played = CURRENT_TIMESTAMP`,
		user.Username, trackID); err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	writeOK(w, r, "", nil, nil)
}
