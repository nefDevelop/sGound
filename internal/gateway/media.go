package gateway

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var audioContentTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".ogg":  "audio/ogg",
	".opus": "audio/ogg",
	".wav":  "audio/wav",
}

// trackPath devuelve la ruta del fichero de un track activo.
func (g *SubsonicGateway) trackPath(id string) (string, error) {
	var path string
	err := g.DB.Conn.QueryRow(
		"SELECT path FROM tracks WHERE id = ? AND status = 'active'", id).Scan(&path)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", sql.ErrNoRows
	}
	return path, nil
}

// jailed comprueba que la ruta quede dentro de la biblioteca.
func (g *SubsonicGateway) jailed(path string) bool {
	if g.MusicPath == "" {
		return true
	}
	absRoot, err := filepath.Abs(g.MusicPath)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func (g *SubsonicGateway) serveTrack(w http.ResponseWriter, r *http.Request, download bool) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	path, err := g.trackPath(id)
	if err == sql.ErrNoRows {
		writeErr(w, r, errNotFound, "Canción no encontrada.")
		return
	}
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	if !g.jailed(path) {
		writeErr(w, r, errNotAuthorized, "Fuera de la biblioteca.")
		return
	}
	// maxBitRate/format se ignoran de momento: se sirve el original.
	if ct, ok := audioContentTypes[strings.ToLower(filepath.Ext(path))]; ok {
		w.Header().Set("Content-Type", ct)
	}
	if download {
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	}
	http.ServeFile(w, r, path)
}

func (g *SubsonicGateway) handleStream(w http.ResponseWriter, r *http.Request) {
	g.serveTrack(w, r, false)
}

func (g *SubsonicGateway) handleDownload(w http.ResponseWriter, r *http.Request) {
	g.serveTrack(w, r, true)
}

// coverFile resuelve el jpg cacheado para un id de track, álbum o artista.
func (g *SubsonicGateway) coverFile(id string) string {
	candidates := []string{id}
	// Álbum -> primer track; artista -> primer track de su primer álbum.
	var trackID string
	if err := g.DB.Conn.QueryRow(
		"SELECT id FROM tracks WHERE album_id = ? AND status = 'active' ORDER BY disc_num, track_num LIMIT 1", id).Scan(&trackID); err == nil && trackID != "" {
		candidates = append([]string{trackID}, candidates...)
	} else {
		var albumID string
		if err := g.DB.Conn.QueryRow(
			"SELECT al.id FROM albums al JOIN artists ar ON ar.id = al.artist_id WHERE ar.id = ? ORDER BY al.title LIMIT 1", id).Scan(&albumID); err == nil && albumID != "" {
			if err := g.DB.Conn.QueryRow(
				"SELECT id FROM tracks WHERE album_id = ? AND status = 'active' ORDER BY disc_num, track_num LIMIT 1", albumID).Scan(&trackID); err == nil && trackID != "" {
				candidates = append([]string{trackID}, candidates...)
			}
		}
	}
	for _, c := range candidates {
		p := filepath.Join(g.CachePath, "covers", c+".jpg")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func (g *SubsonicGateway) handleGetCoverArt(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	f := g.coverFile(id)
	if f == "" {
		writeErr(w, r, errNotFound, "Portada no encontrada.")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, f)
}
