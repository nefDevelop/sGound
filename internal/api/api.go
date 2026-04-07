package api

import (
	"encoding/json"
	"net/http"
	"os"
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
	mux.HandleFunc("/api/cover/", s.handleGetCover)
	mux.HandleFunc("/api/tracks", s.handleGetTracks)
	mux.HandleFunc("/api/tracks/", s.handleTrackActions)
	mux.HandleFunc("/api/scan", s.handleScan)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/library/health", s.handleLibraryHealth)
	mux.HandleFunc("/api/artists/", s.handleArtistActions)
	mux.HandleFunc("/api/lyrics/", s.handleLyrics)
	mux.HandleFunc("/api/metadata/search", s.handleMetadataSearch)
	mux.HandleFunc("/api/autocomplete", s.handleAutocomplete)
	mux.HandleFunc("/api/playlists", s.handlePlaylists)

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
			MusicPath string `json:"musicPath"`
		}
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		s.Config.MusicPath = newCfg.MusicPath
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
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) == 4 && pathParts[3] == "albums" {
		s.handleGetArtistAlbums(pathParts[2], w, r)
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
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) == 4 && pathParts[3] == "tracks" {
		s.handleGetAlbumTracks(pathParts[2], w, r)
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
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	
	// PUT /api/tracks/{id}/tags
	if len(pathParts) == 4 && pathParts[3] == "tags" && r.Method == http.MethodPut {
		s.handleEditTags(pathParts[2], w, r)
		return
	}

	// POST /api/tracks/{id}/organize
	if len(pathParts) == 4 && pathParts[3] == "organize" && r.Method == http.MethodPost {
		s.handleManualOrganize(pathParts[2], w, r)
		return
	}
...
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
	tracksRows, _ := s.DB.Conn.Query(`
		SELECT t.id, t.title, a.name as artist, al.title as album, t.genre
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		WHERE t.title LIKE ? OR t.genre LIKE ?
		LIMIT 20`, searchTerm, searchTerm)
	defer tracksRows.Close()
	var tracks []map[string]string
	for tracksRows.Next() {
		var id, title, artist, album, genre string
		tracksRows.Scan(&id, &title, &artist, &album, &genre)
		tracks = append(tracks, map[string]string{"id": id, "title": title, "artist": artist, "album": album, "genre": genre})
	}
	results["tracks"] = tracks

	// 2. Search Artists (por nombre)
	artistsRows, _ := s.DB.Conn.Query("SELECT id, name FROM artists WHERE name LIKE ? LIMIT 10", searchTerm)
	defer artistsRows.Close()
	var artists []map[string]string
	for artistsRows.Next() {
		var id, name string
		artistsRows.Scan(&id, &name)
		artists = append(artists, map[string]string{"id": id, "name": name})
	}
	results["artists"] = artists

	// 3. Search Albums (por título)
	albumsRows, _ := s.DB.Conn.Query(`
		SELECT al.id, al.title, ar.name as artist 
		FROM albums al 
		JOIN artists ar ON al.artist_id = ar.id 
		WHERE al.title LIKE ? 
		LIMIT 10`, searchTerm)
	defer albumsRows.Close()
	var albums []map[string]string
	for albumsRows.Next() {
		var id, title, artist string
		albumsRows.Scan(&id, &title, &artist)
		albums = append(albums, map[string]string{"id": id, "title": title, "artist": artist})
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
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&stats["totalTracks"])
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM albums").Scan(&stats["totalAlbums"])
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM artists").Scan(&stats["totalArtists"])
	s.DB.Conn.QueryRow("SELECT COUNT(*) FROM playlists").Scan(&stats["totalPlaylists"])

	// Joyas olvidadas (10 canciones aleatorias para redescubrir)
	rows, _ := s.DB.Conn.Query(`
		SELECT t.id, t.title, a.name as artist 
		FROM tracks t 
		JOIN albums al ON t.album_id = al.id 
		JOIN artists a ON al.artist_id = a.id 
		ORDER BY RANDOM() LIMIT 10`)
	defer rows.Close()
	var forgotten []map[string]string
	for rows.Next() {
		var id, title, artist string
		rows.Scan(&id, &title, &artist)
		forgotten = append(forgotten, map[string]string{"id": id, "title": title, "artist": artist})
	}
	stats["forgottenGems"] = forgotten

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

	var list []map[string]interface{}
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
	trackID := r.URL.Path[len("/api/lyrics/"):]
	
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

	// 1. Intentar descargar letras de API externa
	resp, err := http.Get(fmt.Sprintf("https://api.lyrics.ovh/v1/%s/%s", artist, title))
	if err != nil {
		http.Error(w, "Failed to fetch lyrics", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	// Simple list for now
	rows, _ := s.DB.Conn.Query("SELECT id, name, is_smart FROM playlists")
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

func (s *Server) handleGetCover(w http.ResponseWriter, r *http.Request) {
	trackID := r.URL.Path[len("/api/cover/"):]
	if trackID == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var fpath string
	if err := s.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&fpath); err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// 1. Intentar extraer portada embebida
	f, err := os.Open(fpath)
	if err == nil {
		defer f.Close()
		if metadata, err := tag.ReadFrom(f); err == nil && metadata.Picture() != nil {
			pic := metadata.Picture()
			w.Header().Set("Content-Type", pic.MIMEType)
			w.Write(pic.Data)
			return
		}
	}

	// 2. Fallback: Buscar archivos de imagen en la misma carpeta
	dir := filepath.Dir(fpath)
	coverFiles := []string{"cover.jpg", "cover.png", "folder.jpg", "folder.png", "album.jpg", "front.jpg"}
	
	for _, cf := range coverFiles {
		fullPath := filepath.Join(dir, cf)
		if imgData, err := os.ReadFile(fullPath); err == nil {
			// Detectar el tipo de contenido por la extensión
			ext := strings.ToLower(filepath.Ext(cf))
			mime := "image/jpeg"
			if ext == ".png" {
				mime = "image/png"
			}
			w.Header().Set("Content-Type", mime)
			w.Write(imgData)
			return
		}
	}

	http.Error(w, "No cover found", http.StatusNotFound)
}
