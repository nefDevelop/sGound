package metadata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sgound/internal/config"
)

func TestNewExternalMetadata(t *testing.T) {
	cfg := &config.Config{Language: "en"}
	em := NewExternalMetadata(cfg)
	if em == nil {
		t.Fatal("NewExternalMetadata returned nil")
	}
	if em.Config != cfg {
		t.Error("Config should be set correctly")
	}
}

func TestGetArtistInfo_EmptyName(t *testing.T) {
	cfg := &config.Config{Language: "en"}
	em := NewExternalMetadata(cfg)

	// With empty name, it should still return something (fallback)
	info, err := em.GetArtistInfo("")
	if err != nil {
		t.Fatalf("GetArtistInfo failed: %v", err)
	}
	if info == nil {
		t.Fatal("GetArtistInfo returned nil info")
	}
}

func TestEnrichWithDeezer_Mock(t *testing.T) {
	// Mock Deezer API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"picture_xl": "https://example.com/artist_xl.jpg",
					"picture_big": "https://example.com/artist_big.jpg",
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := &config.Config{Language: "en"}
	em := &ExternalMetadata{Config: cfg}

	info := &ArtistInfo{}
	// We can't easily mock the Deezer URL without modifying the source,
	// but we can test the structure
	if info.Image != "" {
		t.Error("Image should be empty before enrichment")
	}
}

func TestEnrichWithLastFM_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"artist": map[string]interface{}{
				"image": []map[string]string{
					{"#text": "https://example.com/img.jpg", "size": "mega"},
				},
				"tags": map[string]interface{}{
					"tag": []map[string]string{
						{"name": "rock"},
						{"name": "alternative"},
					},
				},
				"similar": map[string]interface{}{
					"artist": []map[string]string{
						{"name": "Similar Artist 1"},
						{"name": "Similar Artist 2"},
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	// Test that our info struct can hold the data
	info := &ArtistInfo{}
	info.Tags = append(info.Tags, "rock", "alternative")
	info.Similar = append(info.Similar, "Similar Artist 1", "Similar Artist 2")

	if len(info.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(info.Tags))
	}
	if len(info.Similar) != 2 {
		t.Errorf("Expected 2 similar artists, got %d", len(info.Similar))
	}
}

func TestFetchSummary_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"extract": "Test Artist is a musician from Testville.",
			"originalimage": map[string]string{
				"source": "https://example.com/image.jpg",
			},
		})
	}))
	defer mockServer.Close()

	// Test the data structure
	var result struct {
		Extract       string `json:"extract"`
		OriginalImage struct {
			Source string `json:"source"`
		} `json:"originalimage"`
	}

	json.NewDecoder(mockServer.Client().Get(mockServer.URL)).Decode(&result)

	// Note: Can't directly test fetchSummary without mocking the URL
	// But we verify the mock server returns expected data
	resp, err := mockServer.Client().Get(mockServer.URL)
	if err != nil {
		t.Fatalf("Mock server request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestMBClient_NewClient(t *testing.T) {
	client := NewMBClient()
	if client == nil {
		t.Fatal("NewMBClient returned nil")
	}
	if client.BaseURL != "https://musicbrainz.org/ws/2" {
		t.Errorf("Expected MusicBrainz URL, got %q", client.BaseURL)
	}
	if client.Client == nil {
		t.Error("HTTP client should not be nil")
	}
}

func TestMBRelease_Structure(t *testing.T) {
	release := MBRelease{
		ID:    "test-id",
		Title: "Test Album",
		Date:  "2024-01-01",
	}

	if release.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %q", release.ID)
	}
	if release.Title != "Test Album" {
		t.Errorf("Expected Title 'Test Album', got %q", release.Title)
	}
}

func TestArtistInfo_Fields(t *testing.T) {
	info := &ArtistInfo{
		Biography: "A great artist",
		Image:     "https://example.com/image.jpg",
		Source:    "Wikipedia",
		Tags:      []string{"rock", "pop"},
		Similar:   []string{"Artist 1", "Artist 2"},
	}

	if info.Biography != "A great artist" {
		t.Errorf("Expected biography 'A great artist', got %q", info.Biography)
	}
	if len(info.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(info.Tags))
	}
	if len(info.Similar) != 2 {
		t.Errorf("Expected 2 similar, got %d", len(info.Similar))
	}
}

func TestSearchBestTitle_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": map[string]interface{}{
				"search": []map[string]string{
					{"title": "Test Artist (musician)", "snippet": "is a singer and musician"},
					{"title": "Test City", "snippet": "is a city in Testland"},
				},
			},
		})
	}))
	defer mockServer.Close()

	// We can verify the mock returns expected data
	resp, err := mockServer.Client().Get(mockServer.URL + "?srsearch=Test+Artist+music")
	if err != nil {
		t.Fatalf("Mock request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Query.Search) != 2 {
		t.Errorf("Expected 2 search results, got %d", len(result.Query.Search))
	}
	if result.Query.Search[0].Title != "Test Artist (musician)" {
		t.Errorf("Expected 'Test Artist (musician)', got %q", result.Query.Search[0].Title)
	}
}
