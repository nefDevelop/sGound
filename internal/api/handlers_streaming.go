package api

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"sgound/internal/db"
	"sgound/internal/tags"
)

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

	absMusicPath, _ := filepath.Abs(s.Config.MusicPath)
	absTrackPath, _ := filepath.Abs(fpath)
	if !strings.HasPrefix(absTrackPath, absMusicPath) {
		slog.Error("Unauthorized file access attempt", "path", fpath, "user", user.Username)
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	go func() {
		s.MasterDB.UpdateTrackUserData(user.Username, trackID, -1, true)
	}()

	bitrate := r.URL.Query().Get("bitrate")
	useFFmpeg := false
	ffmpegArgs := []string{"-i", fpath}

	if s.Config.ReplayGainEnabled && rg != 0 {
		useFFmpeg = true
		ffmpegArgs = append(ffmpegArgs, "-af", fmt.Sprintf("volume=%.2fdB", rg))
	}

	if bitrate != "" && bitrate != "original" {
		useFFmpeg = true
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
		ffmpegArgs = append(ffmpegArgs, "-f", "mp3", "-map_metadata", "-1", "pipe:1")

		cmd := exec.CommandContext(r.Context(), "ffmpeg", ffmpegArgs...)
		cmd.Stdout = w
		if err := cmd.Start(); err != nil {
			slog.Error("FFmpeg streaming failed", "error", err, "args", ffmpegArgs)
			http.Error(w, "Streaming failed", http.StatusInternalServerError)
			return
		}

		err := cmd.Wait()
		if err != nil && r.Context().Err() == nil {
			slog.Error("FFmpeg process finished with error", "error", err)
		}
		return
	}

	http.ServeFile(w, r, fpath)
}

func (s *Server) handleGetCover(w http.ResponseWriter, r *http.Request) {
	trackID := s.extractID(r, "/api/cover")
	if trackID == "placeholder" || trackID == "" {
		s.servePlaceholderCover(w)
		return
	}

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

	absMusicPath, _ := filepath.Abs(s.Config.MusicPath)
	absTrackPath, _ := filepath.Abs(fpath)
	if !strings.HasPrefix(absTrackPath, absMusicPath) {
		s.servePlaceholderCover(w)
		return
	}

	imageProcessingSem <- struct{}{}
	defer func() { <-imageProcessingSem }()

	f, err := os.Open(fpath)
	if err == nil {
		defer f.Close()
		metadata, err := tags.Open(fpath)
		if err == nil {
			if pic := metadata.Picture(); pic != nil {
				src, _, err := image.Decode(bytes.NewReader(pic.Data))
				if err == nil {
					resized := imaging.Resize(src, 600, 0, imaging.Lanczos)
					imaging.Save(resized, cachePath)
					w.Header().Set("Cache-Control", "public, max-age=604800")
					w.Header().Set("Content-Type", "image/jpeg")
					imaging.Encode(w, resized, imaging.JPEG)
					return
				}
				os.WriteFile(cachePath, pic.Data, 0644)
				w.Header().Set("Cache-Control", "public, max-age=604800")
				w.Header().Set("Content-Type", pic.MIME)
				w.Write(pic.Data)
				return
			}
		}
	}

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
			limit := int64(10 * 1024 * 1024)
			imgData, err := io.ReadAll(io.LimitReader(fImg, limit))
			fImg.Close()
			if err != nil {
				continue
			}
			src, _, err := image.Decode(bytes.NewReader(imgData))
			if err == nil {
				resized := imaging.Resize(src, 600, 0, imaging.Lanczos)
				imaging.Save(resized, cachePath)
				w.Header().Set("Cache-Control", "public, max-age=604800")
				w.Header().Set("Content-Type", "image/jpeg")
				imaging.Encode(w, resized, imaging.JPEG)
				return
			}
			os.WriteFile(cachePath, imgData, 0644)
			w.Header().Set("Cache-Control", "public, max-age=604800")
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(imgData)
			return
		}
	}

	s.servePlaceholderCover(w)
}

func (s *Server) servePlaceholderCover(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 24 24" fill="none" stroke="#666" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>`)
}

func (s *Server) handleGetArtistImage(w http.ResponseWriter, r *http.Request) {
	artistID := s.extractID(r, "/api/artists/")
	artistID = strings.TrimSuffix(artistID, "/image")

	if artistID == "" {
		s.servePlaceholderArtist(w)
		return
	}

	cachePath := filepath.Join(s.Config.CachePath, "artists", artistID+".jpg")
	os.MkdirAll(filepath.Dir(cachePath), 0755)

	if _, err := os.Stat(cachePath); err == nil {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, cachePath)
		return
	}

	var imageURL string
	var name string
	err := s.getDB(r).Conn.QueryRow("SELECT image_url, name FROM artists WHERE id = ?", artistID).Scan(&imageURL, &name)
	if err != nil {
		s.servePlaceholderArtist(w)
		return
	}

	if imageURL == "" {
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

	resp, err := http.Get(imageURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		s.servePlaceholderArtist(w)
		return
	}
	defer resp.Body.Close()

	limit := int64(10 * 1024 * 1024)
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	imageProcessingSem <- struct{}{}
	defer func() { <-imageProcessingSem }()

	src, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		os.WriteFile(cachePath, imgData, 0644)
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Write(imgData)
		return
	}

	resized := imaging.Resize(src, 800, 0, imaging.Lanczos)
	_ = imaging.Save(resized, cachePath)

	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("Content-Type", "image/jpeg")
	imaging.Encode(w, resized, imaging.JPEG)
}

func (s *Server) servePlaceholderArtist(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 24 24" fill="none" stroke="#666" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>`)
}
