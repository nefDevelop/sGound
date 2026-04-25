package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"sgound/internal/config"
	"sgound/internal/db"
)

func TestAPI_Flow(t *testing.T) {
	// 1. Setup temp DB
	dbFile, _ := os.CreateTemp("", "sgound-api-test-*.db")
	dbPath := dbFile.Name()
	dbFile.Close()
	defer os.Remove(dbPath)

	database, err := db.ConnectMaster(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}
	defer database.Conn.Close()

	// 2. Setup Config
	cfg := &config.Config{
		DBPath:    dbPath,
		MusicPath: "/tmp",
		Port:      "8080",
		AllowedUsers: map[string]string{
			"admin": "rw",
		},
	}

	// 3. Initialize Server
	srv := NewServer(database, cfg)
	handler := srv.SetupRoutes()

	// 4. Test Login (should fail if no users)
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

	// 5. Test Setup (Create first user)
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

	// 6. Test Login (should succeed now)
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

	// 7. Test Authenticated Request
	req = httptest.NewRequest("GET", "/api/artists", nil)
	req.Header.Set("Authorization", "Bearer "+loginResponse.Token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for authenticated request, got %d", w.Code)
	}
}
