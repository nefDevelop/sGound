package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"sgound/internal/config"
	"sgound/internal/db"
)

func TestHandleGetInbox_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/inbox", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var tracks []interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 0 {
		t.Errorf("Expected 0 inbox tracks, got %d", len(tracks))
	}
}

func TestHandleGetInbox_WithPending(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Pending', '/path', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/inbox", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var tracks []interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 inbox track, got %d", len(tracks))
	}
}

func TestHandleApproveTracks(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Track', '/path', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"ids":["t1"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tracks/approve", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status string
	database.Conn.QueryRow("SELECT status FROM tracks WHERE id = 't1'").Scan(&status)
	if status != "active" {
		t.Errorf("Expected status 'active' after approve, got %q", status)
	}
}

func TestHandleApproveAllTracks(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Track1', '/1', 'pending')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t2', 'al1', 'Track2', '/2', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/tracks/approve-all", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	// Procesado asíncrono en segundo plano: 202 + espera activa.
	if rr.Code != http.StatusAccepted {
		t.Fatalf("Expected 202, got %d", rr.Code)
	}

	var count int
	for i := 0; i < 100; i++ {
		database.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'active'").Scan(&count)
		if count == 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if count != 2 {
		t.Errorf("Expected 2 active tracks, got %d", count)
	}
}

func TestHandleRejectTracks(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Track', '/path', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"ids":["t1"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tracks/reject", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status string
	database.Conn.QueryRow("SELECT status FROM tracks WHERE id = 't1'").Scan(&status)
	if status != "rejected" {
		t.Errorf("Expected status 'rejected' after reject, got %q", status)
	}
}

func TestHandleRateTrack(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Track', '/path', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"id":"t1","rating":5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tracks/rate", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rating, _, _ := database.GetTrackUserData("admin", "t1")
	if rating != 5 {
		t.Errorf("Expected rating 5, got %d", rating)
	}
}

func TestHandleLibraryHealth(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/health", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleLibraryDuplicates(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/duplicates", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleLibraryOrphans(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/orphans", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleLibraryLowQuality(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/low-quality", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleScanStatus(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/status", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if _, ok := resp["scanning"]; !ok {
		t.Error("Expected 'scanning' field in response")
	}
}

func TestHandleGetArtists_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleGetArtists_WithData(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var artists []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&artists)
	if len(artists) != 1 {
		t.Errorf("Expected 1 artist, got %d", len(artists))
	}
}

func TestHandleGetAlbums_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/albums", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleGetTracks_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/tracks", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleGetCover_NotFound(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/cover/nonexistent", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	// Should return placeholder or 404
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("Expected 200 or 404, got %d", rr.Code)
	}
}

func TestHandleGetStreams_NotFound(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/stream/nonexistent", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandlePlaylists_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/playlists", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleCreatePlaylist(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"name":"Test Playlist","description":"A test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/playlists", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetLogs(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleLibraryMetadata(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', '/path', 'active', 'mp3')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/metadata?id=t1", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetArtistImage_NotFound(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists/nonexistent/image", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	// Should return placeholder or 404
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("Expected 200 or 404, got %d", rr.Code)
	}
}

func TestHandleLibraryCleanup(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/library/cleanup", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleLyrics_NotFound(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/lyrics?id=nonexistent", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["lyrics"] != "" {
		t.Error("Expected empty lyrics for nonexistent track")
	}
}

func TestSetupRoutes_AllRoutesRegistered(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	// Create a real mp3 for streaming tests
	tmpDir := t.TempDir()
	mp3File := tmpDir + "/test.mp3"
	os.WriteFile(mp3File, []byte("fake audio"), 0644)

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song', ?, 'active')", mp3File)

	cfg := &config.Config{MusicPath: tmpDir}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	routes := []string{
		"/api/artists",
		"/api/albums",
		"/api/tracks",
		"/api/search?q=test",
		"/api/dashboard",
		"/api/library/health",
		"/api/library/duplicates",
		"/api/library/orphans",
		"/api/library/low-quality",
		"/api/library/inbox",
		"/api/library/status",
		"/api/playlists",
		"/api/logs",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		req.Header.Set("X-API-Key", "key")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Route %s returned %d, expected 200", route, rr.Code)
		}
	}
}
