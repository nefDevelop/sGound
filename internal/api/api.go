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
	"database/sql"

	"github.com/dhowden/tag"
	"github.com/disintegration/imaging"
	
	"sgound/internal/db"
	"sgound/internal/tags"
	"sgound/internal/config"
	"sgound/internal/scanner"
	"sgound/internal/metadata"
	"sgound/internal/auth"
	"sgound/internal/models"
	"sgound/web"
	"sync"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"bytes"
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
	organizeStatuses sync.Map // [string]*OrganizeStatus
}

type OrganizeStatus struct {
	IsRunning bool   `json:"isRunning"`
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Current   string `json:"current"`
}

func NewServer(masterDB *db.MasterDatabase, cfg *config.Config) *Server {
	secret, err := masterDB.GetJWTSecret()
	if err != nil {
		// Generate random secret on first run
		secret = auth.GenerateAPIKey() // Use same generator for simplicity
		masterDB.SetJWTSecret(secret)
	}

	return &Server{
		MasterDB:         masterDB,
		Config:           cfg,
		activeDBs:        make(map[string]*db.Database),
		jwtSecret:        []byte(secret),
		scanStatuses:     sync.Map{},
		organizeStatuses: sync.Map{},
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
	mux.HandleFunc("/api/library/organize", auth(s.handleOrganizeLibrary))
	mux.HandleFunc("/api/library/organize/status", auth(s.handleOrganizeStatus))
	mux.HandleFunc("/api/library/status", auth(s.handleScanStatus))
	mux.HandleFunc("/api/library/inbox", auth(s.handleGetInbox))
	mux.HandleFunc("/api/tracks/approve", auth(s.handleApproveTracks))
	mux.HandleFunc("/api/tracks/reject", auth(s.handleRejectTracks))
	mux.HandleFunc("/api/tracks/rate", auth(s.handleRateTrack))
	mux.HandleFunc("/api/lyrics", auth(s.handleLyrics))
	mux.HandleFunc("/api/lyrics/", auth(s.handleLyrics))
	mux.HandleFunc("/api/playlists", auth(s.handlePlaylists))
	mux.HandleFunc("/api/playlists/", auth(s.handlePlaylistActions))
	mux.HandleFunc("/api/smart-playlists/", auth(s.handleSmartPlaylists))
	mux.HandleFunc("/api/library/metadata", auth(s.handleLibraryMetadata))
	mux.HandleFunc("/api/autocomplete", auth(s.handleAutocomplete))
	mux.HandleFunc("/api/openapi.yaml", s.handleServeOpenAPI)
	mux.HandleFunc("/api/docs", s.handleServeSwaggerUI)

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

type dbKey string
const databaseKey dbKey = "database"

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
	w.Header().Set("Content-Type", "application/json")
	username := r.URL.Query().Get("username")
	
	if username == "" || username == "check" {
		count, _ := s.MasterDB.GetUserCount()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"setupRequired": count == 0,
			"global":        true,
		})
		return
	}

	// Check if user is in AllowedUsers in config
	perms, allowed := s.Config.AllowedUsers[username]
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not authorized in server config"})
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
	
	// Definir la ruta de la DB específica de este usuario
	userDBPath := filepath.Join(filepath.Dir(s.Config.DBPath), "library_"+body.Username+".db")

	// Create user with perms from config
	err := s.MasterDB.CreateUser(body.Username, hash, perms, userDBPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Trigger initial scan in background for the new user
	initialStatus := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store(body.Username, initialStatus)

	go func() {
		slog.Info("Starting initial background scan for auto-configuration", "user", body.Username, "path", s.Config.MusicPath)
		uDB, err := db.Connect(userDBPath)
		if err == nil {
			sc := scanner.NewScanner(uDB)
			sc.OnProgress = func(path string, count int) {
				initialStatus.Processed = count
				s.scanStatuses.Store(body.Username, initialStatus)
			}
			
			defer func() {
				initialStatus.Scanning = false
				s.scanStatuses.Store(body.Username, initialStatus)
			}()

			sc.ScanDirectory(s.Config.MusicPath)
		} else {
			slog.Error("Auto-scan failed: could not connect to user DB", "user", body.Username, "error", err)
			initialStatus.Scanning = false
			s.scanStatuses.Store(body.Username, initialStatus)
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

	if r.Method == http.MethodPatch {
		var body struct {
			Key    string `json:"key"`
			Active bool   `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		err := s.MasterDB.ToggleAPIKeyStatus(body.Key, user.Username, body.Active)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
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
					if !keyInfo.IsActive {
						http.Error(w, "API Key is blocked", http.StatusForbidden)
						return
					}
					user, _ = s.MasterDB.GetUserByUsername(keyInfo.Username)
					if user != nil {
						effectivePerms = keyInfo.Permissions
						// Extraer IP real (considerando proxies)
						ip := r.Header.Get("X-Forwarded-For")
						if ip == "" {
							ip = r.RemoteAddr
						}
						ua := r.Header.Get("User-Agent")
						// Actualizar uso en segundo plano
						go s.MasterDB.UpdateAPIKeyUsage(apiKey, ip, ua)
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

		// 4. Database Connection Guard
		// Intentamos obtener/conectar a la DB del usuario aquí de forma centralizada
		userDB := s.connectUserDB(user)
		if userDB == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "User database is temporarily unavailable (might be indexing)"})
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, databaseKey, userDB)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// connectUserDB manages the pool of active user databases
func (s *Server) connectUserDB(user *db.User) *db.Database {
	s.dbMutex.RLock()
	d, ok := s.activeDBs[user.DBPath]
	s.dbMutex.RUnlock()
	if ok {
		return d
	}
	
	s.dbMutex.Lock()
	defer s.dbMutex.Unlock()
	// Double check pattern
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

func (s *Server) getDB(r *http.Request) *db.Database {
	d, _ := r.Context().Value(databaseKey).(*db.Database)
	return d
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

	rows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT ar.id, ar.name 
		FROM artists ar
		JOIN albums al ON ar.id = al.artist_id
		JOIN tracks t ON al.id = t.album_id
		WHERE t.status = 'active'
		ORDER BY ar.name
	`)
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

	if len(artists) == 0 {
		artists = []map[string]string{}
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
	if r.Method == http.MethodGet {
		artistID := s.extractID(r, "/api/artists/")
		s.handleGetArtist(artistID, w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetArtist(artistID string, w http.ResponseWriter, r *http.Request) {
	var name string
	err := s.getDB(r).Conn.QueryRow("SELECT name FROM artists WHERE id = ?", artistID).Scan(&name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var albumCount, trackCount int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT id) FROM albums WHERE artist_id = ?", artistID).Scan(&albumCount)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks t JOIN albums a ON t.album_id = a.id WHERE a.artist_id = ? AND t.status = 'active'", artistID).Scan(&trackCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": artistID, "name": name, "albumCount": albumCount, "trackCount": trackCount,
	})
}

func (s *Server) handleGetArtistAlbums(artistID string, w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT al.id, al.title, COALESCE(al.year, 0) 
		FROM albums al
		JOIN tracks t ON al.id = t.album_id
		WHERE al.artist_id = ? AND t.status = 'active'
		ORDER BY al.year DESC
	`, artistID)
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

	if len(albums) == 0 {
		albums = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albums)
}

func (s *Server) handleGetAlbums(w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT al.id, al.title, COALESCE(al.year, 0), ar.name as artist, 
		       (SELECT t2.id FROM tracks t2 WHERE t2.album_id = al.id AND t2.status = 'active' LIMIT 1) as sample_track_id
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON al.id = t.album_id
		WHERE t.status = 'active'
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

	// Obtener ID de la primera canción para la carátula
	var coverId string
	s.getDB(r).Conn.QueryRow("SELECT id FROM tracks WHERE album_id = ? LIMIT 1", albumID).Scan(&coverId)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": albumID, "title": title, "artist": artist, "year": year, "coverId": coverId,
	})
}

func (s *Server) handleGetAlbumTracks(albumID string, w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT id, title, track_num, duration, genre 
		FROM tracks 
		WHERE album_id = ? AND status = 'active'
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

	if len(tracks) == 0 {
		tracks = []map[string]interface{}{}
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

func (s *Server) handleOrganizeLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := r.Context().Value(userKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Evitar que corran dos al mismo tiempo
	if status, ok := s.organizeStatuses.Load(user); ok && status.(*OrganizeStatus).IsRunning {
		http.Error(w, "Organization already in progress", http.StatusConflict)
		return
	}

	db := s.getDB(r)
	var total int
	db.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&total)

	status := &OrganizeStatus{
		IsRunning: true,
		Total:     total,
		Processed: 0,
		Current:   "Counting files...",
	}
	s.organizeStatuses.Store(user, status)

	// Lógica en segundo plano
	go func() {
		rows, err := db.Conn.Query("SELECT id, title FROM tracks")
		if err != nil {
			status.IsRunning = false
			return
		}
		defer rows.Close()

		editor := tags.NewEditor(db)
		for rows.Next() {
			var id, title string
			if err := rows.Scan(&id, &title); err == nil {
				status.Current = title
				editor.OrganizeFile(id, s.Config.MusicPath, s.Config.RenamePattern)
				status.Processed++
			}
		}
		status.IsRunning = false
		status.Current = "Done"
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) handleOrganizeStatus(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(string)
	status, ok := s.organizeStatuses.Load(user)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OrganizeStatus{IsRunning: false})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
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

	genre := r.URL.Query().Get("genre")
	if genre != "" {
		if status == "all" {
			whereClause += " AND t.genre = ?"
		} else {
			whereClause += " AND t.genre = ?"
		}
		args = append(args, genre)
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

	for _, oldID := range data.IDs {
		// Al aprobar, generamos el ID persistente REAL y lo escribimos en el archivo
		db := s.getDB(r)
		var path string
		err := db.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", oldID).Scan(&path)
		if err != nil { continue }

		newID := tags.GenerateUUID()
		
		// Lógica de Carátulas: Si no tiene interna, buscar local e incrustar
		go func(p string, tid string) {
			f, err := os.Open(p)
			if err != nil { return }
			defer f.Close()
			metadata, _ := tag.ReadFrom(f)
			if metadata == nil || metadata.Picture() == nil {
				// No tiene interna, buscamos local
				dir := filepath.Dir(p)
				common := []string{"cover", "folder", "front", "album"}
				entries, _ := os.ReadDir(dir)
				for _, ent := range entries {
					name := strings.ToLower(ent.Name())
					ext := filepath.Ext(name)
					if ext != ".jpg" && ext != ".jpeg" && ext != ".png" { continue }
					base := strings.TrimSuffix(name, ext)
					for _, c := range common {
						if base == c {
							imgData, _ := os.ReadFile(filepath.Join(dir, ent.Name()))
							mime := "image/jpeg"
							if ext == ".png" { mime = "image/png" }
							tags.NewEditor(s.getDB(r)).EmbedCover(tid, imgData, mime)
							return
						}
					}
				}
			}
		}(path, oldID)

		if err := tags.WritePersistentID(path, newID); err == nil {
			// Si se escribió correctamente, actualizamos el ID en la DB y lo activamos
			// Usamos una transacción o simplemente actualizamos por partes
			_, err = db.Conn.Exec("UPDATE tracks SET id = ?, status = 'active' WHERE id = ?", newID, oldID)
			if err != nil {
				slog.Error("Failed to update track ID in DB after physical write", "oldID", oldID, "newID", newID, "err", err)
			} else {
				// Si el track tenía tags en AutoRename, lo organizamos ahora que es oficial
				if s.Config.AutoRename {
					tags.NewEditor(db).OrganizeFile(newID, s.Config.MusicPath, s.Config.RenamePattern)
				}
			}
		} else {
			slog.Error("Failed to write persistent ID to file during approval", "path", path, "err", err)
			// Fallback: lo activamos con el ID antiguo si falla la escritura física (para no perder el track)
			db.Conn.Exec("UPDATE tracks SET status = 'active' WHERE id = ?", oldID)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
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
		// Mark as rejected so scanner doesn't pick it up as pending again
		_, err := s.getDB(r).Conn.Exec("UPDATE tracks SET status = 'rejected' WHERE id = ?", id)
		if err != nil {
			slog.Error("Failed to reject track", "id", id, "error", err)
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
	db := s.getDB(r)
	
	// 1. Obtener la ruta del archivo antes de borrar el registro
	var path string
	err := db.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Error fetching track path", http.StatusInternalServerError)
		}
		return
	}

	// 2. Intentar borrar el archivo físico
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to delete physical file", "path", path, "error", err)
		// Continuamos para limpiar la DB aunque falle el borrado físico (por si el archivo ya no existía)
	}

	// 3. Borrar de la base de datos
	_, err = db.Conn.Exec("DELETE FROM tracks WHERE id = ?", trackID)
	if err != nil {
		http.Error(w, "Failed to delete track from DB", http.StatusInternalServerError)
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

	// 1. Search Tracks (por título o género) - Solo activas
	tracksRows, err := s.getDB(r).Conn.Query(`
		SELECT t.id, t.title, a.name as artist 
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		WHERE (t.title LIKE ? OR t.genre LIKE ?) AND t.status = 'active'
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

	// 2. Search Artists - Solo si tienen pistas activas
	artistRows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT ar.id, ar.name 
		FROM artists ar
		JOIN albums al ON ar.id = al.artist_id
		JOIN tracks t ON al.id = t.album_id
		WHERE ar.name LIKE ? AND t.status = 'active'
		LIMIT 20
	`, searchTerm)
	
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

	// 3. Search Albums - Solo si tienen pistas activas
	albumRows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT al.id, al.title, ar.name as artist 
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON al.id = t.album_id
		WHERE al.title LIKE ? AND t.status = 'active'
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
	// 4. Search Genres
	genreRows, err := s.getDB(r).Conn.Query(`
		SELECT DISTINCT genre 
		FROM tracks 
		WHERE genre LIKE ? AND status = 'active'
		LIMIT 10
	`, searchTerm)
	
	var genres []string
	if err == nil {
		defer genreRows.Close()
		for genreRows.Next() {
			var g string
			genreRows.Scan(&g)
			if g != "" {
				genres = append(genres, g)
			}
		}
	}
	results["genres"] = genres

	// 5. Search Playlists
	playlistRows, err := s.getDB(r).Conn.Query(`
		SELECT id, name 
		FROM playlists 
		WHERE name LIKE ?
		LIMIT 10
	`, searchTerm)
	
	var playlists []map[string]interface{}
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
		query = "SELECT DISTINCT ar.name FROM artists ar JOIN albums al ON ar.id = al.artist_id JOIN tracks t ON al.id = t.album_id WHERE ar.name LIKE ? ORDER BY ar.name LIMIT 10"
	case "genre":
		query = "SELECT DISTINCT genre FROM tracks WHERE genre LIKE ? ORDER BY genre LIMIT 10"
	case "album":
		query = "SELECT DISTINCT al.title FROM albums al JOIN tracks t ON al.id = t.album_id WHERE al.title LIKE ? ORDER BY al.title LIMIT 10"
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
	// Obtener décadas únicas - Solo activas
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

	// Obtener géneros únicos - Solo activas
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

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})
	
	// Contadores básicos - Filtrando por status='active'
	var tTracks, tAlbums, tArtists, tPlaylists, tGenres int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'active'").Scan(&tTracks)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT album_id) FROM tracks WHERE status = 'active'").Scan(&tAlbums)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT ar.id) FROM artists ar JOIN albums al ON ar.id = al.artist_id JOIN tracks t ON al.id = t.album_id WHERE t.status = 'active'").Scan(&tArtists)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM playlists").Scan(&tPlaylists)
	s.getDB(r).Conn.QueryRow("SELECT COUNT(DISTINCT genre) FROM tracks WHERE status = 'active' AND genre != ''").Scan(&tGenres)
	stats["totalTracks"] = tTracks
	stats["totalAlbums"] = tAlbums
	stats["totalArtists"] = tArtists
	stats["totalPlaylists"] = tPlaylists
	stats["totalGenres"] = tGenres

	// Inbox count (pendientes de aprobar)
	var pendingCount int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'pending'").Scan(&pendingCount)
	stats["inboxCount"] = pendingCount

	// Joyas olvidadas (9 canciones aleatorias para redescubrir) - Solo activas
	gems := make([]map[string]interface{}, 0)
	rows, err := s.getDB(r).Conn.Query(`
		SELECT id, title, (SELECT name FROM artists WHERE id = (SELECT artist_id FROM albums WHERE id = tracks.album_id)) as artist 
		FROM tracks 
		WHERE status = 'active'
		ORDER BY RANDOM() LIMIT 9
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, artist string
			rows.Scan(&id, &title, &artist)
			gems = append(gems, map[string]interface{}{"id": id, "title": title, "artist": artist})
		}
	}
	stats["forgottenGems"] = gems

	// Lo más escuchado (Top 10) - Solo activas
	topPlays := make([]map[string]interface{}, 0)
	rows2, err := s.getDB(r).Conn.Query(`
		SELECT id, title, (SELECT name FROM artists WHERE id = (SELECT artist_id FROM albums WHERE id = tracks.album_id)) as artist, play_count 
		FROM tracks 
		WHERE play_count > 0 AND status = 'active'
		ORDER BY play_count DESC LIMIT 10
	`)
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

	// Recientemente añadidos con Lógica de Discovery (Moods/Energy)
	recentAlbums := make([]map[string]interface{}, 0)
	rows3, err := s.getDB(r).Conn.Query(`
		SELECT a.id, a.title, art.name, MAX(t.created_at) as added_at, 
		       COALESCE(MAX(t.genre), '') as genre, MAX(t.bit_rate) as bitrate, MAX(t.play_count) as pc
		FROM albums a
		JOIN artists art ON a.artist_id = art.id
		JOIN tracks t ON a.id = t.album_id
		WHERE t.status = 'active'
		GROUP BY a.id, a.title, art.name
		ORDER BY added_at DESC LIMIT 6
	`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var id, title, artist, addedAt, genre string
			var br, pc int
			rows3.Scan(&id, &title, &artist, &addedAt, &genre, &br, &pc)
			
			// Algoritmo de Mood/Tag Refinado
			tag := "Recuerdo"
			color := "tertiary"
			g := strings.ToLower(genre)
			
			if pc == 0 {
				tag = "Descubrir"
				color = "secondary"
			} else if strings.Contains(g, "rock") || strings.Contains(g, "pop") || strings.Contains(g, "electronic") || strings.Contains(g, "metal") {
				tag = "Energy"
				color = "primary"
			} else if strings.Contains(g, "jazz") || strings.Contains(g, "classical") {
				tag = "Inspiración"
				color = "secondary"
			} else if strings.Contains(g, "chill") || strings.Contains(g, "ambient") || strings.Contains(g, "lo-fi") || strings.Contains(g, "blues") {
				tag = "Chill"
				color = "tertiary"
			} else if br > 1000000 {
				tag = "Premium"
				color = "primary"
			}

			recentAlbums = append(recentAlbums, map[string]interface{}{
				"id": id, 
				"title": title, 
				"artist": artist, 
				"addedAt": addedAt,
				"tag": tag,
				"tagColor": color,
			})
		}
	}
	stats["discovery"] = recentAlbums
	
	// Trends: Top 3 genres by track count
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
				trends = append(trends, map[string]interface{}{
					"genre":      g,
					"percentage": percentage,
				})
			}
		}
	}
	stats["trends"] = trends

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

	var incomplete int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE (genre IS NULL OR genre = '') AND status = 'active'").Scan(&incomplete)
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

	// Agrupar duplicados
	groups := make(map[string][]models.Track)
	for _, t := range dups {
		key := ""
		if t.FileHash != "" {
			key = "hash:" + t.FileHash
		} else if t.Duration > 0 {
			key = fmt.Sprintf("dur:%s:%d", strings.ToLower(t.Title), t.Duration)
		} else {
			key = fmt.Sprintf("tag:%s:%s:%s", strings.ToLower(t.Artist), strings.ToLower(t.Album), strings.ToLower(t.Title))
		}
		groups[key] = append(groups[key], t)
	}

	// Convertir a lista de grupos para el frontend
	var result [][]models.Track
	for _, g := range groups {
		if len(g) > 1 {
			result = append(result, g)
		}
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
		if body.IsSmart {
			isSmart = 1
		}

		ruleSQL := ""
		if body.IsSmart {
			ruleSQL = s.generateRuleSQL(body.Rules)
		}

		_, err := s.getDB(r).Conn.Exec("INSERT INTO playlists (id, name, is_smart, rule_sql) VALUES (?, ?, ?, ?)", id, body.Name, isSmart, ruleSQL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	rows, err := s.getDB(r).Conn.Query("SELECT id, name, is_smart, rule_sql FROM playlists")
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
		list = append(list, map[string]interface{}{
			"id": id, "name": name, "isSmart": isSmart == 1,
		})
	}
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
		// SQLite compatible date substract
		parts = append(parts, fmt.Sprintf("(last_played IS NULL OR last_played < datetime('now', '-%d days'))", rules.LastPlayedDays))
	}
	if rules.Instrumental != nil {
		val := 0
		if *rules.Instrumental {
			val = 1
		}
		parts = append(parts, fmt.Sprintf("instrumental = %d", val))
	}

	if len(parts) == 0 {
		return "SELECT id FROM tracks WHERE status = 'active'"
	}

	return "SELECT id FROM tracks WHERE status = 'active' AND " + strings.Join(parts, " AND ")
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
	
	whereClause := "WHERE t.status = 'active'"
	var orderClause string
	var args []interface{}

	switch typeStr {
	case "recently-played":
		whereClause += " AND t.last_played IS NOT NULL"
		orderClause = "ORDER BY t.last_played DESC LIMIT 50"
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
	// 1. Obtener info de la playlist
	var isSmart int
	var ruleSQL string
	err := s.getDB(r).Conn.QueryRow("SELECT is_smart, rule_sql FROM playlists WHERE id = ?", id).Scan(&isSmart, &ruleSQL)
	if err != nil {
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	var rows *sql.Rows
	if isSmart == 1 && ruleSQL != "" {
		// Ejecutar regla dinámica
		rows, err = s.getDB(r).Conn.Query(`
			SELECT t.id, t.title, a.name, al.title, t.duration
			FROM tracks t
			JOIN albums al ON t.album_id = al.id
			JOIN artists a ON al.artist_id = a.id
			WHERE t.id IN (` + ruleSQL + `)
			ORDER BY a.name, al.title, t.track_num
		`)
	} else {
		// Lista estática tradicional
		rows, err = s.getDB(r).Conn.Query(`
			SELECT t.id, t.title, a.name, al.title, t.duration
			FROM tracks t
			JOIN playlist_tracks pt ON t.id = pt.track_id
			JOIN albums al ON t.album_id = al.id
			JOIN artists a ON al.artist_id = a.id
			WHERE pt.playlist_id = ? AND t.status = 'active'
			ORDER BY pt.position ASC
		`, id)
	}

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

	trackID := s.extractID(r, "/api/cover")
	if trackID == "" {
		http.NotFound(w, r)
		return
	}

	// 0. VERIFICAR CACHÉ (VELOCIDAD MÁXIMA)
	cacheDir := filepath.Join(os.TempDir(), "sgound_covers")
	os.MkdirAll(cacheDir, 0755)
	cachePath := filepath.Join(cacheDir, trackID+".jpg")

	if _, err := os.Stat(cachePath); err == nil {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, cachePath)
		return
	}

	var fpath string
	err := s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&fpath)
	if err != nil {
		err = s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE album_id = ? LIMIT 1", trackID).Scan(&fpath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// 1. Intentar extraer embebida ligera
	f, err := os.Open(fpath)
	if err == nil {
		defer f.Close()
		metadata, err := tag.ReadFrom(f)
		if err == nil {
			if pic := metadata.Picture(); pic != nil {
				// Redimensionar antes de cachear
				src, _, err := image.Decode(bytes.NewReader(pic.Data))
				if err == nil {
					resized := imaging.Resize(src, 600, 0, imaging.Lanczos)
					imaging.Save(resized, cachePath)
					w.Header().Set("Cache-Control", "public, max-age=604800")
					w.Header().Set("Content-Type", "image/jpeg")
					imaging.Encode(w, resized, imaging.JPEG)
					return
				}
				// Fallback si falla el decode: guardar original
				os.WriteFile(cachePath, pic.Data, 0644)
				w.Header().Set("Cache-Control", "public, max-age=604800")
				w.Header().Set("Content-Type", pic.MIMEType)
				w.Write(pic.Data)
				return
			}
		}
	}

	// 2. Intentar buscar archivo local (cover.jpg, etc)
	dir := filepath.Dir(fpath)
	commonNames := map[string]bool{"cover": true, "folder": true, "front": true, "album": true, "artwork": true}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		ext := filepath.Ext(name)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" { continue }
		base := strings.TrimSuffix(name, ext)
		if commonNames[base] {
			imgData, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
			// Redimensionar antes de cachear
			src, _, err := image.Decode(bytes.NewReader(imgData))
			if err == nil {
				resized := imaging.Resize(src, 600, 0, imaging.Lanczos)
				imaging.Save(resized, cachePath)
				w.Header().Set("Cache-Control", "public, max-age=604800")
				w.Header().Set("Content-Type", "image/jpeg")
				imaging.Encode(w, resized, imaging.JPEG)
				return
			}
			// Fallback
			os.WriteFile(cachePath, imgData, 0644)
			w.Header().Set("Cache-Control", "public, max-age=604800")
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(imgData)
			return
		}
	}

	// 3. Fallback: No hay carátula.
	http.NotFound(w, r)
}

func (s *Server) handleServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "openapi.yaml")
}

func (s *Server) handleServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Sgound API Docs</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" >
    <style>
      html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
      *, *:before, *:after { box-sizing: inherit; }
      body { margin:0; background: #fafafa; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"> </script>
    <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "/api/openapi.yaml",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
        ],
      })
      window.ui = ui
    }
    </script>
</body>
</html>
	`)
}

func (s *Server) HandleFileEvent(path string) {
	slog.Info("Watcher: Detected change", "path", path)
	
	// Pre-check if file exists
	if _, err := os.Stat(path); err != nil {
		return
	}

	users, err := s.MasterDB.GetUsers()
	if err != nil {
		slog.Error("Watcher: Failed to get users", "error", err)
		return
	}

	for _, user := range users {
		// We use a local copy to avoid pointer issues in the loop
		u := user
		uDB := s.connectUserDB(&u)
		if uDB == nil {
			continue
		}
		
		sc := scanner.NewScanner(uDB)
		if err := sc.ParseAndSaveFile(path); err != nil {
			slog.Error("Watcher: Failed to process file", "user", u.Username, "path", path, "error", err)
		}
	}
}

func (s *Server) TriggerGlobalScan() {
	slog.Info("Starting scheduled global scan")
	users, err := s.MasterDB.GetUsers()
	if err != nil {
		slog.Error("Scheduled scan: failed to get users", "error", err)
		return
	}

	for _, user := range users {
		u := user
		uDB := s.connectUserDB(&u)
		if uDB == nil {
			continue
		}
		
		go func(username string, db *db.Database) {
			slog.Info("Scheduled scan: processing user", "user", username)
			sc := scanner.NewScanner(db)
			if err := sc.ScanDirectory(s.Config.MusicPath); err != nil {
				slog.Error("Scheduled scan: failed for user", "user", username, "error", err)
			}
		}(u.Username, uDB)
	}
}
