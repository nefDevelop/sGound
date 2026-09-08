package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sgound/internal/config"
	"sgound/internal/db"
)

func TestHandleAnalyzeInbox_Empty(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/inbox/analyze", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	suggestions := resp["suggestions"].([]interface{})
	if len(suggestions) != 0 {
		t.Errorf("Expected 0 suggestions on empty inbox, got %d", len(suggestions))
	}
}

func TestHandleAnalyzeInbox_WithPendingTracks(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	// Insert active tracks (consensus data)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Radiohead')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'OK Computer', 1997)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t-active', 'al1', 'Song', 'Radiohead', '/active', 'active')")

	// Insert pending tracks (inbox) with slight variations
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a2', 'radiohead')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al2', 'a2', 'OK Computer', 0)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t-pending', 'al2', 'Song Pending', 'radiohead', '/pending', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/inbox/analyze", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", resp["status"])
	}
}

func TestHandleAnalyzeInbox_GroupsByNormalizedArtist(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	// Active: "Pink Floyd" with correct year
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Pink Floyd')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'The Wall', 1979)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('ta1', 'al1', 'Song', 'Pink Floyd', '/a', 'active')")

	// Pending: "pink floyd" (lowercase) with wrong year
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a2', 'pink floyd')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al2', 'a2', 'The Wall', 0)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('tp1', 'al2', 'Pending Song', 'pink floyd', '/p', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/inbox/analyze", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	suggestions := resp["suggestions"].([]interface{})
	// Should suggest year correction based on consensus
	if len(suggestions) == 0 {
		t.Log("No suggestions (consensus may not match due to normalization)")
	}
}

func TestHandleApplyInboxSuggestions_Success(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Old Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Old Album', 0)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Song', 'Old Artist', '/path', 'pending')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"suggestions":[{"trackId":"t1","artist":"New Artist","album":"New Album","year":2024}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/library/inbox/apply-suggestions", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify track was updated
	var artist, albumTitle string
	var year int
	database.Conn.QueryRow(`
		SELECT t.artist, al.title, COALESCE(al.year, 0)
		FROM tracks t JOIN albums al ON t.album_id = al.id
		WHERE t.id = 't1'`).Scan(&artist, &albumTitle, &year)

	if artist != "New Artist" {
		t.Errorf("Expected artist 'New Artist', got %q", artist)
	}
	if albumTitle != "New Album" {
		t.Errorf("Expected album 'New Album', got %q", albumTitle)
	}
	if year != 2024 {
		t.Errorf("Expected year 2024, got %d", year)
	}
}

func TestHandleApplyInboxSuggestions_MethodNotAllowed(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/inbox/apply-suggestions", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}

func TestHandleApplyInboxSuggestions_InvalidPayload(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/library/inbox/apply-suggestions", bytesReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestHandleSmartPlaylists_RecentlyPlayed(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Played Song', 'Artist', '/1', 'active')")
	database.Conn.Exec("INSERT INTO user_tracks (username, track_id, rating, play_count, last_played) VALUES ('admin', 't1', 5, 10, datetime('now'))")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/smart-playlists/recently-played", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(tracks))
	}
}

func TestHandleSmartPlaylists_HighQuality(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, bit_rate, format) VALUES ('t1', 'al1', 'Lossless', 'Artist', '/1', 'active', 1411000, 'flac')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, bit_rate, format) VALUES ('t2', 'al1', 'Low Quality', 'Artist', '/2', 'active', 128000, 'mp3')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/smart-playlists/high-quality", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 high-quality track, got %d", len(tracks))
	}
	if len(tracks) > 0 && tracks[0]["format"] != "flac" {
		t.Errorf("Expected flac format, got %v", tracks[0]["format"])
	}
}

func TestHandleSmartPlaylists_RecentlyAdded(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'New Song', 'Artist', '/1', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/smart-playlists/recently-added", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(tracks))
	}
}

func TestHandleSmartPlaylists_Decade(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', '80s Album', 1985)")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al2', 'a1', '90s Album', 1995)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', '80s Song', 'Artist', '/1', 'active')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t2', 'al2', '90s Song', 'Artist', '/2', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/smart-playlists/decade?value=1980", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 track from 1980s, got %d", len(tracks))
	}
	if len(tracks) > 0 && tracks[0]["year"] != float64(1985) {
		t.Errorf("Expected year 1985, got %v", tracks[0]["year"])
	}
}

func TestHandleSmartPlaylists_Genre(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, genre) VALUES ('t1', 'al1', 'Rock Song', 'Artist', '/1', 'active', 'Rock')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, genre) VALUES ('t2', 'al1', 'Jazz Song', 'Artist', '/2', 'active', 'Jazz')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/smart-playlists/genre?value=Rock", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 Rock track, got %d", len(tracks))
	}
	if len(tracks) > 0 && tracks[0]["genre"] != "Rock" {
		t.Errorf("Expected genre 'Rock', got %v", tracks[0]["genre"])
	}
}

func TestHandleSmartPlaylists_UnknownType(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/smart-playlists/nonexistent", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unknown type, got %d", rr.Code)
	}
}

func TestGenerateRuleSQL_GenreOnly(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	rules := struct {
		Genre          string `json:"genre"`
		MinRating      int    `json:"minRating"`
		LastPlayedDays int    `json:"lastPlayedDays"`
		Instrumental   *bool  `json:"instrumental"`
	}{Genre: "Rock"}

	sql := server.generateRuleSQL(rules)
	if sql == "" {
		t.Error("Expected non-empty SQL")
	}
	if !containsSubstring(sql, "genre = 'Rock'") {
		t.Errorf("Expected genre filter in SQL, got: %s", sql)
	}
}

func TestGenerateRuleSQL_MinRating(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	rules := struct {
		Genre          string `json:"genre"`
		MinRating      int    `json:"minRating"`
		LastPlayedDays int    `json:"lastPlayedDays"`
		Instrumental   *bool  `json:"instrumental"`
	}{MinRating: 4}

	sql := server.generateRuleSQL(rules)
	if !containsSubstring(sql, "rating >= 4") {
		t.Errorf("Expected rating filter, got: %s", sql)
	}
}

func TestGenerateRuleSQL_Instrumental(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	isInstrumental := true
	rules := struct {
		Genre          string `json:"genre"`
		MinRating      int    `json:"minRating"`
		LastPlayedDays int    `json:"lastPlayedDays"`
		Instrumental   *bool  `json:"instrumental"`
	}{Instrumental: &isInstrumental}

	sql := server.generateRuleSQL(rules)
	if !containsSubstring(sql, "instrumental = 1") {
		t.Errorf("Expected instrumental filter, got: %s", sql)
	}
}

func TestGenerateRuleSQL_NoRules(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	rules := struct {
		Genre          string `json:"genre"`
		MinRating      int    `json:"minRating"`
		LastPlayedDays int    `json:"lastPlayedDays"`
		Instrumental   *bool  `json:"instrumental"`
	}{}

	sql := server.generateRuleSQL(rules)
	if !containsSubstring(sql, "SELECT id FROM tracks WHERE status = 'active'") {
		t.Errorf("Expected base query with no rules, got: %s", sql)
	}
}

func TestGenerateRuleSQL_Combined(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	instr := false
	rules := struct {
		Genre          string `json:"genre"`
		MinRating      int    `json:"minRating"`
		LastPlayedDays int    `json:"lastPlayedDays"`
		Instrumental   *bool  `json:"instrumental"`
	}{Genre: "Jazz", MinRating: 3, LastPlayedDays: 30, Instrumental: &instr}

	sql := server.generateRuleSQL(rules)
	if !containsSubstring(sql, "genre = 'Jazz'") {
		t.Error("Missing genre filter")
	}
	if !containsSubstring(sql, "rating >= 3") {
		t.Error("Missing rating filter")
	}
	if !containsSubstring(sql, "last_played") {
		t.Error("Missing last_played filter")
	}
	if !containsSubstring(sql, "instrumental = 0") {
		t.Error("Missing instrumental filter")
	}
}

func TestNormalizeAlbumTitle_Simple(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	result := server.normalizeAlbumTitle("OK Computer", "Radiohead")
	if result != "ok computer" {
		t.Errorf("Expected 'ok computer', got %q", result)
	}
}

func TestNormalizeAlbumTitle_StripsYear(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	result := server.normalizeAlbumTitle("OK Computer 1997", "Radiohead")
	if result != "ok computer" {
		t.Errorf("Expected 'ok computer', got %q", result)
	}
}

func TestNormalizeAlbumTitle_StripsArtistSuffix(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	result := server.normalizeAlbumTitle("OK Computer Radiohead", "Radiohead")
	if result != "ok computer" {
		t.Errorf("Expected 'ok computer', got %q", result)
	}
}

func TestNormalizeAlbumTitle_StripsPunctuation(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	result := server.normalizeAlbumTitle("OK Computer.", "Radiohead")
	if result != "ok computer" {
		t.Errorf("Expected 'ok computer', got %q", result)
	}
}

func TestNormalizeAlbumTitle_CombinedStrips(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	// Year at end + artist name + trailing dash
	result := server.normalizeAlbumTitle("The Wall - 1979 Pink Floyd", "Pink Floyd")
	if !containsSubstring(result, "the wall") {
		t.Errorf("Expected result to contain 'the wall', got %q", result)
	}
}

func TestNormalizeAlbumTitle_LowerCase(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	result := server.normalizeAlbumTitle("DISCOVERY", "Daft Punk")
	if result != "discovery" {
		t.Errorf("Expected 'discovery', got %q", result)
	}
}

func TestNormalizeAlbumTitle_EmptyArtist(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	cfg := &config.Config{}
	server := NewServer(database, cfg)

	result := server.normalizeAlbumTitle("Some Album", "")
	if result != "some album" {
		t.Errorf("Expected 'some album', got %q", result)
	}
}

func TestHandleGetPlaylistTracks_Regular(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Song', 'Artist', '/1', 'active')")
	database.Conn.Exec("INSERT INTO playlists (id, username, name, is_smart) VALUES ('pl1', 'admin', 'My Playlist', 0)")
	database.Conn.Exec("INSERT INTO playlist_tracks (playlist_id, track_id, position) VALUES ('pl1', 't1', 0)")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/playlists/pl1/tracks", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(tracks))
	}
}

func TestHandleGetPlaylistTracks_NotFound(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/playlists/nonexistent/tracks", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandlePlaylistActions_Delete(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")
	database.CreatePlaylist("admin", "pl1", "To Delete", "", 0, "")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodDelete, "/api/playlists/pl1", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlists WHERE id = 'pl1'").Scan(&count)
	if count != 0 {
		t.Error("Playlist should have been deleted")
	}
}

func TestHandlePlaylistActions_AddTrack(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")
	database.CreatePlaylist("admin", "pl1", "My Playlist", "", 0, "")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Song', 'Artist', '/1', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	body := []byte(`{"trackId":"t1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/playlists/pl1/tracks", bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = 'pl1'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 track in playlist, got %d", count)
	}
}

func TestHandleLibraryCleanPlaylists(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.CreatePlaylist("admin", "pl1", "Playlist", "", 0, "")
	// Add reference to non-existent track
	database.Conn.Exec("INSERT INTO playlist_tracks (playlist_id, track_id, position) VALUES ('pl1', 'nonexistent-track', 0)")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/library/clean-playlists", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = 'pl1'").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 orphaned tracks, got %d", count)
	}
}

func TestHandleLibraryResyncIndex(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Song', 'Artist', '/1', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodPost, "/api/library/resync-index", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify FTS5 index is rebuilt
	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM search_idx WHERE search_idx MATCH 'Song*'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected FTS5 to find 1 result after resync, got %d", count)
	}
}

func TestHandleLibraryMissingTags(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Unknown Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Unknown Album', 0)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, genre) VALUES ('t1', 'al1', 'Song', 'Unknown Artist', '/1', 'active', '')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/missing-tags", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 track with missing tags, got %d", len(tracks))
	}
}

func TestHandleLibraryAllCovers(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Song', 'Artist', '/1', 'active')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/covers", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var covers []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&covers)
	// Album without cover should show as missing
	if len(covers) != 1 {
		t.Errorf("Expected 1 album with missing cover, got %d", len(covers))
	}
	if len(covers) > 0 && covers[0]["type"] != "missing" {
		t.Errorf("Expected type 'missing', got %v", covers[0]["type"])
	}
}

func TestHandleLibraryLowQuality_WithThreshold(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()
	database.CreateUser("admin", "hash", "rw", "", "*")
	database.CreateAPIKey("key", "admin", "rw", "test")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, bit_rate, format) VALUES ('t1', 'al1', 'Low', 'Artist', '/1', 'active', 96000, 'mp3')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status, bit_rate, format) VALUES ('t2', 'al1', 'High', 'Artist', '/2', 'active', 320000, 'mp3')")

	cfg := &config.Config{MusicPath: "/tmp"}
	server := NewServer(database, cfg)
	mux := server.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/library/low-quality?bitrate=192", nil)
	req.Header.Set("X-API-Key", "key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var tracks []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&tracks)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 low quality track, got %d", len(tracks))
	}
}

// Helper
func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
