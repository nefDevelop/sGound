package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	"sgound/internal/auth"
	"sgound/internal/config"
	"sgound/internal/db"
	"sgound/internal/metadata"
	"sgound/internal/scanner"
	"sgound/internal/watcher"
	"sgound/web"
)

type ScanStatus struct {
	Scanning  bool `json:"scanning"`
	Processed int  `json:"processed"`
	RawCount  int  `json:"rawCount"`
}

var (
	imageProcessingSem = make(chan struct{}, 16) // Aumentado para mayor concurrencia en grids de imágenes
)

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
	sf               singleflight.Group
}

type OrganizeStatus struct {
	IsRunning bool     `json:"isRunning"`
	Total     int      `json:"total"`
	Processed int      `json:"processed"`
	Current   string   `json:"current"`
	Errors    []string `json:"errors"`
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
	slog.Info("📥 Inbox counts updated", "pending", pending, "raw", raw, "total", pending+raw)
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
	mux.HandleFunc("/api/tracks/{id}/cover", auth(s.handleUploadCover))
	mux.HandleFunc("/api/scan", auth(s.handleScan))
	mux.HandleFunc("/api/config", auth(s.handleConfig))
	mux.HandleFunc("/api/search", auth(s.handleSearch))
	mux.HandleFunc("/api/dashboard", auth(s.handleDashboard))
	mux.HandleFunc("/api/library/health", auth(s.handleLibraryHealth))
	mux.HandleFunc("/api/library/duplicates", auth(s.handleLibraryDuplicates))
	mux.HandleFunc("/api/library/orphans", auth(s.handleLibraryOrphans))
	mux.HandleFunc("/api/library/low-quality", auth(s.handleLibraryLowQuality))
	mux.HandleFunc("/api/library/missing-tags", auth(s.handleLibraryMissingTags))
	mux.HandleFunc("/api/library/covers", auth(s.handleLibraryAllCovers))
	mux.HandleFunc("/api/library/rescan-cover", auth(s.handleRescanCover))
	mux.HandleFunc("/api/library/normalize-cover", auth(s.handleNormalizeCover))
	mux.HandleFunc("/api/library/cleanup", auth(s.handleLibraryCleanup))
	mux.HandleFunc("/api/library/organize", auth(s.handleOrganizeLibrary))
	mux.HandleFunc("/api/library/organize/preview", auth(s.handleOrganizeLibraryPreview))
	mux.HandleFunc("/api/library/organize/status", auth(s.handleOrganizeStatus))
	mux.HandleFunc("/api/library/resync-index", auth(s.handleLibraryResyncIndex))
	mux.HandleFunc("/api/library/clean-playlists", auth(s.handleLibraryCleanPlaylists))
	mux.HandleFunc("/api/library/status", auth(s.handleScanStatus))
	mux.HandleFunc("/api/library/inbox", auth(s.handleGetInbox))
	mux.HandleFunc("/api/library/inbox/prioritize", auth(s.handlePrioritizeTrack))
	mux.HandleFunc("/api/library/inbox/analyze", auth(s.handleAnalyzeInbox))
	mux.HandleFunc("/api/library/inbox/apply-suggestions", auth(s.handleApplyInboxSuggestions))
	mux.HandleFunc("/api/tracks/approve", auth(s.handleApproveTracks))
	mux.HandleFunc("/api/tracks/approve-all", auth(s.handleApproveAllTracks))
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

func (s *Server) getDB(_ *http.Request) *db.MasterDatabase {
	return s.MasterDB
}

func (s *Server) executeTransaction(fn func(tx *sql.Tx) error) error {
	var err error
	for i := 0; i < 5; i++ {
		tx, errTx := s.MasterDB.Conn.Begin()
		if errTx != nil {
			if strings.Contains(errTx.Error(), "database is locked") {
				time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
				continue
			}
			return errTx
		}

		err = fn(tx)
		if err == nil {
			return tx.Commit()
		}

		tx.Rollback()
		if strings.Contains(err.Error(), "database is locked") {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("transaction failed after retries: %w", err)
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
	id = strings.TrimSuffix(id, "/image")
	id = strings.TrimSuffix(id, "/cover-url")
	id = strings.TrimSuffix(id, "/cover")

	return id
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
