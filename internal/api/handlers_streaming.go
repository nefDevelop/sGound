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
	"time"

	"github.com/disintegration/imaging"
	"sgound/internal/db"
	"sgound/internal/tags"
	"sgound/internal/utils"
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

	// Bypass cache if version parameter is present
	forceRefresh := r.URL.Query().Get("v") != ""
	if _, err := os.Stat(cachePath); err == nil && !forceRefresh {
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
					resized := imaging.Resize(src, 600, 0, imaging.Linear)
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
				resized := imaging.Resize(src, 600, 0, imaging.Linear)
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
	// Intentar obtener de PathValue (Go 1.22+) primero
	artistID := r.PathValue("id")
	if artistID == "" {
		artistID = s.extractID(r, "/api/artists/")
		artistID = strings.TrimSuffix(artistID, "/image")
	}
	artistID = strings.TrimSpace(artistID)

	if artistID == "" {
		slog.Warn("Empty artist ID in image request", "path", r.URL.Path)
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

	// Final check: if we have it in cache (either from before or just processed), serve it
	_, err, _ := s.sf.Do("artist_img_"+artistID, func() (interface{}, error) {
		// Re-check cache inside singleflight to ensure we don't process if another request finished
		if _, err := os.Stat(cachePath); err == nil {
			return nil, nil
		}

		var imageURL string
		var name string
		
		// 1. Intentar buscar por ID exacto
		err := s.getDB(r).Conn.QueryRow("SELECT image_url, name FROM artists WHERE id = ?", artistID).Scan(&imageURL, &name)
		if err != nil {
			// (Lógica de auto-repair que ya teníamos...)
			var foundName string
			errScan := s.getDB(r).Conn.QueryRow("SELECT artist FROM tracks WHERE status = 'active' AND (artist IS NOT NULL AND artist != '') AND ? = ?", artistID, artistID).Scan(&foundName)
			
			if errScan != nil {
				rows, _ := s.getDB(r).Conn.Query("SELECT DISTINCT artist FROM tracks WHERE status = 'active'")
				if rows != nil {
					defer rows.Close()
					for rows.Next() {
						var aName string
						if err := rows.Scan(&aName); err == nil {
							if utils.GenerateID(aName) == artistID {
								foundName = aName
								break
							}
						}
					}
				}
			}

			if foundName != "" {
				slog.Info("🛠️ Auto-repairing missing artist record", "id", artistID, "name", foundName)
				_, _ = s.getDB(r).Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistID, foundName)
				name = foundName
			} else {
				if len(artistID) != 40 {
					err = s.getDB(r).Conn.QueryRow("SELECT id, image_url, name FROM artists WHERE name = ?", artistID).Scan(&artistID, &imageURL, &name)
				}
				
				if err != nil && foundName == "" {
					return nil, fmt.Errorf("artist not found in DB: %s", artistID)
				}
			}
		}

		if imageURL == "" {
			// Usar SingleFlight por NOMBRE de artista para evitar peticiones redundantes a Wikipedia/Last.fm
			res, _, _ := s.sf.Do("ext_meta_"+name, func() (interface{}, error) {
				slog.Info("🔍 No image URL in DB, fetching external info", "artist", name)
				ext, _ := s.externalMeta.GetArtistInfo(name)
				if ext != nil && ext.Image != "" {
					tagsStr := strings.Join(ext.Tags, ",")
					similarStr := strings.Join(ext.Similar, ",")
					_, _ = s.getDB(r).Conn.Exec("UPDATE artists SET image_url = ?, bio = ?, tags = ?, similar = ? WHERE id = ?",
						ext.Image, ext.Biography, tagsStr, similarStr, artistID)
					return ext.Image, nil
				}
				return "", nil
			})
			imageURL = res.(string)
		}

		if imageURL == "" {
			return nil, fmt.Errorf("no image url found for artist: %s", name)
		}

		slog.Info("📥 Downloading artist image", "artist", name, "url", imageURL)

		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest("GET", imageURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("network error fetching %s: %v", name, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d error for %s", resp.StatusCode, name)
		}

		limit := int64(10 * 1024 * 1024)
		imgData, err := io.ReadAll(io.LimitReader(resp.Body, limit))
		if err != nil {
			return nil, err
		}

		imageProcessingSem <- struct{}{}
		defer func() { <-imageProcessingSem }()

		src, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			slog.Warn("❌ Failed to decode image", "artist", name, "size", len(imgData), "error", err)
			return nil, err
		}

		slog.Info("🎨 Image decoded", "artist", name, "format", format, "size", len(imgData))

		// Asegurar directorio antes de guardar
		cacheDir := filepath.Dir(cachePath)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			slog.Error("📁 Primary cache failed, trying system temp", "path", cacheDir, "error", err)
			// Fallback: usar directorio temporal del sistema si el configurado falla
			tempCacheDir := filepath.Join(os.TempDir(), "sgound-cache", "artists")
			os.MkdirAll(tempCacheDir, 0755)
			cachePath = filepath.Join(tempCacheDir, artistID+".jpg")
		}

		resized := imaging.Resize(src, 800, 0, imaging.Linear)
		err = imaging.Save(resized, cachePath)
		if err != nil {
			slog.Error("💾 Failed to save image to cache", "path", cachePath, "error", err)
			return nil, err
		}

		slog.Info("✨ Successfully cached artist image", "artist", name, "path", cachePath)
		return nil, nil
	})

	if err != nil {
		slog.Warn("Artist image process failed", "artistID", artistID, "error", err)
		// Si falló y el archivo NO existe, servimos el placeholder sin cachear el error
		if _, statErr := os.Stat(cachePath); statErr != nil {
			s.servePlaceholderArtist(w)
			return
		}
	}

	if _, statErr := os.Stat(cachePath); statErr == nil {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, cachePath)
	} else {
		s.servePlaceholderArtist(w)
	}
}

func (s *Server) servePlaceholderArtist(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 24 24" fill="none" stroke="#666" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>`)
}
