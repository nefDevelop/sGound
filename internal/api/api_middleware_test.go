package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sgound/internal/config"
	"sgound/internal/db"
)

func TestAuthMiddleware_APIKey(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("valid-key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "valid-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with valid API key, got %d", rr.Code)
	}
}

func TestAuthMiddleware_APIKeyQuery(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("query-key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists?apiKey=query-key", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with apiKey query param, got %d", rr.Code)
	}
}

func TestAuthMiddleware_BlockedKey(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("blocked-key", "admin", "rw", "test")
	database.ToggleAPIKeyStatus("blocked-key", "admin", false)

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "blocked-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for blocked key, got %d", rr.Code)
	}
}

func TestAuthMiddleware_NoAuth(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ReadOnlyUser_Get(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("viewer", "hash", "r", "", "*")
	database.CreateAPIKey("viewer-key", "viewer", "r", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/artists", nil)
	req.Header.Set("X-API-Key", "viewer-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for GET with read-only key, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ReadOnlyUser_Post(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("viewer", "hash", "r", "", "*")
	database.CreateAPIKey("viewer-key", "viewer", "r", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/scan", nil)
	req.Header.Set("X-API-Key", "viewer-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for POST with read-only key, got %d", rr.Code)
	}
}

func TestSecurityHeaders_Present(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	headers := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
	}
	for _, h := range headers {
		if rr.Header().Get(h) == "" {
			t.Errorf("Missing security header: %s", h)
		}
	}
}

func TestSecurityHeaders_CORS(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Missing CORS Allow-Origin header")
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Missing CORS Allow-Methods header")
	}
}

func TestSecurityHeaders_OPTIONS(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodOptions, "/api/artists", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for OPTIONS, got %d", rr.Code)
	}
}

func TestSecurityHeaders_ContentSecurityPolicy(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Missing Content-Security-Policy header")
	}
}

func TestHandleConfig_GET(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{
		MusicPath: "/music",
		Port:      "9090",
		ThemeColor: "#ff0000",
	}
	server := NewServer(database, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	// Need auth for config
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("test-key", "admin", "rw", "test")

	mux := server.SetupRoutes()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["musicPath"] != "/music" {
		t.Errorf("Expected musicPath '/music', got %q", resp["musicPath"])
	}
}

func TestHandleConfig_PUT(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("test-key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/old"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"musicPath":"/new","themeColor":"#00ff00","language":"en","autoRename":true,"renamePattern":"{title}","replayGainEnabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if server.Config.MusicPath != "/new" {
		t.Errorf("Expected musicPath '/new', got %q", server.Config.MusicPath)
	}
}

func TestHandleConfig_PUT_InvalidPayload(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("test-key", "admin", "rw", "test")

	cfg := &config.Config{}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytesReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestHandleConfig_MethodNotAllowed(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("test-key", "admin", "rw", "test")

	cfg := &config.Config{}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodDelete, "/api/config", nil)
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}

func TestExtractID_FromQueryParam(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/artists?id=artist-123", nil)
	id := server.extractID(req, "/api/artists/")
	if id != "artist-123" {
		t.Errorf("Expected 'artist-123', got %q", id)
	}
}

func TestExtractID_FromPath(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/artists/artist-456", nil)
	id := server.extractID(req, "/api/artists/")
	if id != "artist-456" {
		t.Errorf("Expected 'artist-456', got %q", id)
	}
}

func TestExtractID_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/artists/", nil)
	id := server.extractID(req, "/api/artists/")
	if id != "" {
		t.Errorf("Expected empty ID, got %q", id)
	}
}

func TestGetPathFilter_Wildcard(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	user := &db.User{AllowedPaths: "*"}
	filter, args := server.getPathFilter(user)
	if filter != "" {
		t.Errorf("Expected empty filter for wildcard, got %q", filter)
	}
	if len(args) != 0 {
		t.Errorf("Expected no args for wildcard, got %d", len(args))
	}
}

func TestGetPathFilter_NilUser(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	filter, _ := server.getPathFilter(nil)
	if filter != "" {
		t.Errorf("Expected empty filter for nil user, got %q", filter)
	}
}

func TestGetPathFilter_SpecificPaths(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	user := &db.User{AllowedPaths: "/music/rock,/music/jazz"}
	filter, args := server.getPathFilter(user)

	if filter == "" {
		t.Error("Expected non-empty filter for specific paths")
	}
	if len(args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(args))
	}
}

// Helper function
func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}
