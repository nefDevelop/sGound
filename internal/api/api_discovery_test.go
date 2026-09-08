package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sgound/internal/config"
	"sgound/internal/db"
)

func TestHandleSearch_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty query, got %d", rr.Code)
	}
}

func TestHandleSearch_NoResults(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=nonexistent", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	tracks := resp["tracks"].([]interface{})
	if len(tracks) != 0 {
		t.Errorf("Expected 0 tracks, got %d", len(tracks))
	}
}

func TestHandleSearch_WithTracks(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Radiohead')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'OK Computer', 1997)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Karma Police', 'Radiohead', '/path', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=Karma", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	tracks := resp["tracks"].([]interface{})
	if len(tracks) < 1 {
		t.Error("Expected at least 1 track in results")
	}
}

func TestHandleAutocomplete_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/autocomplete", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var results []string
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestHandleAutocomplete_Artist(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Radiohead')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'OK Computer', 1997)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song', '/path', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/autocomplete?field=artist&q=Radio", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var results []string
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) < 1 {
		t.Error("Expected at least 1 artist in autocomplete")
	}
}

func TestHandleAutocomplete_UnknownField(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/autocomplete?field=unknown&q=test", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var results []string
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 0 {
		t.Errorf("Expected empty results for unknown field, got %d", len(results))
	}
}

func TestHandleDashboard_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["totalTracks"] != float64(0) {
		t.Errorf("Expected totalTracks=0, got %v", resp["totalTracks"])
	}
	if resp["totalAlbums"] != float64(0) {
		t.Errorf("Expected totalAlbums=0, got %v", resp["totalAlbums"])
	}
}

func TestHandleDashboard_WithData(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist1')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album1', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, genre) VALUES ('t1', 'al1', 'Song1', '/path', 'active', 'Rock')")
	database.Conn.Exec("INSERT INTO playlists (id, username, name) VALUES ('pl1', 'admin', 'My Playlist')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["totalTracks"] != float64(1) {
		t.Errorf("Expected totalTracks=1, got %v", resp["totalTracks"])
	}
	if resp["totalAlbums"] != float64(1) {
		t.Errorf("Expected totalAlbums=1, got %v", resp["totalAlbums"])
	}
	if resp["totalArtists"] != float64(1) {
		t.Errorf("Expected totalArtists=1, got %v", resp["totalArtists"])
	}
	if resp["totalPlaylists"] != float64(1) {
		t.Errorf("Expected totalPlaylists=1, got %v", resp["totalPlaylists"])
	}
}

func TestHandleDashboard_PathFilter(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("restricted", "hash", "r", "", "/music/rock")
	database.CreateAPIKey("rock-key", "restricted", "r", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Rock Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Rock Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Rock Song', '/music/rock/song.mp3', 'active')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t2', 'al1', 'Jazz Song', '/music/jazz/song.mp3', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("X-API-Key", "rock-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	// Should only see tracks under /music/rock/
	if resp["totalTracks"] != float64(1) {
		t.Errorf("Expected totalTracks=1 with path filter, got %v", resp["totalTracks"])
	}
}
