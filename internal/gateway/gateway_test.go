package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sgound/internal/db"
)

func setupTestGateway(t *testing.T) *SubsonicGateway {
	t.Helper()
	database, err := db.ConnectMaster(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	t.Cleanup(func() { database.Conn.Close() })
	return NewSubsonicGateway(database)
}

func TestNewSubsonicGateway(t *testing.T) {
	gw := setupTestGateway(t)
	if gw == nil {
		t.Fatal("NewSubsonicGateway returned nil")
	}
	if gw.DB == nil {
		t.Error("Gateway.DB should not be nil")
	}
}

func TestHandlePing_Success(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp SubsonicResponseWrapper
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.SubsonicResponse.Status != "ok" {
		t.Errorf("Expected status 'ok', got %q", resp.SubsonicResponse.Status)
	}
}

func TestHandlePing_Version(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp SubsonicResponseWrapper
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.SubsonicResponse.Version != "1.16.1" {
		t.Errorf("Expected version '1.16.1', got %q", resp.SubsonicResponse.Version)
	}
}

func TestHandlePing_Type(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp SubsonicResponseWrapper
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.SubsonicResponse.Type != "sgound" {
		t.Errorf("Expected type 'sgound', got %q", resp.SubsonicResponse.Type)
	}
}

func TestHandlePing_ServerVersion(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp SubsonicResponseWrapper
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.SubsonicResponse.ServerVersion != "1.0.0" {
		t.Errorf("Expected serverVersion '1.0.0', got %q", resp.SubsonicResponse.ServerVersion)
	}
}

func TestHandlePing_JSONContentType(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}
}

func TestHandlePing_WithQueryParams(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view?u=admin&v=1.16.1&t=token&s=salt", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with query params, got %d", rr.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	// Verify route is registered by making a request
	req := httptest.NewRequest(http.MethodGet, "/rest/ping.view", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Error("Route /rest/ping.view should be registered")
	}

	// Verify unregistered route returns 404
	req2 := httptest.NewRequest(http.MethodGet, "/rest/nonexistent.view", nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotFound {
		t.Errorf("Unregistered route should return 404, got %d", rr2.Code)
	}
}
