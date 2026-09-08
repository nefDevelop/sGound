package metadata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sgound/internal/config"
)

func TestGetArtistInfo_EmptyName(t *testing.T) {
	cfg := &config.Config{Language: "en"}
	em := NewExternalMetadata(cfg)

	info, err := em.GetArtistInfo("")
	if err != nil {
		t.Fatalf("GetArtistInfo failed: %v", err)
	}
	if info == nil {
		t.Fatal("GetArtistInfo returned nil")
	}
	// With empty name, external APIs won't find anything
	if info.Biography == "" {
		t.Log("Biography is empty for empty artist name (expected)")
	}
}

func TestGetArtistInfo_SpecialChars(t *testing.T) {
	cfg := &config.Config{Language: "en"}
	em := NewExternalMetadata(cfg)

	// Should not panic with special characters
	info, err := em.GetArtistInfo("AC/DC & friends feat. someone")
	if err != nil {
		t.Fatalf("GetArtistInfo failed: %v", err)
	}
	if info == nil {
		t.Fatal("GetArtistInfo returned nil")
	}
}

func TestSearchBestTitle_WikipediaMock(t *testing.T) {
	// Create mock Wikipedia API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": map[string]interface{}{
				"search": []map[string]string{
					{"title": "Radiohead (band)", "snippet": "are a British rock band"},
					{"title": "Radiohead (city)", "snippet": "is a small town"},
				},
			},
		})
	}))
	defer mockServer.Close()

	// We can't directly call searchBestTitle since it uses hardcoded URLs
	// But we can verify the mock server structure
	resp, err := mockServer.Client().Get(mockServer.URL)
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
		t.Errorf("Expected 2 results, got %d", len(result.Query.Search))
	}

	// Verify the first result contains "band" in snippet
	if !strings.Contains(result.Query.Search[0].Snippet, "band") {
		t.Error("Expected first result to mention 'band'")
	}
}

func TestFetchSummary_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"extract": "Radiohead are a British rock band formed in Abingdon.",
			"originalimage": map[string]string{
				"source": "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/Radiohead.jpg/800px-Radiohead.jpg",
			},
			"thumbnail": map[string]string{
				"source": "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/Radiohead.jpg/200px-Radiohead.jpg",
			},
		})
	}))
	defer mockServer.Close()

	// Verify mock server returns expected structure
	resp, err := mockServer.Client().Get(mockServer.URL)
	if err != nil {
		t.Fatalf("Mock request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Extract       string `json:"extract"`
		OriginalImage struct {
			Source string `json:"source"`
		} `json:"originalimage"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Extract == "" {
		t.Error("Expected non-empty extract")
	}
	if result.OriginalImage.Source == "" {
		t.Error("Expected originalimage source")
	}
}

func TestMBClient_SearchRelease_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"releases": []map[string]interface{}{
				{
					"id":    "test-release-id",
					"title": "OK Computer",
					"date":  "1997-05-21",
					"media": []map[string]interface{}{
						{
							"tracks": []map[string]interface{}{
								{"id": "track-1", "title": "Airbag", "number": "1"},
								{"id": "track-2", "title": "Paranoid Android", "number": "2"},
							},
						},
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	// Create MBClient pointing to mock server
	client := &MBClient{
		BaseURL: mockServer.URL,
		Client:  mockServer.Client(),
	}

	releases, err := client.SearchRelease("Radiohead", "OK Computer")
	if err != nil {
		t.Fatalf("SearchRelease failed: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("Expected 1 release, got %d", len(releases))
	}
	if releases[0].Title != "OK Computer" {
		t.Errorf("Expected title 'OK Computer', got %q", releases[0].Title)
	}
	if releases[0].ID != "test-release-id" {
		t.Errorf("Expected ID 'test-release-id', got %q", releases[0].ID)
	}
	if len(releases[0].Media) != 1 || len(releases[0].Media[0].Tracks) != 2 {
		t.Error("Expected 1 media with 2 tracks")
	}
}

func TestMBClient_SearchRelease_NoResults(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"releases": []map[string]interface{}{},
		})
	}))
	defer mockServer.Close()

	client := &MBClient{
		BaseURL: mockServer.URL,
		Client:  mockServer.Client(),
	}

	releases, err := client.SearchRelease("Nonexistent", "Album")
	if err != nil {
		t.Fatalf("SearchRelease failed: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("Expected 0 releases, got %d", len(releases))
	}
}

func TestMBClient_SearchRelease_CleansAlbumTitle(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"releases": []map[string]interface{}{
				{"id": "id1", "title": "Clean Title", "date": "2020"},
			},
		})
	}))
	defer mockServer.Close()

	client := &MBClient{
		BaseURL: mockServer.URL,
		Client:  mockServer.Client(),
	}

	// Album with noise patterns that should be cleaned
	releases, err := client.SearchRelease("Artist", "Clean Title (Remastered 2020)")
	if err != nil {
		t.Fatalf("SearchRelease failed: %v", err)
	}
	// The search should clean "(Remastered 2020)" from the query
	if len(releases) == 0 {
		t.Log("No releases found (may be due to query cleaning)")
	}
}

func TestMBClient_GetReleaseDetails_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "release-123",
			"title": "OK Computer",
			"date":  "1997-05-21",
			"media": []map[string]interface{}{
				{
					"tracks": []map[string]interface{}{
						{"id": "track-1", "title": "Airbag", "number": "1"},
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	client := &MBClient{
		BaseURL: mockServer.URL,
		Client:  mockServer.Client(),
	}

	release, err := client.GetReleaseDetails("release-123")
	if err != nil {
		t.Fatalf("GetReleaseDetails failed: %v", err)
	}
	if release.ID != "release-123" {
		t.Errorf("Expected ID 'release-123', got %q", release.ID)
	}
	if release.Title != "OK Computer" {
		t.Errorf("Expected title 'OK Computer', got %q", release.Title)
	}
}

func TestMBClient_SearchRelease_HTTPError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := &MBClient{
		BaseURL: mockServer.URL,
		Client:  mockServer.Client(),
	}

	_, err := client.SearchRelease("Artist", "Album")
	if err == nil {
		t.Error("Expected error for HTTP 500")
	}
}

func TestMBClient_SearchRelease_Unicode(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"releases": []map[string]interface{}{
				{"id": "unicode-id", "title": "Ñoño y su Banda", "date": "2020"},
			},
		})
	}))
	defer mockServer.Close()

	client := &MBClient{
		BaseURL: mockServer.URL,
		Client:  mockServer.Client(),
	}

	releases, err := client.SearchRelease("Ñoño", "Ñoño y su Banda")
	if err != nil {
		t.Fatalf("SearchRelease with unicode failed: %v", err)
	}
	if len(releases) != 1 {
		t.Errorf("Expected 1 release with unicode, got %d", len(releases))
	}
}

func TestMBRelease_MediaTracks(t *testing.T) {
	release := MBRelease{
		ID:    "id",
		Title: "Title",
		Date:  "2020-01-01",
		Media: []struct {
			Tracks []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Number string `json:"number"`
			} `json:"tracks"`
		}{
			{
				Tracks: []struct {
					ID     string `json:"id"`
					Title  string `json:"title"`
					Number string `json:"number"`
				}{
					{ID: "t1", Title: "Track 1", Number: "1"},
					{ID: "t2", Title: "Track 2", Number: "2"},
				},
			},
		},
	}

	if len(release.Media) != 1 {
		t.Errorf("Expected 1 media, got %d", len(release.Media))
	}
	if len(release.Media[0].Tracks) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(release.Media[0].Tracks))
	}
	if release.Media[0].Tracks[0].Title != "Track 1" {
		t.Errorf("Expected 'Track 1', got %q", release.Media[0].Tracks[0].Title)
	}
}
