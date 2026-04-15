package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"log/slog"
	"time"
)

const LASTFM_API_KEY = "3884e889f074ca68516087d19e078028" // API Key pública para Sgound

type ArtistInfo struct {
	Biography string   `json:"biography"`
	Image     string   `json:"image"`
	Tags      []string `json:"tags"`
	Similar   []string `json:"similar"`
}

type ExternalMetadata struct{}

func NewExternalMetadata() *ExternalMetadata {
	return &ExternalMetadata{}
}

// GetArtistInfo consulta Wikipedia y Last.fm para obtener info completa
func (e *ExternalMetadata) GetArtistInfo(artistName string) (*ArtistInfo, error) {
	artistName = strings.TrimSpace(artistName)
	artistName = strings.TrimSuffix(artistName, ".")

	slog.Info("🔍 Buscando info enriquecida", "nombre", artistName)

	// 1. Wikipedia para Bio e Imagen
	wikiTitle := strings.ReplaceAll(artistName, " ", "_")
	info, _ := e.fetchSummary(wikiTitle, "en")
	if info == nil || info.Image == "" {
		infoES, _ := e.fetchSummary(wikiTitle, "es")
		if infoES != nil && (info == nil || infoES.Image != "") {
			info = infoES
		}
	}

	// 2. Si falla Wikipedia directo, buscar título canónico
	if info == nil || info.Biography == "" {
		bestTitle := e.searchBestTitle(artistName)
		if bestTitle != "" {
			info, _ = e.fetchSummary(bestTitle, "en")
		}
	}

	if info == nil {
		info = &ArtistInfo{Biography: "No details found."}
	}

	// 3. Last.fm para Similares y Tags
	e.enrichWithLastFM(artistName, info)

	return info, nil
}

func (e *ExternalMetadata) enrichWithLastFM(name string, info *ArtistInfo) {
	apiURL := fmt.Sprintf("http://ws.audioscrobbler.com/2.0/?method=artist.getinfo&artist=%s&api_key=%s&format=json", 
		url.QueryEscape(name), LASTFM_API_KEY)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		slog.Warn("⚠️ Fallo al conectar con Last.fm", "err", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Artist struct {
			Tags struct {
				Tag []struct {
					Name string `json:"name"`
				} `json:"tag"`
			} `json:"tags"`
			Similar struct {
				Artist []struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"similar"`
		} `json:"artist"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	// Extraer Tags
	for _, t := range result.Artist.Tags.Tag {
		info.Tags = append(info.Tags, t.Name)
	}

	// Extraer Similares
	for _, s := range result.Artist.Similar.Artist {
		info.Similar = append(info.Similar, s.Name)
	}
}

func (e *ExternalMetadata) searchBestTitle(name string) string {
	searchURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&format=json&srlimit=1", url.QueryEscape(name))
	client := &http.Client{}
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "SgoundMusicPlayer/1.0 (admin@example.com)")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Query.Search) == 0 {
		return ""
	}

	return strings.ReplaceAll(result.Query.Search[0].Title, " ", "_")
}

func (e *ExternalMetadata) fetchSummary(title, lang string) (*ArtistInfo, error) {
	searchURL := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s", lang, title)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "SgoundMusicPlayer/1.0 (admin@example.com)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Extract       string `json:"extract"`
		OriginalImage struct {
			Source string `json:"source"`
		} `json:"originalimage"`
		Thumbnail struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	info := &ArtistInfo{Biography: result.Extract}
	if result.OriginalImage.Source != "" {
		info.Image = result.OriginalImage.Source
	} else if result.Thumbnail.Source != "" {
		info.Image = result.Thumbnail.Source
	}
	return info, nil
}
