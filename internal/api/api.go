package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"io/fs"

	"sgound/internal/db"
	"sgound/internal/gateway"
	"sgound/internal/tags"
	"sgound/web"
)

type Server struct {
	DB *db.Database
}

func NewServer(database *db.Database) *Server {
	return &Server{DB: database}
}

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// 1. REST Core API
	mux.HandleFunc("/api/artists", s.handleGetArtists)
	mux.HandleFunc("/api/stream/", s.handleStream) // /api/stream/{id}
	mux.HandleFunc("/api/tracks/", s.handleTrackActions)

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

func (s *Server) handleTrackActions(w http.ResponseWriter, r *http.Request) {
	// Simple router for /api/tracks/{id}/tags
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) == 4 && pathParts[3] == "tags" && r.Method == http.MethodPut {
		s.handleEditTags(pathParts[2], w, r)
		return
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (s *Server) handleEditTags(trackID string, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
		Album  string `json:"album"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	editor := tags.NewEditor(s.DB)
	if err := editor.UpdateTags(trackID, body.Title, body.Artist, body.Album); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleGetArtists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := s.DB.Conn.Query("SELECT id, name FROM artists")
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

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	trackID := r.URL.Path[len("/api/stream/"):]
	if trackID == "" {
		http.Error(w, "Missing track ID", http.StatusBadRequest)
		return
	}

	var path string
	err := s.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "File read error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "File stats error", http.StatusInternalServerError)
		return
	}

	// http.ServeContent handles HTTP Range requests for streaming automatically
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), f)
}
