package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"sgound/internal/metadata"
	"sgound/internal/tags"
)

func (s *Server) handleTrackActions(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/tracks/")

	if strings.HasSuffix(r.URL.Path, "/tags") && r.Method == http.MethodPut {
		s.handleEditTags(trackID, w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/organize") && r.Method == http.MethodPost {
		s.handleManualOrganize(trackID, w, r)
		return
	}
	if r.Method == http.MethodDelete {
		s.handleDeleteTrack(trackID, w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleEditTags(trackID string, w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	editor := tags.NewEditor(s.getDB(r))

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

func (s *Server) handleDeleteTrack(trackID string, w http.ResponseWriter, r *http.Request) {
	dbase := s.getDB(r)
	var path string
	err := dbase.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Error fetching track path", http.StatusInternalServerError)
		}
		return
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to delete physical file", "path", path, "error", err)
	}

	_, err = dbase.Conn.Exec("DELETE FROM tracks WHERE id = ?", trackID)
	if err != nil {
		http.Error(w, "Failed to delete track from DB", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
	s.updateInboxCount()
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
	localLyrics, _ := editor.ReadLyrics(trackID)
	if localLyrics != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"lyrics": localLyrics})
		return
	}

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
				// Auto-save to file
				go func() {
					if err := editor.UpdateLyrics(trackID, lyricsText); err != nil {
						slog.Error("Failed to auto-save lyrics", "trackID", trackID, "error", err)
					} else {
						slog.Info("Lyrics auto-saved to file", "trackID", trackID)
					}
				}()

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"lyrics": lyricsText})
				return
			}
		}
	}

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
	if artist != "" { parts = append(parts, artist) }
	if title != "" { parts = append(parts, title) }
	if album != "" { parts = append(parts, album) }

	searchQuery := strings.Join(parts, " ")
	u := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=music&limit=15", url.QueryEscape(searchQuery))

	resp, err := http.Get(u)
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

func (s *Server) handleSetCoverURL(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/tracks/")
	applyToAlbum := r.URL.Query().Get("album") == "true"
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

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
	s.embedCoverToTracks(r, trackID, applyToAlbum, imgData, mimeType, w)
}

func (s *Server) handleUploadCover(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/tracks/")
	applyToAlbum := r.URL.Query().Get("album") == "true"

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("cover")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	imgData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	s.embedCoverToTracks(r, trackID, applyToAlbum, imgData, mimeType, w)
}

func (s *Server) embedCoverToTracks(r *http.Request, trackID string, applyToAlbum bool, imgData []byte, mimeType string, w http.ResponseWriter) {
	editor := tags.NewEditor(s.getDB(r))
	
	var trackIDs []string
	if applyToAlbum {
		rows, err := s.getDB(r).Conn.Query("SELECT id FROM tracks WHERE album_id = (SELECT album_id FROM tracks WHERE id = ?)", trackID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil { trackIDs = append(trackIDs, id) }
			}
		}
	} else {
		trackIDs = []string{trackID}
	}

	for _, id := range trackIDs {
		if err := editor.EmbedCover(id, imgData, mimeType); err != nil {
			slog.Error("Failed to embed cover", "track", id, "error", err)
		}
		cachePath := filepath.Join(s.Config.CachePath, "covers", id+".jpg")
		os.Remove(cachePath)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleNormalizeCover(w http.ResponseWriter, r *http.Request) {
	albumID := r.URL.Query().Get("id")
	if albumID == "" {
		http.Error(w, "Missing album id", http.StatusBadRequest)
		return
	}

	var fpath string
	err := s.getDB(r).Conn.QueryRow("SELECT path FROM tracks WHERE album_id = ? LIMIT 1", albumID).Scan(&fpath)
	if err != nil {
		http.Error(w, "No tracks found for this album", http.StatusNotFound)
		return
	}

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

	resized := imaging.Resize(src, 800, 0, imaging.Lanczos)
	
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
	}

	s.getDB(r).Conn.Exec(`
		UPDATE albums 
		SET cover_width = ?, cover_height = ?, cover_size = ?, cover_format = 'image/jpeg' 
		WHERE id = ?`, 
		resized.Bounds().Dx(), resized.Bounds().Dy(), len(normalizedData), albumID)

	cachePath := filepath.Join(s.Config.CachePath, "covers", albumID+".jpg")
	os.Remove(cachePath)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
