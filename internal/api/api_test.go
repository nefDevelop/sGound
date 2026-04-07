package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"sgound/internal/config"
	"sgound/internal/db"
)

func TestGetArtistsEmpty(t *testing.T) {
	f, _ := os.CreateTemp("", "sgound-api-*.db")
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, _ := db.Connect(dbPath)
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp", Port: "8088"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var artists []map[string]string
	json.NewDecoder(rr.Body).Decode(&artists)

	if len(artists) != 0 {
		t.Errorf("Expected 0 artists on empty DB, got %d", len(artists))
	}
}

func TestStreamNotFound(t *testing.T) {
	f, _ := os.CreateTemp("", "sgound-api-*.db")
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, _ := db.Connect(dbPath)
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp", Port: "8088"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/stream/notfound", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown track, got %v", status)
	}
}

func TestStreamTranscoding(t *testing.T) {
	// Skip if ffmpeg is not available
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found, skipping transcoding test")
	}

	f, _ := os.CreateTemp("", "sgound-api-*.db")
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, _ := db.Connect(dbPath)
	defer database.Conn.Close()

	// Insert a dummy track
	tempMusic, _ := os.CreateTemp("", "test-*.mp3")
	tempMusicPath := tempMusic.Name()
	tempMusic.Close()
	defer os.Remove(tempMusicPath)

	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, duration, genre, size, format, bit_rate, sample_rate) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/test-track", "album-1", "Test Track", 1, 1, tempMusicPath, 10, "Genre", 100, "mp3", 128000, 44100)

	cfg := &config.Config{MusicPath: "/tmp", Port: "8088"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/stream/test-track?bitrate=64k", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected 200 for transcoding, got %v", status)
	}

	if rr.Header().Get("Content-Type") != "audio/mpeg" {
		t.Errorf("Expected audio/mpeg content type, got %v", rr.Header().Get("Content-Type"))
	}
}
