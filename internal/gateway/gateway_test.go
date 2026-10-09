package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sgound/internal/auth"
	"sgound/internal/db"
)

func setupTestGateway(t *testing.T) *SubsonicGateway {
	t.Helper()
	database, err := db.ConnectMaster(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	t.Cleanup(func() { database.Conn.Close() })
	return NewSubsonicGateway(database, "", "")
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

func setupAuthGateway(t *testing.T) *SubsonicGateway {
	t.Helper()
	gw := setupTestGateway(t)
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := gw.DB.CreateUser("tester", hash, "rw", "", "*"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return gw
}

func TestAuth_MissingVersion(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/getArtists.view?c=test", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `code="10"`) {
		t.Errorf("Se esperaba error 10, respuesta: %s", body)
	}
}

func TestAuth_BadCredentials(t *testing.T) {
	gw := setupTestGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/getArtists.view?v=1.16.1&c=test&u=nadie&p=x", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), `code="40"`) {
		t.Errorf("Se esperaba error 40, respuesta: %s", rr.Body.String())
	}
}

func TestAuth_TokenRejected(t *testing.T) {
	gw := setupAuthGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/getArtists.view?v=1.16.1&c=test&u=tester&t=abc&s=def", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), `code="41"`) {
		t.Errorf("Se esperaba error 41, respuesta: %s", rr.Body.String())
	}
}

func TestGetArtists_EmptyXML(t *testing.T) {
	gw := setupAuthGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/getArtists.view?v=1.16.1&c=test&u=tester&p=secret123", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	body := rr.Body.String()
	for _, want := range []string{`<subsonic-response`, `status="ok"`, `<artists`, `openSubsonic="true"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Falta %q en: %s", want, body)
		}
	}
}

func TestGetArtists_JSON(t *testing.T) {
	gw := setupAuthGateway(t)
	mux := http.NewServeMux()
	gw.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/rest/getArtists.view?v=1.16.1&c=test&f=json&u=tester&p=secret123", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	sub, ok := resp["subsonic-response"]
	if !ok || sub["status"] != "ok" {
		t.Errorf("Respuesta inesperada: %v", resp)
	}
}
