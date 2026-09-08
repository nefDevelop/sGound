package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"sgound/internal/auth"
	"sgound/internal/config"
	"sgound/internal/db"
)

func TestAPI_Flow(t *testing.T) {
	dbFile, _ := os.CreateTemp("", "sgound-api-test-*.db")
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, err := db.ConnectMaster(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}
	defer database.Conn.Close()

	cfg := &config.Config{
		DBPath:    dbPath,
		MusicPath: "/tmp",
		Port:      "8080",
		AllowedUsers: map[string]string{
			"admin": "rw",
		},
	}

	srv := NewServer(database, cfg)
	handler := srv.SetupRoutes()

	// 1. Test Login (should fail if no users)
	loginData := map[string]string{
		"username": "admin",
		"password": "password",
	}
	body, _ := json.Marshal(loginData)
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}

	// 2. Test Setup (Create first user)
	setupData := map[string]string{
		"username": "admin",
		"password": "password",
	}
	body, _ = json.Marshal(setupData)
	req = httptest.NewRequest("POST", "/api/auth/setup", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created for setup, got %d. Body: %s", w.Code, w.Body.String())
	}

	// 3. Test Login (should succeed now)
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for login, got %d", w.Code)
	}

	var loginResponse struct {
		Token string `json:"token"`
	}
	json.NewDecoder(w.Body).Decode(&loginResponse)
	if loginResponse.Token == "" {
		t.Fatal("Expected JWT token in login response")
	}

	// 4. Test Authenticated Request
	req = httptest.NewRequest("GET", "/api/artists", nil)
	req.Header.Set("Authorization", "Bearer "+loginResponse.Token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for authenticated request, got %d", w.Code)
	}
}

func TestAPI_SearchAfterInsert(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Pink Floyd')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'The Wall', 1979)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Another Brick in the Wall', 'Pink Floyd', '/path', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	// Search for the track
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=Another+Brick", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	tracks := resp["tracks"].([]interface{})
	if len(tracks) < 1 {
		t.Error("Expected to find track after insert")
	}
}

func TestAPI_AuthMethods_JWTHeader(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")

	secret, _ := database.GetJWTSecret()
	if secret == "" {
		database.SetJWTSecret("test-secret")
		secret = "test-secret"
	}
	token, _ := auth.GenerateJWT("admin", []byte(secret))

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with JWT header, got %d", rr.Code)
	}
}

func TestAPI_AuthMethods_Cookie(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")

	secret, _ := database.GetJWTSecret()
	if secret == "" {
		database.SetJWTSecret("test-secret")
		secret = "test-secret"
	}
	token, _ := auth.GenerateJWT("admin", []byte(secret))

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.AddCookie(&http.Cookie{Name: "sgound_token", Value: token})
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with JWT cookie, got %d", rr.Code)
	}
}

func TestAPI_AuthMethods_APIKey(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("api-key-123", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "api-key-123")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with API key, got %d", rr.Code)
	}
}

func TestAPI_TagEditing_Flow(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Old Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Old Album', 2000)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Old Title', '/fake/path.mp3', 'active', 'mp3')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	// Get metadata (should work even for fake path)
	req := httptest.NewRequest(http.MethodGet, "/api/library/metadata?id=t1", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for metadata, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAPI_DashboardStats_Consistency(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist1')")
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a2', 'Artist2')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album1', 2024)")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al2', 'a2', 'Album2', 2023)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, genre) VALUES ('t1', 'al1', 'Song1', '/1', 'active', 'Rock')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, genre) VALUES ('t2', 'al1', 'Song2', '/2', 'active', 'Rock')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, genre) VALUES ('t3', 'al2', 'Song3', '/3', 'active', 'Jazz')")
	database.Conn.Exec("INSERT INTO playlists (id, username, name) VALUES ('pl1', 'admin', 'My List')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["totalTracks"] != float64(3) {
		t.Errorf("Expected 3 tracks, got %v", resp["totalTracks"])
	}
	if resp["totalAlbums"] != float64(2) {
		t.Errorf("Expected 2 albums, got %v", resp["totalAlbums"])
	}
	if resp["totalArtists"] != float64(2) {
		t.Errorf("Expected 2 artists, got %v", resp["totalArtists"])
	}
	if resp["totalPlaylists"] != float64(1) {
		t.Errorf("Expected 1 playlist, got %v", resp["totalPlaylists"])
	}
}

func TestAPI_Search_Genres(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, genre) VALUES ('t1', 'al1', 'Rock Song', '/1', 'active', 'Rock')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=Rock", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	genres := resp["genres"].([]interface{})
	if len(genres) < 1 {
		t.Error("Expected at least 1 genre in search results")
	}
}

func TestAPI_Search_Playlists(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO playlists (id, username, name) VALUES ('pl1', 'admin', 'Rock Mix')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=Rock", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	playlists := resp["playlists"].([]interface{})
	if len(playlists) < 1 {
		t.Error("Expected at least 1 playlist in search results")
	}
}

func TestAPI_ReadOnlyUser_FullFlow(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("viewer", "hash", "r", "", "*")
	database.CreateAPIKey("viewer-key", "viewer", "r", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	// GET should work
	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "viewer-key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for GET with read-only user, got %d", rr.Code)
	}

	// POST should be forbidden
	req = httptest.NewRequest(http.MethodPost, "/api/scan", nil)
	req.Header.Set("X-API-Key", "viewer-key")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for POST with read-only user, got %d", rr.Code)
	}
}

func TestAPI_UnauthenticatedAccess(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	routes := []string{
		"/api/artists",
		"/api/albums",
		"/api/tracks",
		"/api/search?q=test",
		"/api/dashboard",
		"/api/library/health",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Route %s should return 401 without auth, got %d", route, rr.Code)
		}
	}
}
