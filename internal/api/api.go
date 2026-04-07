package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"io/fs"
	"log/slog"

	"github.com/dhowden/tag"
	
	"sgound/internal/db"
	"sgound/internal/gateway"
	"sgound/internal/tags"
	"sgound/internal/config"
	"sgound/internal/scanner"
	"sgound/web"
)

type Server struct {
	DB     *db.Database
	Config *config.Config
}

func NewServer(database *db.Database, cfg *config.Config) *Server {
	return &Server{DB: database, Config: cfg}
}

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// 1. REST Core API
	mux.HandleFunc("/api/artists", s.handleGetArtists)
	mux.HandleFunc("/api/artists/", s.handleArtistActions)
	mux.HandleFunc("/api/albums", s.handleGetAlbums)
	mux.HandleFunc("/api/albums/", s.handleAlbumActions)
	mux.HandleFunc("/api/stream/", s.handleStream) // /api/stream/{id}
	mux.HandleFunc("/api/stream", s.handleStream)
	mux.HandleFunc("/api/cover/", s.handleGetCover)
	mux.HandleFunc("/api/cover", s.handleGetCover)
	mux.HandleFunc("/api/tracks", s.handleGetTracks)
	mux.HandleFunc("/api/tracks/", s.handleTrackActions)
	mux.HandleFunc("/api/scan", s.handleScan)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/library/health", s.handleLibraryHealth)
	mux.HandleFunc("/api/lyrics/", s.handleLyrics) // /api/lyrics/{id} or /api/lyrics/search
	mux.HandleFunc("/api/lyrics", s.handleLyrics)
	mux.HandleFunc("/api/lyrics/search", s.handleLyricsSearch)
	mux.HandleFunc("/api/metadata/search", s.handleMetadataSearch)
	mux.HandleFunc("/api/autocomplete", s.handleAutocomplete)
	mux.HandleFunc("/api/playlists", s.handlePlaylists)
	mux.HandleFunc("/api/playlists/", s.handlePlaylistActions)



	// 2. Subsonic Adapter Layer (Retrocompatibilidad OpenSubsonic)
	gw := gateway.NewSubsonicGateway(s.DB)
	gw.RegisterRoutes(mux)

	// 3. Frontend SPA handler Embedded
	if frontendFS, err := web.GetFrontendAssets(); err == nil {
		fileServer := http.FileServer(http.FS(frontendFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Evitar que colisione con el api /api/ -> 404 estricto
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "."
			}
			if _, statErr := fs.Stat(frontendFS, path); statErr != nil {
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return mux
}

func (s *Server) extractID(r *http.Request, prefix string) string {
	// 1. Intentar sacar de la ruta (después del prefijo)
	id := r.URL.Path[len(prefix):]
	
	// 2. Si está vacío en la ruta, intentar sacar del parámetro ?id=
	if id == "" || id == "/" {
		id = r.URL.Query().Get("id")
	}

	// Limpiar posibles barras sobrantes y sub-rutas
	id = strings.Trim(id, "/")
	id = strings.TrimSuffix(id, "/tags")
	id = strings.TrimSuffix(id, "/organize")
	id = strings.TrimSuffix(id, "/albums")
	id = strings.TrimSuffix(id, "/tracks")

	if decoded, err := url.PathUnescape(id); err == nil {
		id = decoded
	}
	
	return id
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	sc := scanner.NewScanner(s.DB)
	go func() {
		if err := sc.ScanDirectory(s.Config.MusicPath); err != nil {
			slog.Error("Manual scan failed", "error", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"scan started"}`))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.Config)
		return
	}

	if r.Method == http.MethodPut {
		var newCfg struct {
			MusicPath     string `json:"musicPath"`
			ThemeColor    string `json:"themeColor"`
			AutoRename    bool   `json:"autoRename"`
			RenamePattern string `json:"renamePattern"`
		}
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		s.Config.MusicPath = newCfg.MusicPath
		s.Config.ThemeColor = newCfg.ThemeColor
		s.Config.AutoRename = newCfg.AutoRename
		s.Config.RenamePattern = newCfg.RenamePattern

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"updated"}`))
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleGetArtists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := s.DB.Conn.Query("SELECT id, name FROM artists ORDER BY name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var artists []map[string]string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		artists = append(artists, map[string]string{"id": id, "name": name})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(artists)
}

func (s *Server) handleArtistActions(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/albums") {
		artistID := s.extractID(r, "/api/artists/")
		s.handleGetArtistAlbums(artistID, w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetArtistAlbums(artistID string, w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Conn.Query("SELECT id, title, year FROM albums WHERE artist_id = ? ORDER BY year DESC", artistID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var albums []map[string]interface{}
	for rows.Next() {
		var id, title string
		var year int
		if err := rows.Scan(&id, &title, &year); err != nil {
			continue
		}
		albums = append(albums, map[string]interface{}{"id": id, "title": title, "year": year})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleGetAlbums(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Conn.Query(`
		SELECT al.id, al.title, al.year, ar.name as artist, 
		       (SELECT t.id FROM tracks t WHERE t.album_id = al.id LIMIT 1) as sample_track_id
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		ORDER BY al.title
	`)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleAlbumActions(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/tracks") {
		albumID := s.extractID(r, "/api/albums/")
		s.handleGetAlbumTracks(albumID, w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetAlbumTracks(albumID string, w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Conn.Query(`
		SELECT id, title, track_num, duration, genre 
		FROM tracks 
		WHERE album_id = ? 
		ORDER BY track_num
	`, albumID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tracks []map[string]interface{}
	for rows.Next() {
		var id, title, genre string
		var trackNum, duration int
		if err := rows.Scan(&id, &title, &trackNum, &duration, &genre); err != nil {
			continue
		}
		tracks = append(tracks, map[string]interface{}{
			"id": id, "title": title, "trackNum": trackNum, "duration": duration, "genre": genre,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleTrackActions(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/tracks/")

	// PUT /api/tracks/{id}/tags
	if strings.HasSuffix(r.URL.Path, "/tags") && r.Method == http.MethodPut {
		s.handleEditTags(trackID, w, r)
		return
	}

	// POST /api/tracks/{id}/organize
	if strings.HasSuffix(r.URL.Path, "/organize") && r.Method == http.MethodPost {
		s.handleManualOrganize(trackID, w, r)
		return
	}
	// DELETE /api/tracks/{id}
	if r.Method == http.MethodDelete {
		s.handleDeleteTrack(trackID, w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/stream/")
	var fpath string
	err := s.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&fpath)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	bitrate := r.URL.Query().Get("bitrate")
	if bitrate != "" && bitrate != "original" {
		// Basic validation: must be something like "128k" or "192k"
		// Only allow alphanumeric characters and 'k'
		for _, char := range bitrate {
			if !((char >= '0' && char <= '9') || char == 'k' || char == 'm') {
				http.Error(w, "Invalid bitrate format", http.StatusBadRequest)
				return
			}
		}

		// Transcoding requested
		w.Header().Set("Content-Type", "audio/mpeg")
		// Use ffmpeg to transcode on the fly
		// We use mp3 as a safe universal format for now
		cmd := exec.Command("ffmpeg", "-i", fpath, "-f", "mp3", "-ab", bitrate, "-map_metadata", "-1", "pipe:1")
		cmd.Stdout = w
		if err := cmd.Start(); err != nil {
			http.Error(w, "Transcoding failed", http.StatusInternalServerError)
			return
		}
		cmd.Wait()
		return
	}

	http.ServeFile(w, r, fpath)
}
func (s *Server) handleEditTags(trackID string, w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	editor := tags.NewEditor(s.DB)
	if err := editor.UpdateTags(trackID, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Renombrado automático si está activo en Settings
	if s.Config.AutoRename {
		editor.OrganizeFile(trackID, s.Config.MusicPath, s.Config.RenamePattern)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleManualOrganize(trackID string, w http.ResponseWriter, r *http.Request) {
	editor := tags.NewEditor(s.DB)
	newPath, err := editor.OrganizeFile(trackID, s.Config.MusicPath, s.Config.RenamePattern)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"newPath": newPath})
}

func (s *Server) handleGetTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := s.DB.Conn.Query(`
		SELECT t.id, t.title, t.track_num, t.disc_num, t.duration, COALESCE(t.genre, ''), 
		       a.name as artist, al.title as album, al.year as year,
		       t.size, t.format, t.bit_rate, t.sample_rate,
		       t.composer, t.comment
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		ORDER BY a.name, al.year, t.track_num
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tracks []map[string]interface{}
	for rows.Next() {
		var id, title, genre, artist, album, format, composer, comment string
		var trackNum, discNum, duration, year, bitRate, sampleRate int
		var size int64
		if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment); err != nil {
			continue
		}
		tracks = append(tracks, map[string]interface{}{
			"id": id, "title": title, "trackNum": trackNum, "discNum": discNum, 
			"duration": duration, "genre": genre, "artist": artist, "album": album, "year": year,
			"size": size, "format": format, "bitrate": bitRate, "sampleRate": sampleRate,
			"composer": composer, "comment": comment,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleDeleteTrack(trackID string, w http.ResponseWriter, r *http.Request) {
	_, err := s.DB.Conn.Exec("DELETE FROM tracks WHERE id = ?", trackID)
	if err != nil {
		http.Error(w, "Failed to delete track", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing search query", http.StatusBadRequest)
		return
	}

	searchTerm := "%" + query + "%"

	results := make(map[string]interface{})

	// 1. Search Tracks (por título o género)
	tracksRows, err := s.DB.Conn.Query(`
		SELECT t.id, t.title, a.name as artist 
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		WHERE t.title LIKE ? OR t.genre LIKE ?
		LIMIT 20
	`, searchTerm, searchTerm)
	
	var tracks []map[string]interface{}
	if err == nil {
		defer tracksRows.Close()
		for tracksRows.Next() {
			var id, title, artist string
			tracksRows.Scan(&id, &title, &artist)
			tracks = append(tracks, map[string]interface{}{"id": id, "title": title, "artist": artist})
		}
	}
	results["tracks"] = tracks

	// 2. Search Artists
	artistRows, err := s.DB.Conn.Query("SELECT id, name FROM artists WHERE name LIKE ?", searchTerm)
	
	var artists []map[string]interface{}
	if err == nil {
		defer artistRows.Close()
		for artistRows.Next() {
			var id, name string
			artistRows.Scan(&id, &name)
			artists = append(artists, map[string]interface{}{"id": id, "name": name})
		}
	}
	results["artists"] = artists

	// 3. Search Albums
	albumRows, err := s.DB.Conn.Query(`
		SELECT al.id, al.title, ar.name as artist 
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		WHERE al.title LIKE ?
		LIMIT 20
	`, searchTerm)
	
	var albums []map[string]interface{}
	if err == nil {
		defer albumRows.Close()
		for albumRows.Next() {
			var id, title, artist string
			albumRows.Scan(&id, &title, &artist)
			albums = append(albums, map[string]interface{}{"id": id, "title": title, "artist": artist})
		}
	}
	results["albums"] = albums

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
		query = "SELECT DISTINCT name FROM artists WHERE name LIKE ? ORDER BY name LIMIT 10"
	case "genre":
		query = "SELECT DISTINCT genre FROM tracks WHERE genre LIKE ? ORDER BY genre LIMIT 10"
	case "album":
		query = "SELECT DISTINCT title FROM albums WHERE title LIKE ? ORDER BY title LIMIT 10"
	case "composer":
		query = "SELECT DISTINCT composer FROM tracks WHERE composer LIKE ? ORDER BY composer LIMIT 10"
	default:
		json.NewEncoder(w).Encode([]string{})
		return
	}

	rows, err := s.DB.Conn.Query(query, q+"%")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var val string
		rows.Scan(&val)
		results = append(results, val)
	}
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})
	
	// Contadores básicos
	var tTracks, tAlbums, tArtists, tPlaylists int
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&tTracks)
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM albums").Scan(&tAlbums)
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM artists").Scan(&tArtists)
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM playlists").Scan(&tPlaylists)
	stats["totalTracks"] = tTracks
	stats["totalAlbums"] = tAlbums
	stats["totalArtists"] = tArtists
	stats["totalPlaylists"] = tPlaylists

	// Joyas olvidadas (10 canciones aleatorias para redescubrir)
	rows, err := s.DB.Conn.Query("SELECT id, title, (SELECT name FROM artists WHERE id = (SELECT artist_id FROM albums WHERE id = tracks.album_id)) as artist FROM tracks ORDER BY RANDOM() LIMIT 10")
	var gems []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, artist string
			rows.Scan(&id, &title, &artist)
			gems = append(gems, map[string]interface{}{"id": id, "title": title, "artist": artist})
		}
	}
	stats["forgottenGems"] = gems

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleLibraryHealth(w http.ResponseWriter, r *http.Request) {
	health := make(map[string]interface{})

	// Pistas sin género o año
	var incomplete int
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE genre IS NULL OR genre = ''").Scan(&incomplete)
	health["missingTags"] = incomplete

	// Pistas sin letras (campo comment o similar por ahora hasta que tengamos campo lyrics real en DB)
	// Como no hay campo lyrics en DB todavía, buscamos en base a una heurística
	health["missingLyrics"] = 0 

	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleMetadataSearch(w http.ResponseWriter, r *http.Request) {
	artist := r.URL.Query().Get("artist")
	title := r.URL.Query().Get("title")
	
	if artist == "" || title == "" {
		http.Error(w, "Artist and title are required", http.StatusBadRequest)
		return
	}

	searchQuery := fmt.Sprintf("%s %s", artist, title)
	url := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=music&limit=5", strings.ReplaceAll(searchQuery, " ", "+"))
	
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "External search failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var itunesResult struct {
		Results []struct {
			ArtistName     string `json:"artistName"`
			TrackName      string `json:"trackName"`
			CollectionName string `json:"collectionName"`
			ReleaseDate    string `json:"releaseDate"`
			PrimaryGenre   string `json:"primaryGenreName"`
			TrackNumber    int    `json:"trackNumber"`
			DiscNumber     int    `json:"discNumber"`
			ArtworkUrl100  string `json:"artworkUrl100"`
		} `json:"results"`
	}

	json.NewDecoder(resp.Body).Decode(&itunesResult)

	list := make([]map[string]interface{}, 0)
	for _, item := range itunesResult.Results {
		year := 0
		if len(item.ReleaseDate) >= 4 { fmt.Sscanf(item.ReleaseDate[:4], "%d", &year) }
		
		list = append(list, map[string]interface{}{
			"artist":   item.ArtistName,
			"title":    item.TrackName,
			"album":    item.CollectionName,
			"year":     year,
			"genre":    item.PrimaryGenre,
			"trackNum": item.TrackNumber,
			"discNum":  item.DiscNumber,
			"artwork":  item.ArtworkUrl100,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleLyrics(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/lyrics/")
	
	if r.Method == http.MethodGet {
		s.handleGetLyrics(trackID, w, r)
		return
	}

	if r.Method == http.MethodPut {
		var body struct {
			Lyrics string `json:"lyrics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		editor := tags.NewEditor(s.DB)
		if err := editor.UpdateLyrics(trackID, body.Lyrics); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"lyrics saved to file"}`))
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleGetLyrics(trackID string, w http.ResponseWriter, r *http.Request) {
	var title, artist string
	err := s.DB.Conn.QueryRow(`
		SELECT t.title, a.name 
		FROM tracks t 
		JOIN albums al ON t.album_id = al.id 
		JOIN artists a ON al.artist_id = a.id 
		WHERE t.id = ?`, trackID).Scan(&title, &artist)
	
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}
	
	editor := tags.NewEditor(s.DB)

	// 1. Intentar leer de archivo local primero (verdad absoluta del usuario)
	localLyrics, _ := editor.ReadLyrics(trackID)
	if localLyrics != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"lyrics": localLyrics})
		return
	}

	// 2. Intentar obtener de LRCLib (mejor calidad, letras sincronizadas)
	resp, err := http.Get(fmt.Sprintf("https://lrclib.net/api/get?artist_name=%s&track_name=%s", url.QueryEscape(artist), url.QueryEscape(title)))
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
		return
	}

	// Fallback: lyrics.ovh
	resp, err = http.Get(fmt.Sprintf("https://api.lyrics.ovh/v1/%s/%s", url.QueryEscape(artist), url.QueryEscape(title)))
	if err != nil {
		http.Error(w, "Failed to fetch lyrics", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func (s *Server) handleLyricsSearch(w http.ResponseWriter, r *http.Request) {
	artist := r.URL.Query().Get("artist")
	title := r.URL.Query().Get("title")
	
	if artist == "" || title == "" {
		http.Error(w, "Artist and title are required", http.StatusBadRequest)
		return
	}

	resp, err := http.Get(fmt.Sprintf("https://lrclib.net/api/search?artist_name=%s&track_name=%s", url.QueryEscape(artist), url.QueryEscape(title)))
	if err != nil {
		http.Error(w, "Failed to search lyrics", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		id := strings.ToLower(strings.ReplaceAll(body.Name, " ", "-")) + "-" + fmt.Sprintf("%d", os.Getpid())
		_, err := s.DB.Conn.Exec("INSERT INTO playlists (id, name, is_smart) VALUES (?, ?, ?)", id, body.Name, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	// Simple list for now
	rows, err := s.DB.Conn.Query("SELECT id, name, is_smart FROM playlists")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, name string
		var isSmart int
		rows.Scan(&id, &name, &isSmart)
		list = append(list, map[string]interface{}{"id": id, "name": name, "isSmart": isSmart == 1})
	}
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handlePlaylistActions(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/tracks") {
		s.handleGetPlaylistTracks(w, r)
		return
	}
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func (s *Server) handleGetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/playlists/")
	id = strings.TrimSuffix(id, "/tracks")

	rows, err := s.DB.Conn.Query(`
		SELECT t.id, t.title, a.name, al.title, t.duration
		FROM tracks t
		JOIN playlist_tracks pt ON t.id = pt.track_id
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		WHERE pt.playlist_id = ?
		ORDER BY pt.position ASC
	`, id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var tid, title, artist, album string
		var duration int
		rows.Scan(&tid, &title, &artist, &album, &duration)
		list = append(list, map[string]interface{}{
			"id": tid, "title": title, "artist": artist, 
			"album": album, "duration": duration,
		})
	}
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleGetCover(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/cover/")
	if trackID == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var fpath string
	if err := s.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&fpath); err != nil {
		slog.Error("Cover lookup: failed to find track path in DB", "trackID", trackID, "error", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// 1. Intentar extraer portada embebida
	f, err := os.Open(fpath)
	if err == nil {
		defer f.Close()
		metadata, err := tag.ReadFrom(f)
		if err == nil {
			if pic := metadata.Picture(); pic != nil {
				w.Header().Set("Cache-Control", "public, max-age=86400")
				w.Header().Set("Content-Type", pic.MIMEType)
				w.Write(pic.Data)
				return
			}
		} else {
			slog.Warn("Cover lookup: failed to read tags from file", "path", fpath, "error", err)
		}
	} else {
		slog.Error("Cover lookup: failed to open file", "path", fpath, "error", err)
	}

	// 2. Fallback: Buscar archivos de imagen en la misma carpeta (Case-insensitive)
	dir := filepath.Dir(fpath)
	entries, err := os.ReadDir(dir)
	if err == nil {
		possibleNames := map[string]bool{
			"cover": true, "folder": true, "album": true, "front": true, "artwork": true,
		}
		for _, entry := range entries {
			if entry.IsDir() { continue }
			name := strings.ToLower(entry.Name())
			ext := filepath.Ext(name)
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" { continue }
			
			base := strings.TrimSuffix(name, ext)
			if possibleNames[base] {
				fullPath := filepath.Join(dir, entry.Name())
				if imgData, err := os.ReadFile(fullPath); err == nil {
					mime := "image/jpeg"
					if ext == ".png" { mime = "image/png" }
					w.Header().Set("Cache-Control", "public, max-age=86400")
					w.Header().Set("Content-Type", mime)
					w.Write(imgData)
					return
				}
			}
		}
	}

	// 3. Last fallback: try to find ANY image in the folder if it's the only one
	if err == nil && len(entries) > 0 {
		for _, entry := range entries {
			if entry.IsDir() { continue }
			name := strings.ToLower(entry.Name())
			ext := filepath.Ext(name)
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
				fullPath := filepath.Join(dir, entry.Name())
				if imgData, err := os.ReadFile(fullPath); err == nil {
					mime := "image/jpeg"
					if ext == ".png" { mime = "image/png" }
					w.Header().Set("Content-Type", mime)
					w.Write(imgData)
					return
				}
			}
		}
	}

	slog.Warn("Cover lookup: no cover found", "path", fpath)
	http.Error(w, "No cover found", http.StatusNotFound)
}
