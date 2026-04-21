package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math/rand/v2"
	"sgound/internal/auth"
	"sgound/internal/config"
	"sgound/internal/db"
	"sgound/internal/metadata"
	"sgound/internal/models"
	"sgound/internal/scanner"
	"sgound/internal/tags"
	"sgound/internal/watcher"
	"sgound/web"
	"sync"
	"sync/atomic"
)

type ScanStatus struct {
	Scanning  bool `json:"scanning"`
	Processed int  `json:"processed"`
	RawCount  int  `json:"rawCount"`
}

type Server struct {
	MasterDB     *db.MasterDatabase
	Config       *config.Config
	Watcher      *watcher.Watcher
	externalMeta *metadata.ExternalMetadata

	jwtSecret        []byte
	scanStatuses     sync.Map // [string]*ScanStatus
	organizeStatuses sync.Map // [string]*OrganizeStatus
	inboxCount       atomic.Int64
	rawCount         atomic.Int64
	activeScanner    *scanner.Scanner // Referencia al escáner en curso
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

	s := &Server{
		MasterDB:         masterDB,
		Config:           cfg,
		externalMeta:     metadata.NewExternalMetadata(cfg),
		jwtSecret:        []byte(secret),
		scanStatuses:     sync.Map{},
		organizeStatuses: sync.Map{},
	}

	// Inicializar contador
	s.updateInboxCount()

	return s
}

func (s *Server) updateInboxCount() {
	var pending, raw int64
	_ = s.MasterDB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'pending'").Scan(&pending)
	_ = s.MasterDB.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'raw'").Scan(&raw)
	
	s.inboxCount.Store(pending)
	s.rawCount.Store(raw)
	slog.Debug("📥 Cache updated", "pending", pending, "raw", raw)
}

type contextKey string

const userKey contextKey = "user"

func (s *Server) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Helper to wrap with auth
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return s.authMiddleware(h)
	}

	// 1. REST Core API
	mux.HandleFunc("/api/artists", auth(s.handleGetArtists))
	mux.HandleFunc("/api/artists/", auth(s.handleArtistActions))
	mux.HandleFunc("/api/artists/{id}/image", auth(s.handleGetArtistImage))
	mux.HandleFunc("/api/albums", auth(s.handleGetAlbums))
	mux.HandleFunc("/api/albums/", auth(s.handleAlbumActions))
	mux.HandleFunc("/api/stream", auth(s.handleStream))
	mux.HandleFunc("/api/stream/", auth(s.handleStream))
	mux.HandleFunc("/api/cover", auth(s.handleGetCover))
	mux.HandleFunc("/api/cover/", auth(s.handleGetCover))
	mux.HandleFunc("/api/tracks", auth(s.handleGetTracks))
	mux.HandleFunc("/api/tracks/", auth(s.handleTrackActions))
	mux.HandleFunc("/api/tracks/{id}/cover-url", auth(s.handleSetCoverURL))
	mux.HandleFunc("/api/scan", auth(s.handleScan))
	mux.HandleFunc("/api/config", auth(s.handleConfig))
	mux.HandleFunc("/api/search", auth(s.handleSearch))
	mux.HandleFunc("/api/dashboard", auth(s.handleDashboard))
	mux.HandleFunc("/api/library/health", auth(s.handleLibraryHealth))
	mux.HandleFunc("/api/library/duplicates", auth(s.handleLibraryDuplicates))
	mux.HandleFunc("/api/library/orphans", auth(s.handleLibraryOrphans))
	mux.HandleFunc("/api/library/low-quality", auth(s.handleLibraryLowQuality))
	mux.HandleFunc("/api/library/missing-covers", auth(s.handleLibraryMissingCovers))
	mux.HandleFunc("/api/library/small-covers", auth(s.handleLibrarySmallCovers))
	mux.HandleFunc("/api/library/huge-covers", auth(s.handleLibraryHugeCovers))
	mux.HandleFunc("/api/library/missing-tags", auth(s.handleLibraryMissingTags))
	mux.HandleFunc("/api/library/normalize-cover", auth(s.handleNormalizeCover))
	mux.HandleFunc("/api/library/cleanup", auth(s.handleLibraryCleanup))
	mux.HandleFunc("/api/library/organize", auth(s.handleOrganizeLibrary))
	mux.HandleFunc("/api/library/organize/status", auth(s.handleOrganizeStatus))
	mux.HandleFunc("/api/library/status", auth(s.handleScanStatus))
	mux.HandleFunc("/api/library/inbox", auth(s.handleGetInbox))
	mux.HandleFunc("/api/library/inbox/prioritize", auth(s.handlePrioritizeTrack))
	mux.HandleFunc("/api/library/inbox/analyze", auth(s.handleAnalyzeInbox))
	mux.HandleFunc("/api/library/inbox/apply-suggestions", auth(s.handleApplyInboxSuggestions))
	mux.HandleFunc("/api/tracks/approve", auth(s.handleApproveTracks))
	mux.HandleFunc("/api/tracks/reject", auth(s.handleRejectTracks))
	mux.HandleFunc("/api/tracks/rate", auth(s.handleRateTrack))
	mux.HandleFunc("/api/lyrics", auth(s.handleLyrics))
	mux.HandleFunc("/api/lyrics/", auth(s.handleLyrics))
	mux.HandleFunc("/api/playlists", auth(s.handlePlaylists))
	mux.HandleFunc("/api/playlists/", auth(s.handlePlaylistActions))
	mux.HandleFunc("/api/smart-playlists/", auth(s.handleSmartPlaylists))
	mux.HandleFunc("/api/library/metadata", auth(s.handleLibraryMetadata))
	mux.HandleFunc("/api/metadata/suggest", auth(s.handleMetadataSuggest))
	mux.HandleFunc("/api/metadata/search", auth(s.handleMetadataSearch))
	mux.HandleFunc("/api/metadata/search-album", auth(s.handleMetadataSearchAlbum))
	mux.HandleFunc("/api/metadata/release-details", auth(s.handleGetReleaseDetails))
	mux.HandleFunc("/api/autocomplete", auth(s.handleAutocomplete))
	mux.HandleFunc("/api/logs", auth(s.handleGetLogs))
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

	return s.securityHeaders(mux)
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
		Secure:   false, // Set to true if using HTTPS
		SameSite: http.SameSiteLaxMode,
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

	// Create user with perms from config
	err := s.MasterDB.CreateUser(body.Username, hash, perms, "", "*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	if user == nil {
		return
	}

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

		// 1. Check JWT Cookie (Web Session)
		if cookie, err := r.Cookie("sgound_token"); err == nil {
			claims, err := auth.VerifyJWT(cookie.Value, s.jwtSecret)
			if err == nil {
				user, _ = s.MasterDB.GetUserByUsername(claims.Username)
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		// 3. Permission Guard for Mutation Methods
		if r.Method != http.MethodGet && user.Permissions == "r" {
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

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' unpkg.com; style-src 'self' 'unsafe-inline' unpkg.com fonts.googleapis.com; font-src 'self' data: fonts.gstatic.com; img-src 'self' data: itunes.apple.com; connect-src 'self' lrclib.net api.lyrics.ovh itunes.apple.com")

		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) getDB(_ *http.Request) *db.MasterDatabase {
	return s.MasterDB
}

func (s *Server) getPathFilter(user *db.User) (string, []interface{}) {
	if user == nil || user.AllowedPaths == "*" || user.AllowedPaths == "" {
		return "", nil
	}

	paths := strings.Split(user.AllowedPaths, ",")
	var conditions []string
	var args []interface{}

	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Asegurar que termina en barra para el LIKE
		if !strings.HasSuffix(p, "/") && !strings.HasSuffix(p, "\\") {
			p = p + "/"
		}
		conditions = append(conditions, "t.path LIKE ?")
		args = append(args, p+"%")
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "(" + strings.Join(conditions, " OR ") + ")", args
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

func (s *Server) EnrichArtists() {
	go func() {
		slog.Info("🚀 Background artist enrichment started")
		// Solo enriquecer artistas que tengan al menos un track activo (aprobado)
		query := `
			SELECT DISTINCT a.id, a.name 
			FROM artists a
			JOIN albums al ON a.id = al.artist_id
			JOIN tracks t ON al.id = t.album_id
			WHERE t.status = 'active' AND (
				(a.image_url IS NULL OR a.image_url = '') OR 
				(a.bio IS NULL OR a.bio = '') OR 
				(a.tags IS NULL OR a.tags = '')
			)`
		rows, err := s.MasterDB.Conn.Query(query)
		if err != nil {
			slog.Error("Failed to query artists for enrichment", "error", err)
			return
		}
		defer rows.Close()

		var artists []struct{ ID, Name string }
		for rows.Next() {
			var a struct{ ID, Name string }
			if err := rows.Scan(&a.ID, &a.Name); err == nil {
				artists = append(artists, a)
			}
		}

		if len(artists) == 0 {
			slog.Info("✨ All artists are already enriched")
			return
		}

		slog.Info("📈 Enrolling artists for enrichment", "count", len(artists))

		for _, a := range artists {
			// Skip generic names that won't have meaningful metadata
			lowerName := strings.ToLower(a.Name)
			if lowerName == "various artists" || lowerName == "unknown artist" || lowerName == "unknown" {
				continue
			}

			ext, err := s.externalMeta.GetArtistInfo(a.Name)
			if err == nil && ext != nil {
				tagsStr := strings.Join(ext.Tags, ",")
				similarStr := strings.Join(ext.Similar, ",")
				_, _ = s.MasterDB.Conn.Exec("UPDATE artists SET image_url = ?, bio = ?, tags = ?, similar = ? WHERE id = ?",
					ext.Image, ext.Biography, tagsStr, similarStr, a.ID)
				slog.Debug("✨", "artist", a.Name)
			}
			// Wait to avoid Wikipedia/Last.fm rate limits
			time.Sleep(1500 * time.Millisecond)
		}
		slog.Info("🏁 Background enrichment finished")
	}()
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Evitar escaneos duplicados
	if val, ok := s.scanStatuses.Load("global"); ok {
		if val.(*ScanStatus).Scanning {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"already scanning"}`))
			return
		}
	}

	// Status global de escaneo
	status := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store("global", status)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"global scan started"}`))

	go func() {
		// Actualizar contador inmediatamente al empezar por si ya hay tracks raw/pending
		s.updateInboxCount()

		sc := scanner.NewScanner(s.MasterDB)
		s.activeScanner = sc // Guardar referencia activa

		sc.OnDiscovery = func(count int) {
			// Cuando se descubren archivos (raw), actualizamos el contador para que aparezca el Inbox
			s.updateInboxCount()
		}

		sc.OnProgress = func(path string, count int) {
			// Throttling: Solo actualizamos el estado cada 5 archivos para que la UI sea más fluida
			if count%5 == 0 {
				status.Processed = count
				s.scanStatuses.Store("global", status)
				// Actualizar caché de inbox cada vez que avanzamos para que el UI lo vea
				s.updateInboxCount()
			}
		}

		defer func() {
			status.Scanning = false
			s.scanStatuses.Store("global", status)
			s.activeScanner = nil // Limpiar referencia
			s.updateInboxCount() // Actualización final
			// Trigger enrichment for new artists
			s.EnrichArtists()
		}()
		if err := sc.ScanDirectory(s.Config.MusicPath); err != nil {
			slog.Error("Global scan failed", "error", err)
		}
	}()
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
			Language          string `json:"language"`
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
		s.Config.Language = newCfg.Language
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

	var artists []models.Artist
	for rows.Next() {
		var a models.Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.ImageURL); err != nil {
			continue
		}
		artists = append(artists, a)
	}

	if len(artists) == 0 {
		artists = []models.Artist{}
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

	// Clean empty slices from Split
	if cachedTags.String == "" {
		tags = []string{}
	}
	if cachedSimilar.String == "" {
		similar = []string{}
	}

	// If no cached info or tags, fetch it
	if biography == "" || externalImage == "" || len(tags) == 0 {
		ext, _ := s.externalMeta.GetArtistInfo(name)
		if ext != nil {
			biography = ext.Biography
			externalImage = ext.Image
			tags = ext.Tags
			similar = ext.Similar
			// Update cache in DB
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

func (s *Server) handleGetArtistImage(w http.ResponseWriter, r *http.Request) {
	artistID := s.extractID(r, "/api/artists/")
	// s.extractID will return something like "artist_id/image". We just want the ID.
	artistID = strings.TrimSuffix(artistID, "/image")

	if artistID == "" {
		s.servePlaceholderArtist(w)
		return
	}

	// 1. Verificar Caché (VELOCIDAD MÁXIMA)
	cachePath := filepath.Join(s.Config.CachePath, "artists", artistID+".jpg")
	os.MkdirAll(filepath.Dir(cachePath), 0755)

	if _, err := os.Stat(cachePath); err == nil {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, cachePath)
		return
	}

	// 2. Obtener URL externa de la DB
	var imageURL string
	var name string
	err := s.getDB(r).Conn.QueryRow("SELECT image_url, name FROM artists WHERE id = ?", artistID).Scan(&imageURL, &name)
	if err != nil {
		s.servePlaceholderArtist(w)
		return
	}

	if imageURL == "" {
		// Intentar enriquecer bajo demanda
		ext, _ := s.externalMeta.GetArtistInfo(name)
		if ext != nil && ext.Image != "" {
			imageURL = ext.Image
			tagsStr := strings.Join(ext.Tags, ",")
			similarStr := strings.Join(ext.Similar, ",")
			_, _ = s.getDB(r).Conn.Exec("UPDATE artists SET image_url = ?, bio = ?, tags = ?, similar = ? WHERE id = ?",
				ext.Image, ext.Biography, tagsStr, similarStr, artistID)
		} else {
			s.servePlaceholderArtist(w)
			return
		}
	}

	// 3. Descargar y Procesar
	resp, err := http.Get(imageURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		s.servePlaceholderArtist(w)
		return
	}
	defer resp.Body.Close()

	// Limit reading to 10MB to avoid memory exhaustion
	limit := int64(10 * 1024 * 1024)
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	src, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		// Fallback: guardar original si no podemos decodificar (quizás es formato extraño)
		os.WriteFile(cachePath, imgData, 0644)
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Write(imgData)
		return
	}

	// Redimensionar a 800px de ancho (proporcional)
	resized := imaging.Resize(src, 800, 0, imaging.Lanczos)

	// Guardar en caché como JPEG optimizado
	err = imaging.Save(resized, cachePath)
	if err != nil {
		slog.Error("Failed to save resized artist image", "error", err)
	}

	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("Content-Type", "image/jpeg")
	imaging.Encode(w, resized, imaging.JPEG)
}

func (s *Server) servePlaceholderArtist(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 24 24" fill="none" stroke="#666" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>`)
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
		SELECT DISTINCT al.id, al.title, COALESCE(al.year, 0) 
		FROM albums al
		JOIN tracks t ON al.id = t.album_id
		%s
		ORDER BY al.year DESC`, whereClause)

	rows, err := s.getDB(r).Conn.Query(query, args...)
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

	// Obtener ID de la primera canción para la carátula
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
	user, _ := r.Context().Value(userKey).(*db.User)

	var fpath string
	var rg float64
	err := s.getDB(r).Conn.QueryRow("SELECT path, COALESCE(replay_gain, 0.0) FROM tracks WHERE id = ?", trackID).Scan(&fpath, &rg)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// Security: Validate path is within music directory
	absMusicPath, _ := filepath.Abs(s.Config.MusicPath)
	absTrackPath, _ := filepath.Abs(fpath)
	if !strings.HasPrefix(absTrackPath, absMusicPath) {
		slog.Error("Unauthorized file access attempt", "path", fpath, "user", user.Username)
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Registrar reproducción INDIVIDUAL
	go func() {
		s.MasterDB.UpdateTrackUserData(user.Username, trackID, -1, true)
	}()

	bitrate := r.URL.Query().Get("bitrate")
	useFFmpeg := false
	ffmpegArgs := []string{"-i", fpath}

	// 1. ReplayGain Filter
	if s.Config.ReplayGainEnabled && rg != 0 {
		useFFmpeg = true
		ffmpegArgs = append(ffmpegArgs, "-af", fmt.Sprintf("volume=%.2fdB", rg))
	}

	// 2. Transcoding / Bitrate
	if bitrate != "" && bitrate != "original" {
		useFFmpeg = true
		// Basic validation
		for _, char := range bitrate {
			if !((char >= '0' && char <= '9') || char == 'k' || char == 'm') {
				http.Error(w, "Invalid bitrate format", http.StatusBadRequest)
				return
			}
		}
		ffmpegArgs = append(ffmpegArgs, "-ab", bitrate)
	}

	if useFFmpeg {
		w.Header().Set("Content-Type", "audio/mpeg")
		// Force MP3 for compatibility when processing with ffmpeg
		ffmpegArgs = append(ffmpegArgs, "-f", "mp3", "-map_metadata", "-1", "pipe:1")

		// Usamos CommandContext para que el proceso muera si la petición se cancela
		cmd := exec.CommandContext(r.Context(), "ffmpeg", ffmpegArgs...)
		cmd.Stdout = w
		if err := cmd.Start(); err != nil {
			slog.Error("FFmpeg streaming failed", "error", err, "args", ffmpegArgs)
			http.Error(w, "Streaming failed", http.StatusInternalServerError)
			return
		}

		// Esperar a que termine o sea cancelado por el contexto
		err := cmd.Wait()
		if err != nil && r.Context().Err() == nil {
			slog.Error("FFmpeg process finished with error", "error", err)
		}
		return
	}

	http.ServeFile(w, r, fpath)
}
func (s *Server) handleSetCoverURL(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/tracks/")
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 1. Descargar imagen
	resp, err := http.Get(body.URL)
	if err != nil {
		http.Error(w, "Failed to download image", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	mimeType := resp.Header.Get("Content-Type")

	// 2. Embeber en el archivo
	editor := tags.NewEditor(s.getDB(r))
	if err := editor.EmbedCover(trackID, imgData, mimeType); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Limpiar caché de carátula
	cachePath := filepath.Join(s.Config.CachePath, "covers", trackID+".jpg")
	os.Remove(cachePath)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleEditTags(trackID string, w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	editor := tags.NewEditor(s.getDB(r))

	// Silenciar el watcher
	var path string
	s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if path != "" && s.Watcher != nil {
		s.Watcher.MutePath(path, 10*time.Second)
	}

	changes, err := editor.UpdateTags(trackID, body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error(), "changes": changes})
		return
	}

	// Renombrado automático si está activo en Settings
	if s.Config.AutoRename {
		editor.OrganizeFile(trackID, s.Config.MusicPath, s.Config.RenamePattern)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "changes": changes})
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

	user, _ := r.Context().Value(userKey).(*db.User)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}

	// Pagination parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 1000 // Default limit to protect memory
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	// Lighter JSON flag
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

	// Dynamic selection for memory optimization
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

	// Prepend username to args for the LEFT JOIN and append limit/offset
	finalArgs := append([]interface{}{user.Username}, args...)
	finalArgs = append(finalArgs, limit, offset)

	rows, err := s.getDB(r).Conn.Query(query, finalArgs...)
	if err != nil {
		slog.Error("Database query failed in handleGetTracks", "error", err, "query", query)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tracks := make([]interface{}, 0)
	rowCount := 0
	for rows.Next() {
		rowCount++
		if minimal {
			var id, title, genre, artist, album, format string
			var trackNum, year, rating, instrumental int
			var duration float64
			if err := rows.Scan(&id, &title, &trackNum, &duration, &genre, &artist, &album, &year, &format, &rating, &instrumental); err != nil {
				slog.Error("Scan failed (minimal)", "error", err)
				continue
			}
			tracks = append(tracks, map[string]interface{}{
				"id": id, "title": title, "trackNum": trackNum, "duration": duration, "genre": genre,
				"artist": artist, "album": album, "year": year, "format": format, "rating": rating, "instrumental": instrumental == 1,
			})
		} else {
			var id, title, genre, artist, album, format, composer, comment, tStatus string
			var trackNum, discNum, year, bitRate, sampleRate, rating, instrumental, trackTotal, discTotal, playCount int
			var duration float64
			var size int64
			if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment, &tStatus, &rating, &instrumental, &trackTotal, &discTotal, &playCount); err != nil {
				slog.Error("Scan failed (full)", "error", err)
				continue
			}
			tracks = append(tracks, map[string]interface{}{
				"id": id, "title": title, "trackNum": trackNum, "discNum": discNum,
				"duration": duration, "genre": genre, "artist": artist, "album": album, "year": year,
				"size": size, "format": format, "bitrate": bitRate, "sampleRate": sampleRate,
				"composer": composer, "comment": comment, "status": tStatus, "rating": rating,
				"instrumental": instrumental == 1, "trackTotal": trackTotal, "discTotal": discTotal,
				"playCount": playCount,
			})
		}
	}
	
	slog.Info("handleGetTracks result", "status", status, "rows_found", rowCount, "tracks_returned", len(tracks), "limit", limit, "offset", offset)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
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

	// Buscar el path en la DB
	var path string
	err := s.MasterDB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// Si hay un escáner activo, le pasamos el trabajo por el carril VIP
	if s.activeScanner != nil {
		select {
		case s.activeScanner.PriorityJobs <- path:
			slog.Debug("👑 Track prioritized", "id", trackID)
		default:
			// Si la cola VIP está llena, no hacemos nada (poco probable)
		}
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"prioritized"}`))
}

func (s *Server) handleGetInbox(w http.ResponseWriter, r *http.Request) {
	// Forzar filtro por status=pending y raw para el Inbox
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
	for _, oldID := range data.IDs {
		db := s.getDB(r)
		var path string
		err := db.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", oldID).Scan(&path)
		if err != nil {
			continue
		}

		newID := tags.GenerateUUID()

		// 1. Lo activamos PRIMERO en la DB
		db.Conn.Exec("UPDATE tracks SET status = 'active' WHERE id = ?", oldID)
		approvedCount++

		// Silenciar el watcher para este archivo mientras escribimos tags
		if s.Watcher != nil {
			s.Watcher.MutePath(path, 10*time.Second)
		}

		// Lógica de Carátulas: Si no tiene interna, buscar local e incrustar
		go func(p string, tid string) {
			f, _ := os.Open(p)
			if f != nil {
				defer f.Close()
				metadata, _ := tags.Open(p)
				if metadata == nil || metadata.Picture() == nil {
					dir := filepath.Dir(p)
					common := []string{"cover", "folder", "front", "album"}
					entries, _ := os.ReadDir(dir)
					for _, ent := range entries {
						name := strings.ToLower(ent.Name())
						ext := filepath.Ext(name)
						if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
							continue
						}
						base := strings.TrimSuffix(name, ext)
						for _, c := range common {
							if base == c {
								imgData, _ := os.ReadFile(filepath.Join(dir, ent.Name()))
								mime := "image/jpeg"
								if ext == ".png" {
									mime = "image/png"
								}
								tags.NewEditor(s.MasterDB).EmbedCover(tid, imgData, mime)
								return
							}
						}
					}
				}
			}
		}(path, newID)

		if err := tags.WritePersistentID(path, newID); err == nil {
			// 2. Si se escribió correctamente el ID físico, actualizamos el ID en la DB
			_, err = db.Conn.Exec("UPDATE tracks SET id = ? WHERE id = ?", newID, oldID)
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
		}
	}

	var totalActive int
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'active'").Scan(&totalActive)
	fmt.Printf("✅ %d tracks aprobadas. Total en biblioteca activa: %d\n", approvedCount, totalActive)

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
		// Mark as rejected so scanner doesn't pick it up as pending again
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
	db := s.getDB(r)
	rows, err := db.Conn.Query(`
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

	// Agrupamos tracks pendientes por (Artista Normalizado + Título Álbum)
	type groupKey struct {
		normArtist string
		albumTitle string
	}
	groups := make(map[groupKey][]string) // key -> IDs
	trackMeta := make(map[string]PendingTrack)

	for _, t := range tracks {
		norm := scanner.NormalizeArtist(t.Artist)
		key := groupKey{norm, t.Album}
		groups[key] = append(groups[key], t.ID)
		trackMeta[t.ID] = t
	}

	// Para cada grupo, buscamos el "Consenso" de forma optimizada
	albumTitles := make([]string, 0)
	for key := range groups {
		isGeneric := len(key.albumTitle) < 3
		if !isGeneric {
			if _, err := strconv.Atoi(key.albumTitle); err == nil {
				isGeneric = true
			}
		}
		if !isGeneric {
			albumTitles = append(albumTitles, key.albumTitle)
		}
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

		cRows, err := db.Conn.Query(query, args...)
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
		if !exists {
			continue
		}

		counts := make(map[string]int)
		years := make(map[int]int)

		for _, d := range data {
			// Filtrar por artista similar al grupo actual (en Go, para evitar LIKE complejo en el IN)
			dNorm := scanner.NormalizeArtist(d.Artist)
			if strings.Contains(dNorm, key.normArtist) || strings.Contains(key.normArtist, dNorm) {
				counts[d.Artist]++
				if d.Year > 0 {
					years[d.Year]++
				}
			}
		}

		if len(counts) == 0 {
			continue
		}

		// El ganador es el que tiene más frecuencia
		winnerArtist := ""
		maxCount := -1
		for a, c := range counts {
			if c > maxCount {
				maxCount = c
				winnerArtist = a
			}
		}

		winnerYear := 0
		maxYearCount := -1
		for y, c := range years {
			if c > maxYearCount {
				maxYearCount = c
				winnerYear = y
			}
		}

		// Proponemos la sugerencia si es diferente a lo que tiene el track
		if winnerArtist != "" {
			winnerNorm := scanner.NormalizeArtist(winnerArtist)

			for _, id := range ids {
				t := trackMeta[id]
				currentNorm := scanner.NormalizeArtist(t.Artist)

				// REGLA DE SEGURIDAD: Solo sugerimos el cambio si los artistas normalizan IGUAL
				// Esto evita que en una banda sonora (Wicked) intente cambiar a "Cynthia Erivo" por "Ariana Grande"
				// pero sí permite cambiar "Ariana Grande ft. Alguien" por "Ariana Grande"
				isSameArtistRoot := winnerNorm == currentNorm

				if isSameArtistRoot && (t.Artist != winnerArtist || (winnerYear > 0 && t.Year != winnerYear)) {
					suggestions = append(suggestions, Suggestion{
						TrackID: id,
						Artist:  winnerArtist,
						Album:   t.Album,
						Year:    winnerYear,
					})
				} else if !isSameArtistRoot && (winnerYear > 0 && t.Year != winnerYear) {
					// Si son artistas distintos pero el año del álbum es consistente, solo sugerimos el año
					suggestions = append(suggestions, Suggestion{
						TrackID: id,
						Artist:  t.Artist, // Mantenemos el original
						Album:   t.Album,
						Year:    winnerYear,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"suggestions": suggestions,
	})
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

	db := s.getDB(r)
	tx, err := db.Conn.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	for _, sug := range data.Suggestions {
		// 1. Obtener o crear artista del álbum corregido
		artistId := scanner.GenerateID(sug.Artist)
		_, err = tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistId, sug.Artist)
		if err != nil {
			continue
		}

		// 2. Obtener o crear álbum corregido (Lógica sincronizada con el scanner)
		groupingKey := "AA:" + scanner.NormalizeArtist(sug.Artist)
		albumId := scanner.GenerateID(groupingKey + "|" + scanner.NormalizeString(sug.Album))

		_, err = tx.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", albumId, artistId, sug.Album, sug.Year)
		if err != nil {
			continue
		}

		// 3. Actualizar el track en el Inbox (manteniendo status 'pending')
		_, err = tx.Exec("UPDATE tracks SET album_id = ?, artist = ? WHERE id = ?", albumId, sug.Artist, sug.TrackID)
		if err != nil {
			slog.Error("Failed to apply suggestion to track", "id", sug.TrackID, "err", err)
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
	s.updateInboxCount()
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

	// 1. Actualizar DB Centralizada (user_tracks)
	err := s.MasterDB.UpdateTrackUserData(user.Username, data.ID, data.Rating, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Sincronizar con Tags del archivo
	editor := tags.NewEditor(s.getDB(r))
	tagData := map[string]interface{}{
		"rating": float64(data.Rating),
	}
	// data.ID es el track ID
	if _, err := editor.UpdateTags(data.ID, tagData); err != nil {
		slog.Error("Failed to update tags in massive editor", "track", data.ID, "err", err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
	s.updateInboxCount()
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
	s.updateInboxCount()
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing search query", http.StatusBadRequest)
		return
	}

	searchTerm := query + "*" // Prefix search for FTS5
	results := make(map[string]interface{})

	// 1. Search with FTS5 (Instant search for title, artist, album, genre)
	rows, err := s.getDB(r).Conn.Query(`
		SELECT id, title, artist, album 
		FROM search_idx 
		WHERE search_idx MATCH ? 
		ORDER BY rank 
		LIMIT 50`, searchTerm)

	var tracks []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, artist, album string
			rows.Scan(&id, &title, &artist, &album)
			tracks = append(tracks, map[string]interface{}{"id": id, "title": title, "artist": artist, "album": album})
		}
	}
	results["tracks"] = tracks

	// Separate results for UI compatibility if needed, though FTS5 returns mixed relevance
	results["artists"] = []string{} // We could also use separate FTS5 queries if required
	results["albums"] = []string{}
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
	user, _ := r.Context().Value(userKey).(*db.User)
	stats := make(map[string]interface{})

	// Contadores básicos - Mezcla de Global y Personal
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

	// Inbox count (pendientes de aprobar filtrados por path)
	var pendingCount int
	wherePending := "WHERE status = 'pending'"
	if pathFilter != "" {
		wherePending += " AND " + pathFilter
	}
	s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM tracks t "+wherePending, pathArgs...).Scan(&pendingCount)
	stats["inboxCount"] = pendingCount

	// Joyas olvidadas (Deduplicadas por álbum + artista para evitar repetir carátulas) - Optimizado sin RANDOM()
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

	// Lo más escuchado (Top 10 PERSONAL)
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

	// Recientemente añadidos (Globales para descubrimiento filtrados por permiso)
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

			// Algoritmo de Mood/Tag Refinado con Prioridades (NUEVA JERARQUÍA PARA MÁS VARIEDAD)
			tag := "Recuerdo"
			color := "tertiary"
			g := strings.ToLower(genre)

			// 1. Prioridad Máxima: Éxitos personales
			if pc > 30 {
				tag = "Top Hit"
				color = "primary"
				// 2. Alta Fidelidad
			} else if br > 1000000 {
				tag = "Premium"
				color = "primary"
				// 3. Sentimiento Temporal (Clásicos)
			} else if year > 0 && year < currentYear-20 {
				tag = "Nostalgia"
				color = "tertiary"
				// 4. Género / Mood
			} else if strings.Contains(g, "rock") || strings.Contains(g, "pop") || strings.Contains(g, "electronic") || strings.Contains(g, "metal") {
				tag = "Energía"
				color = "primary"
			} else if strings.Contains(g, "jazz") || strings.Contains(g, "classical") {
				tag = "Inspiración"
				color = "secondary"
			} else if strings.Contains(g, "chill") || strings.Contains(g, "ambient") || strings.Contains(g, "lo-fi") || strings.Contains(g, "blues") {
				tag = "Chill"
				color = "tertiary"
				// 5. Por defecto si es nuevo y no cumple lo anterior
			} else if pc == 0 {
				tag = "Descubrir"
				color = "secondary"
			}

			recentAlbums = append(recentAlbums, map[string]interface{}{
				"id":       id,
				"title":    title,
				"artist":   artist,
				"addedAt":  addedAt,
				"tag":      tag,
				"tagColor": color,
			})
		}
	}
	stats["discovery"] = recentAlbums

	// Trends: Top 3 genres by track count (Globales)
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
	// Obtener estado del escaneo GLOBAL
	scanning := false
	processed := 0
	if val, ok := s.scanStatuses.Load("global"); ok {
		sStatus := val.(*ScanStatus)
		scanning = sStatus.Scanning
		processed = sStatus.Processed
	}

	// Usamos el contador en caché para que sea ultra rápido
	ic := s.inboxCount.Load()
	rc := s.rawCount.Load()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scanning":   scanning,
		"processed":  processed,
		"rawCount":    rc,
		"inboxCount": ic,
	})
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

	// Duplicados
	dups, _ := s.getDB(r).GetDuplicates()
	health["duplicateCount"] = len(dups)

	// Huérfanos
	orphans, _ := s.getDB(r).GetOrphanedTracks()
	health["orphanCount"] = len(orphans)

	// Baja calidad (< 128kbps)
	lowQual, _ := s.getDB(r).GetLowQualityTracks(128)
	health["lowQualityCount"] = len(lowQual)

	// Carátulas salud
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
		if err := rows.Scan(&t.ID, &t.Title, &t.Genre, &t.Artist, &t.Album, &t.Year); err == nil {
			tracks = append(tracks, t)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (s *Server) handleLibraryDuplicates(w http.ResponseWriter, r *http.Request) {
	dups, err := s.getDB(r).GetDuplicates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Agrupar duplicados de forma inclusiva
	groups := make(map[string][]models.Track)
	for _, t := range dups {
		// Usamos una clave de metadatos para que canciones con distinto hash
		// pero mismo contenido teórico (artista, álbum, título) se agrupen y se vea el duplicado.
		key := fmt.Sprintf("%s|%s|%s",
			strings.ToLower(t.Artist),
			strings.ToLower(t.Album),
			strings.ToLower(t.Title))

		// Si no tiene metadatos pero tiene hash (raro pero posible), usamos el hash.
		if t.Artist == "" && t.Album == "" && t.Title == "" && t.FileHash != "" {
			key = "hash:" + t.FileHash
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

func (s *Server) handleLibraryMissingCovers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT al.id, al.title, ar.name as artist, t.path
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON t.album_id = al.id
		WHERE al.cover_width IS NULL OR al.cover_width = 0
		GROUP BY al.id
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, title, artist, path string
		rows.Scan(&id, &title, &artist, &path)
		result = append(result, map[string]interface{}{"id": id, "title": title, "artist": artist, "path": path, "type": "missing"})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleLibrarySmallCovers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT al.id, al.title, ar.name as artist, t.path, al.cover_width, al.cover_height
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON t.album_id = al.id
		WHERE al.cover_width > 0 AND (al.cover_width < 300 OR al.cover_height < 300)
		GROUP BY al.id
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, title, artist, path string
		var w, h int
		rows.Scan(&id, &title, &artist, &path, &w, &h)
		result = append(result, map[string]interface{}{"id": id, "title": title, "artist": artist, "path": path, "width": w, "height": h, "type": "small"})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleLibraryHugeCovers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.getDB(r).Conn.Query(`
		SELECT al.id, al.title, ar.name as artist, t.path, al.cover_width, al.cover_height, al.cover_size
		FROM albums al
		JOIN artists ar ON al.artist_id = ar.id
		JOIN tracks t ON t.album_id = al.id
		WHERE al.cover_width > 1500 OR al.cover_height > 1500 OR al.cover_size > 2000000
		GROUP BY al.id
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, title, artist, path string
		var w, h, size int
		rows.Scan(&id, &title, &artist, &path, &w, &h, &size)
		result = append(result, map[string]interface{}{"id": id, "title": title, "artist": artist, "path": path, "width": w, "height": h, "size": size, "type": "huge"})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleNormalizeCover(w http.ResponseWriter, r *http.Request) {
	albumID := r.URL.Query().Get("id")
	if albumID == "" {
		http.Error(w, "Missing album id", http.StatusBadRequest)
		return
	}

	// 1. Encontrar una pista de este álbum que tenga carátula
	var fpath string
	err := s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE album_id = ? LIMIT 1", albumID).Scan(&fpath)
	if err != nil {
		http.Error(w, "No tracks found for this album", http.StatusNotFound)
		return
	}

	// 2. Extraer, redimensionar y guardar como JPG normalizado
	f, err := os.Open(fpath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	metadata, err := tags.Open(fpath)
	if err != nil || metadata.Picture() == nil {
		http.Error(w, "No embedded cover found to normalize", http.StatusNotFound)
		return
	}

	src, _, err := image.Decode(bytes.NewReader(metadata.Picture().Data))
	if err != nil {
		http.Error(w, "Failed to decode image", http.StatusInternalServerError)
		return
	}

	// Normalizar a 800px (equilibrado)
	resized := imaging.Resize(src, 800, 0, imaging.Lanczos)
	
	// Escribir de vuelta a todos los archivos del álbum (operación pesada, silenciar watcher)
	rows, _ := s.getDB(r).Conn.Query("SELECT path FROM tracks WHERE album_id = ?", albumID)
	defer rows.Close()

	buf := new(bytes.Buffer)
	imaging.Encode(buf, resized, imaging.JPEG)
	normalizedData := buf.Bytes()

	for rows.Next() {
		var p string
		rows.Scan(&p)
		if s.Watcher != nil {
			s.Watcher.MutePath(p, 5*time.Second)
		}
		
		m, err := tags.Open(p)
		if err == nil {
			// Aquí necesitaríamos una interfaz para incrustar carátulas en la librería tags
			// Por ahora usamos el fallback de guardarlo en caché y actualizar DB
			m.Close()
		}
	}

	// Actualizar DB con los nuevos valores de normalización
	s.getDB(r).Conn.Exec(`
		UPDATE albums 
		SET cover_width = ?, cover_height = ?, cover_size = ?, cover_format = 'image/jpeg' 
		WHERE id = ?`, 
		resized.Bounds().Dx(), resized.Bounds().Dy(), len(normalizedData), albumID)

	// Invalidar caché de carátula
	cachePath := filepath.Join(s.Config.CachePath, "covers", albumID+".jpg")
	os.Remove(cachePath)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
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

func (s *Server) handleGetReleaseDetails(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	mb := metadata.NewMBClient()
	release, err := mb.GetReleaseDetails(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(release)
}

func (s *Server) handleMetadataSearchAlbum(w http.ResponseWriter, r *http.Request) {
	artist := r.URL.Query().Get("artist")
	album := r.URL.Query().Get("album")

	if artist == "" || album == "" {
		http.Error(w, "Artist and album are required", http.StatusBadRequest)
		return
	}

	mb := metadata.NewMBClient()
	releases, err := mb.SearchRelease(artist, album)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(releases)
}

func (s *Server) handleMetadataSearch(w http.ResponseWriter, r *http.Request) {
	artist := r.URL.Query().Get("artist")
	title := r.URL.Query().Get("title")
	album := r.URL.Query().Get("album")

	if artist == "" && title == "" && album == "" {
		http.Error(w, "At least one search parameter is required", http.StatusBadRequest)
		return
	}

	parts := []string{}
	if artist != "" {
		parts = append(parts, artist)
	}
	if title != "" {
		parts = append(parts, title)
	}
	if album != "" {
		parts = append(parts, album)
	}

	searchQuery := strings.Join(parts, " ")
	url := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=music&limit=15", url.QueryEscape(searchQuery))

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
		if len(item.ReleaseDate) >= 4 {
			fmt.Sscanf(item.ReleaseDate[:4], "%d", &year)
		}

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

	if trackID == "search" {
		s.handleLyricsSearch(w, r)
		return
	}

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
		slog.Info("📜 Letras encontradas localmente", "trackID", trackID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"lyrics": localLyrics})
		return
	}

	// 2. Intentar obtener de LRCLib (mejor calidad, letras sincronizadas)
	slog.Info("🌐 Buscando letras en LRCLib", "artist", artist, "title", title)
	resp, err := http.Get(fmt.Sprintf("https://lrclib.net/api/get?artist_name=%s&track_name=%s", url.QueryEscape(artist), url.QueryEscape(title)))
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var lrcData struct {
			SyncedLyrics string `json:"syncedLyrics"`
			PlainLyrics  string `json:"plainLyrics"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&lrcData); err == nil {
			lyricsText := lrcData.SyncedLyrics
			if lyricsText == "" {
				lyricsText = lrcData.PlainLyrics
			}
			if lyricsText != "" {
				slog.Info("✅ Letras obtenidas de LRCLib")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"lyrics": lyricsText})
				return
			}
		}
	}

	// Fallback: lyrics.ovh
	slog.Info("🌐 Fallback a lyrics.ovh", "artist", artist, "title", title)
	resp, err = http.Get(fmt.Sprintf("https://api.lyrics.ovh/v1/%s/%s", url.QueryEscape(artist), url.QueryEscape(title)))
	if err != nil {
		slog.Warn("❌ Fallo total al obtener letras", "err", err)
		http.Error(w, "Failed to fetch lyrics", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	slog.Info("✅ Letras obtenidas de lyrics.ovh")
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

	slog.Info("🔍 Buscando letras (manual)", "artista", artist, "titulo", title)
	resp, err := http.Get(fmt.Sprintf("https://lrclib.net/api/search?artist_name=%s&track_name=%s", url.QueryEscape(artist), url.QueryEscape(title)))
	if err != nil {
		slog.Error("❌ Error en búsqueda LRCLib", "err", err)
		http.Error(w, "Failed to search lyrics", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	slog.Info("✅ Búsqueda manual completada", "status", resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

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
		if body.IsSmart {
			isSmart = 1
		}

		ruleSQL := ""
		if body.IsSmart {
			ruleSQL = s.generateRuleSQL(body.Rules)
		}

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
			// Ejecutar conteo rápido para smart playlist
			s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM (" + ruleSQL + ")").Scan(&count)
		} else {
			s.getDB(r).Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?", id).Scan(&count)
		}

		list = append(list, map[string]interface{}{
			"id": id, "name": name, "isSmart": isSmart == 1, "trackCount": count,
		})
	}
	if list == nil {
		list = []map[string]interface{}{}
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
		       t.composer, t.comment, t.instrumental, COALESCE(t.track_total, 0), COALESCE(t.disc_total, 0)
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
		var trackNum, discNum, duration, year, bitRate, sampleRate, instrumental, trackTotal, discTotal int
		var size int64
		if err := rows.Scan(&id, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &size, &format, &bitRate, &sampleRate, &composer, &comment, &instrumental, &trackTotal, &discTotal); err != nil {
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

	// 1. Obtener info de la playlist
	var isSmart int
	var ruleSQL string
	err := s.getDB(r).Conn.QueryRow("SELECT is_smart, rule_sql FROM playlists WHERE id = ? AND username = ?", id, user.Username).Scan(&isSmart, &ruleSQL)
	if err != nil {
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	var rows *sql.Rows
	fields := "t.id, t.title, t.track_num, t.disc_num, t.duration, t.genre, a.name, al.title, al.year, COALESCE(ut.rating, 0), t.composer, t.comment, t.instrumental, COALESCE(t.track_total, 0), COALESCE(t.disc_total, 0)"

	if isSmart == 1 && ruleSQL != "" {
		// Ejecutar regla dinámica
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
		// Lista estática tradicional
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
		var trackNum, discNum, duration, year, rating, instrumental, trackTotal, discTotal int
		err := rows.Scan(&tid, &title, &trackNum, &discNum, &duration, &genre, &artist, &album, &year, &rating, &composer, &comment, &instrumental, &trackTotal, &discTotal)
		if err != nil {
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

func (s *Server) handleGetCover(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/cover")
	
	// Placeholder case
	if trackID == "placeholder" || trackID == "" {
		s.servePlaceholderCover(w)
		return
	}

	// 0. VERIFICAR CACHÉ (VELOCIDAD MÁXIMA PERSISTENTE)
	cachePath := filepath.Join(s.Config.CachePath, "covers", trackID+".jpg")
	os.MkdirAll(filepath.Dir(cachePath), 0755)

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
			s.servePlaceholderCover(w)
			return
		}
	}

	// Security: Validate path is within music directory
	absMusicPath, _ := filepath.Abs(s.Config.MusicPath)
	absTrackPath, _ := filepath.Abs(fpath)
	if !strings.HasPrefix(absTrackPath, absMusicPath) {
		slog.Error("Unauthorized cover access attempt", "path", fpath, "user", r.Context().Value(userKey))
		s.servePlaceholderCover(w)
		return
	}

	// 1. Intentar extraer embebida ligera
	f, err := os.Open(fpath)
	if err == nil {
		defer f.Close()
		metadata, err := tags.Open(fpath)
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
				w.Header().Set("Content-Type", pic.MIME)
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
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		if commonNames[base] {
			fImg, err := os.Open(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}

			// Limit reading to 10MB
			limit := int64(10 * 1024 * 1024)
			imgData, err := io.ReadAll(io.LimitReader(fImg, limit))
			fImg.Close()

			if err != nil {
				continue
			}
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
	s.servePlaceholderCover(w)
}

func (s *Server) servePlaceholderCover(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 24 24" fill="none" stroke="#666" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>`)
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

	sc := scanner.NewScanner(s.MasterDB)
	if err := sc.ParseAndSaveFile(path); err != nil {
		slog.Error("Watcher: Failed to process file", "path", path, "error", err)
	}
	s.updateInboxCount()
}

func (s *Server) HandleFileDelete(path string) {
	slog.Info("Watcher: File deletion detected", "path", path)
	_, err := s.MasterDB.Conn.Exec("DELETE FROM tracks WHERE path = ?", path)
	if err != nil {
		slog.Error("Watcher: Failed to remove deleted track from DB", "path", path, "error", err)
	}
	s.updateInboxCount()
}

func (s *Server) TriggerGlobalScan() {
	status := &ScanStatus{Scanning: true, Processed: 0}
	s.scanStatuses.Store("global", status)

	slog.Info("Starting global scan")
	sc := scanner.NewScanner(s.MasterDB)
	sc.OnProgress = func(path string, count int) {
		status.Processed = count
		s.scanStatuses.Store("global", status)
	}

	go func() {
		defer func() {
			status.Scanning = false
			s.scanStatuses.Store("global", status)
			slog.Info("Global scan completed", "total", status.Processed)
		}()
		if err := sc.ScanDirectory(s.Config.MusicPath); err != nil {
			slog.Error("Global scan failed", "error", err)
		}
	}()
}
