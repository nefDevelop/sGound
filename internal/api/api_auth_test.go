package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sgound/internal/auth"
	"sgound/internal/config"
	"sgound/internal/db"
)

func setupTestServer(t *testing.T) (*Server, *db.MasterDatabase) {
	t.Helper()
	database, err := db.ConnectMaster(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	t.Cleanup(func() { database.Conn.Close() })

	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("test-key", "admin", "rw", "test")

	cfg := &config.Config{
		MusicPath:    "/tmp",
		Port:         "8088",
		AllowedUsers: map[string]string{"admin": "rw", "viewer": "r"},
	}
	server := NewServer(database, cfg)
	return server, database
}

func TestHandleLogin_Success(t *testing.T) {
	server, database := setupTestServer(t)

	// Create user with proper password hash
	hash, _ := auth.HashPassword("mypassword")
	database.Conn.Exec("UPDATE users SET password_hash = ? WHERE username = 'admin'", hash)

	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "mypassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("Expected token in response")
	}
	if resp["username"] != "admin" {
		t.Errorf("Expected username 'admin', got %q", resp["username"])
	}
	if resp["permissions"] != "rw" {
		t.Errorf("Expected permissions 'rw', got %q", resp["permissions"])
	}
}

func TestHandleLogin_BadPassword(t *testing.T) {
	server, database := setupTestServer(t)
	hash, _ := auth.HashPassword("correct")
	database.Conn.Exec("UPDATE users SET password_hash = ? WHERE username = 'admin'", hash)

	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrong",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestHandleLogin_NonexistentUser(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"username": "nonexistent",
		"password": "pass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestHandleLogin_InvalidPayload(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("invalid")))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestHandleLogin_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}

func TestHandleSetup_Success(t *testing.T) {
	server, database := setupTestServer(t)
	// Delete existing users to allow setup
	database.Conn.Exec("DELETE FROM users")

	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "newpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "success" {
		t.Errorf("Expected status 'success', got %q", resp["status"])
	}
}

func TestHandleSetup_NotAllowed(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"username": "unauthorized",
		"password": "pass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestHandleSetup_AlreadyExists(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "pass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for existing user, got %d", rr.Code)
	}
}

func TestHandleSetup_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/setup", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}

func TestHandleAuthStatus_NoUsers(t *testing.T) {
	server, database := setupTestServer(t)
	database.Conn.Exec("DELETE FROM users")

	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["setupRequired"] != true {
		t.Error("Expected setupRequired=true when no users")
	}
}

func TestHandleAuthStatus_WithUsers(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["setupRequired"] != false {
		t.Error("Expected setupRequired=false when users exist")
	}
}

func TestHandleAPIKeys_Get(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/apikeys", nil)
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var keys []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&keys)
	if len(keys) < 1 {
		t.Error("Expected at least 1 API key")
	}
}

func TestHandleAPIKeys_Create(t *testing.T) {
	server, _ := setupTestServer(t)
	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"label":       "New Key",
		"permissions": "r",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["key"] == nil || resp["key"] == "" {
		t.Error("Expected key in response")
	}
}

func TestHandleAPIKeys_Delete(t *testing.T) {
	server, database := setupTestServer(t)
	database.CreateAPIKey("to-delete", "admin", "r", "Delete Me")
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/apikeys?key=to-delete", nil)
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rr.Code)
	}

	_, err := database.GetAPIKeyInfo("to-delete")
	if err == nil {
		t.Error("Key should have been deleted")
	}
}

func TestHandleAPIKeys_Toggle(t *testing.T) {
	server, database := setupTestServer(t)
	database.CreateAPIKey("toggle-key", "admin", "r", "Toggle Me")
	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]interface{}{
		"key":    "toggle-key",
		"active": false,
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/auth/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	keyInfo, _ := database.GetAPIKeyInfo("toggle-key")
	if keyInfo.IsActive {
		t.Error("Key should be inactive after toggle")
	}
}

func TestHandleAPIKeys_ReadOnlyGuard(t *testing.T) {
	server, database := setupTestServer(t)
	database.CreateUser("viewer", "hash", "r", "", "*")
	database.CreateAPIKey("viewer-key", "viewer", "r", "Viewer Key")
	mux := server.SetupRoutes()

	body, _ := json.Marshal(map[string]string{
		"label":       "RW Key",
		"permissions": "rw",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "viewer-key")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for read-only user creating RW key, got %d", rr.Code)
	}
}
