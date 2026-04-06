package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"sgound/internal/db"
)

func TestGetArtistsEmpty(t *testing.T) {
	f, _ := os.CreateTemp("", "sgound-api-*.db")
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, _ := db.Connect(dbPath)
	defer database.Conn.Close()

	server := NewServer(database)
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

	server := NewServer(database)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/stream/notfound", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown track, got %v", status)
	}
}
