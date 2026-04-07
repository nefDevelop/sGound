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
	"strconv"
	"strings"
	"io/fs"
	"log/slog"
	"context"

	"github.com/dhowden/tag"
	
	"sgound/internal/db"
	"sgound/internal/tags"
	"sgound/internal/config"
	"sgound/internal/scanner"
	"sgound/internal/metadata"
	"sgound/internal/auth"
	"sgound/web"
	"sync"
)

type ScanStatus struct {
	Scanning  bool `json:"scanning"`
	Processed int  `json:"processed"`
}

type Server struct {
	MasterDB *db.MasterDatabase
	Config   *config.Config

	activeDBs    map[string]*db.Database
	dbMutex      sync.RWMutex
	jwtSecret    []byte
	scanStatuses sync.Map // [string]*ScanStatus
}

func NewServer(masterDB *db.MasterDatabase, cfg *config.Config) *Server {
	secret, err := masterDB.GetJWTSecret()
	if err != nil {
		// Generate random secret on first run
		secret = auth.GenerateAPIKey() // Use same generator for simplicity
		masterDB.SetJWTSecret(secret)
	}

	return &Server{
		MasterDB:     masterDB,
		Config:       cfg,
		activeDBs:    make(map[string]*db.Database),
		jwtSecret:    []byte(secret),
		scanStatuses: sync.Map{},
	}
}

type contextKey string
const userKey contextKey = "user"

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Helper to wrap with auth
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return s.authMiddleware(h)
	}

	// 1. REST Core API
	mux.HandleFunc("/api/artists", auth(s.handleGetArtists))
	mux.HandleFunc("/api/artists/", auth(s.handleArtistActions))
	mux.HandleFunc("/api/albums", auth(s.handleGetAlbums))
	mux.HandleFunc("/api/albums/", auth(s.handleAlbumActions))
	mux.HandleFunc("/api/stream", auth(s.handleStream))
	mux.HandleFunc("/api/stream/", auth(s.handleStream))
	mux.HandleFunc("/api/cover", auth(s.handleGetCover))
	mux.HandleFunc("/api/cover/", auth(s.handleGetCover))
	mux.HandleFunc("/api/tracks", auth(s.handleGetTracks))
	mux.HandleFunc("/api/tracks/", auth(s.handleTrackActions))
	mux.HandleFunc("/api/scan", auth(s.handleScan))
	mux.HandleFunc("/api/config", auth(s.handleConfig))
	mux.HandleFunc("/api/search", auth(s.handleSearch))
	mux.HandleFunc("/api/dashboard", auth(s.handleDashboard))
	mux.HandleFunc("/api/library/health", auth(s.handleLibraryHealth))
	mux.HandleFunc("/api/library/status", auth(s.handleScanStatus))
	mux.HandleFunc("/api/library/inbox", auth(s.handleGetInbox))
	mux.HandleFunc("/api/tracks/approve", auth(s.handleApproveTracks))
	mux.HandleFunc("/api/tracks/rate", auth(s.handleRateTrack))
	mux.HandleFunc("/api/lyrics", auth(s.handleLyrics))
	mux.HandleFunc("/api/lyrics/", auth(s.handleLyrics))
	mux.HandleFunc("/api/playlists", auth(s.handlePlaylists))
	mux.HandleFunc("/api/playlists/", auth(s.handlePlaylistActions))

	// 2. Auth Actions
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/auth/setup", s.handleSetup)

	// 2.5 API Key Management (Self-service)
	mux.HandleFunc("/api/auth/apikeys", s.authMiddleware(s.handleAPIKeys))

	// 3. Subsonic Adapter Layer (Needs refactoring for multi-user)
	// gw := gateway.NewSubsonicGateway(...)
	// gw.RegisterRoutes(mux)

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

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	user, err := s.MasterDB.GetUserByUsername(creds.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	match, err := auth.VerifyPassword(creds.Password, user.PasswordHash)
	if err != nil || !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(user.Username, s.jwtSecret)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	// Set as cookie for web sessions
	http.SetCookie(w, &http.Cookie{
		Name:     "sgound_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":       token,
		"username":    user.Username,
		"permissions": user.Permissions,
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	// Check if user is in AllowedUsers in config
	perms, allowed := s.Config.AllowedUsers[username]
	if !allowed {
		http.Error(w, "User not authorized in server config", http.StatusForbidden)
		return
	}

	user, _ := s.MasterDB.GetUserByUsername(username)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allowed":       true,
		"setupRequired": user == nil,
		"permissions":   perms,
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Check if allowed
	perms, allowed := s.Config.AllowedUsers[body.Username]
	if !allowed {
		http.Error(w, "User not authorized in server config", http.StatusForbidden)
		return
	}

	user, _ := s.MasterDB.GetUserByUsername(body.Username)
	if user != nil {
		http.Error(w, "User already exists", http.StatusForbidden)
		return
	}

	hash, _ := auth.HashPassword(body.Password)
	// Create user with perms from config
	err := s.MasterDB.CreateUser(body.Username, hash, perms, s.Config.DBPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Trigger initial scan in background for the new user
	userDBPath := filepath.Join(filepath.Dir(s.Config.DBPath), "library_"+body.Username+".db")
	go func() {
		slog.Info("Starting initial background scan for auto-configuration", "user", body.Username, "path", s.Config.MusicPath)
		uDB, err := db.Connect(userDBPath)
		if err == nil {
			status := &ScanStatus{Scanning: true, Processed: 0}
			s.scanStatuses.Store(body.Username, status)

			sc := scanner.NewScanner(uDB)
			sc.OnProgress = func(path string, count int) {
				status.Processed = count
				s.scanStatuses.Store(body.Username, status)
			}
			
			defer func() {
				status.Scanning = false
				s.scanStatuses.Store(body.Username, status)
			}()

			sc.ScanDirectory(s.Config.MusicPath)
		} else {
			slog.Error("Auto-scan failed: could not connect to user DB", "user", body.Username, "error", err)
		}
	}()

	// Automatic Session: Set JWT cookie so user lands directly in dashboard
	token, err := auth.GenerateJWT(body.Username, s.jwtSecret)
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "sgound_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400 * 30,
		})
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"username":    body.Username,
		"permissions": perms,
	})
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	if user == nil { return }

	if r.Method == http.MethodGet {
		keys, _ := s.MasterDB.GetAPIKeys(user.Username)
		json.NewEncoder(w).Encode(keys)
		return
	}

	if r.Method == http.MethodPost {
		var body struct {
			Label       string `json:"label"`
			Permissions string `json:"permissions"` // "r" or "rw"
		}
		json.NewDecoder(r.Body).Decode(&body)

		// Guard: Cannot grant more than you have
		if user.Permissions == "r" && body.Permissions == "rw" {
			http.Error(w, "Cannot create RW key with R user account", http.StatusForbidden)
			return
		}

		key := auth.GenerateAPIKey()
		err := s.MasterDB.CreateAPIKey(key, user.Username, body.Permissions, body.Label)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"key": key, "label": body.Label, "permissions": body.Permissions})
		return
	}

	if r.Method == http.MethodDelete {
		key := r.URL.Query().Get("key")
		s.MasterDB.DeleteAPIKey(key, user.Username)
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user *db.User
		var effectivePerms string

		// 1. Check JWT Cookie (Web Session)
		if cookie, err := r.Cookie("sgound_token"); err == nil {
			claims, err := auth.VerifyJWT(cookie.Value, s.jwtSecret)
			if err == nil {
				user, _ = s.MasterDB.GetUserByUsername(claims.Username)
				if user != nil {
					effectivePerms = user.Permissions
				}
			}
		}

		// 2. Check API Key (External clients)
		if user == nil {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("apiKey")
			}
			if apiKey != "" {
				keyInfo, err := s.MasterDB.GetAPIKeyInfo(apiKey)
				if err == nil {
					user, _ = s.MasterDB.GetUserByUsername(keyInfo.Username)
					if user != nil {
						effectivePerms = keyInfo.Permissions
					}
				}
			}
		}

		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 3. Permission Guard for Mutation Methods
		if r.Method != http.MethodGet && effectivePerms == "r" {
			// Special case: we allow OPTIONS for CORS
			if r.Method != http.MethodOptions {
				http.Error(w, "Forbidden: Read-only access", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (s *Server) getDB(r *http.Request) *db.Database {
	user, ok := r.Context().Value(userKey).(*db.User)
	if !ok {
		return nil
	}
	s.dbMutex.RLock()
	d, ok := s.activeDBs[user.DBPath]
	s.dbMutex.RUnlock()
	if ok {
		return d
	}
	s.dbMutex.Lock()
	defer s.dbMutex.Unlock()
	if d, ok := s.activeDBs[user.DBPath]; ok {
		return d
	}
	newDB, err := db.Connect(user.DBPath)
	if err != nil {
		slog.Error("Failed to connect user DB", "path", user.DBPath, "err", err)
		return nil
	}
	s.activeDBs[user.DBPath] = newDB
	return newDB
}

func (s *Server) extractID(r *http.Request, prefix string) string {
	// 1. Intentar sacar del query param ?id= (prioridad)
	if qID := r.URL.Query().Get("id"); qID != "" {
		return strings.Trim(qID, "/")
	}

	// 2. Intentar sacar de la ruta
	path := r.URL.Path
	// Normalizar prefijo para la comparación
	cleanPrefix := strings.TrimSuffix(prefix, "/")
	
	var id string
	if strings.HasPrefix(path, cleanPrefix) {
		id = path[len(cleanPrefix):]
	}

	// Limpiar barras y sub-rutas
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

	user, _ := r.Context().Value(userKey).(*db.User)
	
	// Reset status
	status := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store(user.Username, status)

	sc := scanner.NewScanner(s.getDB(r))
	sc.OnProgress = func(path string, count int) {
		status.Processed = count
		s.scanStatuses.Store(user.Username, status)
	}

	go func() {
		defer func() {
			status.Scanning = false
			s.scanStatuses.Store(user.Username, status)
		}()
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
			MusicPath         string `json:"musicPath"`
			ThemeColor        string `json:"themeColor"`
			AutoRename        bool   `json:"autoRename"`
			RenamePattern     string `json:"renamePattern"`
			ReplayGainEnabled bool   `json:"replayGainEnabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		s.Config.MusicPath = newCfg.MusicPath
		s.Config.ThemeColor = newCfg.ThemeColor
		s.Config.AutoRename = newCfg.AutoRename
		s.Config.RenamePattern = newCfg.RenamePattern
		s.Config.ReplayGainEnabled = newCfg.ReplayGainEnabled

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

	rows, err := s.getDB(r).Conn.Query("SELECT id, name FROM artists ORDER BY name")
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
	rows, err := s.getDB(r).Conn.Query("SELECT id, title, year FROM albums WHERE artist_id = ? ORDER BY year DESC", artistID)
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
	rows, err := s.getDB(r).Conn.Query(`
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
	rows, err := s.getDB(r).Conn.Query(`
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
	err := s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&fpath)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// Registrar reproducción
	go func() {
		s.getDB(r).Conn.Exec("UPDATE tracks SET last_played = CURRENT_TIMESTAMP, play_count = play_count + 1 WHERE id = ?", trackID)
	}()

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

	editor := tags.NewEditor(s.getDB(r))
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
	editor := tags.NewEditor(s.getDB(r))
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

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}
	
	whereClause := "WHERE t.status = ?"
	args := []interface{}{status}
	if status == "all" {
		whereClause = "WHERE 1=1"
		args = []interface{}{}
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.track_num, t.disc_num, t.duration, COALESCE(t.genre, ''), 
		       a.name as artist, al.title as album, al.year as year,
		       t.size, t.format, t.bit_rate, t.sample_rate,
		       t.composer, t.comment, t.status, t.rating, t.instrumental
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		%s
		ORDER BY a.name, al.year, t.track_num`, whereClause)

	rows, err := s.getDB(r).Conn.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tracks := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, title, genre, artist, album, format, composer, comment, tStatus string
		var trackNum, discNum, duration, year, bitRate, sampleRate, rating, instrumental int
		var size int64
		if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment, &tStatus, &rating, &instrumental); err != nil {
			slog.Error("Scan error", "err", err)
			continue
		}
		tracks = append(tracks, map[string]interface{}{
			"id": id, "title": title, "trackNum": trackNum, "discNum": discNum, 
			"duration": duration, "genre": genre, "artist": artist, "album": album, "year": year,
			"size": size, "format": format, "bitrate": bitRate, "sampleRate": sampleRate,
			"composer": composer, "comment": comment, "status": tStatus, "rating": rating,
			"instrumental": instrumental == 1,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleGetInbox(w http.ResponseWriter, r *http.Request) {
	// Forzar filtro por status=pending
	q := r.URL.Query()
	q.Set("status", "pending")
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

	for _, id := range data.IDs {
		_, err := s.getDB(r).Conn.Exec("UPDATE tracks SET status = 'active' WHERE id = ?", id)
		if err != nil {
			slog.Error("Failed to approve track", "id", id, "error", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleRateTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		ID     string `json:"id"`
		Rating int    `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// 1. Actualizar DB
	_, err := s.getDB(r).Conn.Exec("UPDATE tracks SET rating = ? WHERE id = ?", data.Rating, data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Sincronizar con Tags del archivo
	editor := tags.NewEditor(s.getDB(r))
	// Usamos un mapa para UpdateTags que ya maneja la lógica de escritura física
	tagData := map[string]interface{}{
		"rating": float64(data.Rating), // El editor espera float64 de los JSON decoders genéricos aveces
	}
	
	if err := editor.UpdateTags(data.ID, tagData); err != nil {
		slog.Error("Failed to sync rating to tags", "id", data.ID, "error", err)
		// No fallamos la petición completa si solo falla el tag físico
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleDeleteTrack(trackID string, w http.ResponseWriter, r *http.Request) {
	_, err := s.getDB(r).Conn.Exec("DELETE FROM tracks WHERE id = ?", trackID)
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
	tracksRows, err := s.getDB(r).Conn.Query(`
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
	artistRows, err := s.getDB(r).Conn.Query("SELECT id, name FROM artists WHERE name LIKE ?", searchTerm)
	
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
	albumRows, err := s.getDB(r).Conn.Query(`
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

	rows, err := s.getDB(r).Conn.Query(query, q+"%")
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

func (s *Server) handleLibraryMetadata(w http.ResponseWriter, r *http.Request) {
	// Obtener décadas únicas
	decadeRows, err := s.getDB(r).Conn.Query("SELECT DISTINCT (year / 10) * 10 as decade FROM albums WHERE year > 0 ORDER BY decade DESC")
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

	// Obtener géneros únicos
	genreRows, err := s.getDB(r).Conn.Query("SELECT DISTINCT genre FROM tracks WHERE genre IS NOT NULL AND genre != '' ORDER BY genre ASC")
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

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})
	
	// Contadores básicos
	var tTracks, tAlbums, tArtists, tPlaylists int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&tTracks)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM albums").Scan(&tAlbums)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM artists").Scan(&tArtists)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM playlists").Scan(&tPlaylists)
	stats["totalTracks"] = tTracks
	stats["totalAlbums"] = tAlbums
	stats["totalArtists"] = tArtists
	stats["totalPlaylists"] = tPlaylists

	// Inbox count (pendientes de aprobar)
	var pendingCount int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'pending'").Scan(&pendingCount)
	stats["inboxCount"] = pendingCount

	// Joyas olvidadas (9 canciones aleatorias para redescubrir)
	gems := make([]map[string]interface{}, 0)
	rows, err := s.getDB(r).Conn.Query("SELECT id, title, (SELECT name FROM artists WHERE id = (SELECT artist_id FROM albums WHERE id = tracks.album_id)) as artist FROM tracks ORDER BY RANDOM() LIMIT 9")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, artist string
			rows.Scan(&id, &title, &artist)
			gems = append(gems, map[string]interface{}{"id": id, "title": title, "artist": artist})
		}
	}
	stats["forgottenGems"] = gems

	// Lo más escuchado (Top 10)
	topPlays := make([]map[string]interface{}, 0)
	rows2, err := s.getDB(r).Conn.Query("SELECT id, title, (SELECT name FROM artists WHERE id = (SELECT artist_id FROM albums WHERE id = tracks.album_id)) as artist, play_count FROM tracks WHERE play_count > 0 ORDER BY play_count DESC LIMIT 10")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleScanStatus(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Obtener estado del escaneo
	scanning := false
	processed := 0
	if val, ok := s.scanStatuses.Load(user.Username); ok {
		sStatus := val.(*ScanStatus)
		scanning = sStatus.Scanning
		processed = sStatus.Processed
	}

	// Obtener contador de inbox (pendientes)
	var inboxCount int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'pending'").Scan(&inboxCount)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"scanning":   scanning,
		"processed":  processed,
		"inboxCount": inboxCount,
	})
}

func (s *Server) handleLibraryHealth(w http.ResponseWriter, r *http.Request) {
	health := make(map[string]interface{})

	// Pistas sin género o año
	var incomplete int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE genre IS NULL OR genre = ''").Scan(&incomplete)
	health["missingTags"] = incomplete

	// Duplicados
	dups, _ := s.getDB(r).GetDuplicates()
	health["duplicateCount"] = len(dups)

	// Huérfanos
	orphans, _ := s.getDB(r).GetOrphanedTracks()
	health["orphanCount"] = len(orphans)

	// Baja calidad (< 128kbps)
	lowQual, _ := s.getDB(r).GetLowQualityTracks(128)
	health["lowQualityCount"] = len(lowQual)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleLibraryDuplicates(w http.ResponseWriter, r *http.Request) {
	dups, err := s.getDB(r).GetDuplicates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dups)
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
	for _, o := range orphans {
		ids = append(ids, o.ID)
	}

	if err := s.getDB(r).DeleteTracks(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"status":"success", "removed": %d}`, len(ids))))
}

func (s *Server) handleLibraryLowQuality(w http.ResponseWriter, r *http.Request) {
	thresholdStr := r.URL.Query().Get("bitrate")
	threshold := 128
	if thresholdStr != "" {
		if val, err := strconv.Atoi(thresholdStr); err == nil {
			threshold = val
		}
	}

	tracks, err := s.getDB(r).GetLowQualityTracks(threshold)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}


func (s *Server) handleMetadataSuggest(w http.ResponseWriter, r *http.Request) {
	trackID := r.URL.Query().Get("id")
	if trackID == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	var title, artist, album string
	err := s.getDB(r).Conn.QueryRow(`
		SELECT t.title, ar.name, al.title
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists ar ON al.artist_id = ar.id
		WHERE t.id = ?`, trackID).Scan(&title, &artist, &album)

	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	mb := metadata.NewMBClient()
	releases, err := mb.SearchRelease(artist, album)
	if err != nil || len(releases) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	// Cogemos la primera por simplicidad para auto-tag
	details, err := mb.GetReleaseDetails(releases[0].ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var suggestions []map[string]interface{}
	for _, medium := range details.Media {
		for _, track := range medium.Tracks {
			suggestions = append(suggestions, map[string]interface{}{
				"id":       track.ID,
				"title":    track.Title,
				"trackNum": track.Number,
				"artist":   artist,
				"album":    details.Title,
				"year":     details.Date,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
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

		editor := tags.NewEditor(s.getDB(r))
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
	err := s.getDB(r).Conn.QueryRow(`
		SELECT t.title, a.name 
		FROM tracks t 
		JOIN albums al ON t.album_id = al.id 
		JOIN artists a ON al.artist_id = a.id 
		WHERE t.id = ?`, trackID).Scan(&title, &artist)
	
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}
	
	editor := tags.NewEditor(s.getDB(r))

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
		_, err := s.getDB(r).Conn.Exec("INSERT INTO playlists (id, name, is_smart) VALUES (?, ?, ?)", id, body.Name, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	// Simple list for now
	rows, err := s.getDB(r).Conn.Query("SELECT id, name, is_smart FROM playlists")
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

func (s *Server) handleSmartPlaylists(w http.ResponseWriter, r *http.Request) {
	typeStr := s.extractID(r, "/api/smart-playlists/")
	
	baseQuery := `
		SELECT t.id, t.title, t.track_num, t.disc_num, t.duration, COALESCE(t.genre, ''),
		       a.name as artist, al.title as album, al.year as year,
		       t.size, t.format, t.bit_rate, t.sample_rate,
		       t.composer, t.comment, t.instrumental
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id`
	
	var whereClause, orderClause string
	var args []interface{}

	switch typeStr {
	case "recently-played":
		whereClause = "WHERE t.last_played IS NOT NULL"
		orderClause = "ORDER BY t.last_played DESC LIMIT 50"
	case "high-quality":
		whereClause = "WHERE t.bit_rate >= 320000 OR t.format = 'flac' OR t.format = 'wav'"
		orderClause = "ORDER BY t.bit_rate DESC"
	case "recently-added":
		orderClause = "ORDER BY t.created_at DESC LIMIT 50"
	case "decade":
		decade := r.URL.Query().Get("value")
		if decade != "" {
			dInt, _ := strconv.Atoi(decade)
			whereClause = "WHERE al.year >= ? AND al.year <= ?"
			args = append(args, dInt, dInt+9)
		}
		orderClause = "ORDER BY al.year DESC, al.title, t.track_num"
	case "genre":
		genre := r.URL.Query().Get("value")
		if genre != "" {
			whereClause = "WHERE t.genre = ?"
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tracks []map[string]interface{}
	for rows.Next() {
		var id, title, genre, artist, album, format, composer, comment string
		var trackNum, discNum, duration, year, bitRate, sampleRate, instrumental int
		var size int64
		if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment, &instrumental); err != nil {
			continue
		}
		tracks = append(tracks, map[string]interface{}{
			"id": id, "title": title, "trackNum": trackNum, "discNum": discNum, 
			"duration": duration, "genre": genre, "artist": artist, "album": album, "year": year,
			"size": size, "format": format, "bitrate": bitRate, "sampleRate": sampleRate,
			"composer": composer, "comment": comment, "instrumental": instrumental == 1,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handlePlaylistActions(w http.ResponseWriter, r *http.Request) {
	id := s.extractID(r, "/api/playlists/")

	if strings.HasSuffix(r.URL.Path, "/tracks") {
		if r.Method == http.MethodGet {
			s.handleGetPlaylistTracks(w, r)
		} else if r.Method == http.MethodPost {
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
		s.getDB(r).DeletePlaylist(id)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
		return
	}

	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func (s *Server) handleGetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/playlists/")
	id = strings.TrimSuffix(id, "/tracks")

	rows, err := s.getDB(r).Conn.Query(`
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
	// GIF transparente de 1x1 base
	transparentPixel := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
		0x00, 0x00, 0x00, 0x21, 0xF9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2C, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3B,
	}

	trackID := s.extractID(r, "/api/cover") // Usar prefijo limpio
	if trackID == "" {
		w.Header().Set("Content-Type", "image/gif")
		w.WriteHeader(http.StatusOK)
		w.Write(transparentPixel)
		return
	}

	var fpath string
	// Intentamos buscar por track ID primero
	err := s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&fpath)
	if err != nil {
		slog.Debug("Cover lookup: ID not found in tracks, trying albums", "id", trackID)
		// Si falla, quizás es un ID de álbum? Intentamos buscar el primer track del álbum
		err = s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE album_id = ? LIMIT 1", trackID).Scan(&fpath)
		if err != nil {
			slog.Warn("Cover lookup: ID not found in tracks or albums", "id", trackID)
			w.Header().Set("Content-Type", "image/gif")
			w.WriteHeader(http.StatusOK)
			w.Write(transparentPixel)
			return
		}
	}

	slog.Info("Requesting cover art", "id", trackID, "fpath", fpath)

	// 1. Intentar extraer portada embebida usando dhowden/tag
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
			slog.Warn("Cover lookup: tag reading failed (dhowden/tag)", "path", fpath, "error", err)
		}
	} else {
		slog.Error("Cover lookup: failed to open file", "path", fpath, "error", err)
	}

	// 1.5 Fallback: Intentar con ffmpeg para extraer carátula embebida (muy robusto)
	slog.Debug("Cover lookup: trying ffmpeg as fallback for embedded art", "path", fpath)
	ffmpegMsg := exec.Command("ffmpeg", "-i", fpath, "-an", "-vcodec", "copy", "-f", "image2", "pipe:1")
	ffmpegOutput, err := ffmpegMsg.Output()
	if err == nil && len(ffmpegOutput) > 0 {
		// Heurística básica de tipo: si empieza por \xFF\xD8 es JPEG
		mime := "image/jpeg"
		if strings.HasPrefix(string(ffmpegOutput), "\x89PNG") {
			mime = "image/png"
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", mime)
		w.Write(ffmpegOutput)
		return
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

	slog.Warn("Cover lookup: no cover found, returning default", "path", fpath)
	
	// En lugar de dar 404 (que ensucia la consola), devolvemos un GIF transparente de 1x1
	// Esto hace que el navegador esté feliz y no marque error.
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache breve para no re-escanear tanto
	w.WriteHeader(http.StatusOK)
	w.Write(transparentPixel)
}
