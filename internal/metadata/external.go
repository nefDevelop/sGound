package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"log/slog"
	"time"
	"sgound/internal/config"
)

const LASTFM_API_KEY = "3884e889f074ca68516087d19e078028" // API Key pública para Sgound

type ArtistInfo struct {
	Biography string   `json:"biography"`
	Image     string   `json:"image"`
	Tags      []string `json:"tags"`
	Similar   []string `json:"similar"`
}

type ExternalMetadata struct{
	Config *config.Config
}

func NewExternalMetadata(cfg *config.Config) *ExternalMetadata {
	return &ExternalMetadata{Config: cfg}
}

// GetArtistInfo consulta Wikipedia y Last.fm para obtener info completa
func (e *ExternalMetadata) GetArtistInfo(artistName string) (*ArtistInfo, error) {
	artistName = strings.TrimSpace(artistName)
	artistName = strings.TrimSuffix(artistName, ".")

	slog.Debug("🔍 Buscando info enriquecida", "nombre", artistName, "lang", e.Config.Language)

	// 1. Wikipedia para Bio e Imagen (Prioridad absoluta)
	wikiTitle := strings.ReplaceAll(artistName, " ", "_")
	
	// Idioma configurado
	primaryLang := e.Config.Language
	secondaryLang := "en"
	if primaryLang == "en" {
		secondaryLang = "es"
	}

	info, _ := e.fetchSummary(wikiTitle, primaryLang)
	if info == nil || info.Image == "" {
		wInfo2, _ := e.fetchSummary(wikiTitle, secondaryLang)
		if wInfo2 != nil {
			if info == nil {
				info = wInfo2
			} else if info.Image == "" {
				info.Image = wInfo2.Image
				if info.Biography == "" {
					info.Biography = wInfo2.Biography
				}
			}
		}
	}

	if info == nil {
		info = &ArtistInfo{Biography: "No details found."}
	}

	// 2. Last.fm para Similares y Tags
	e.enrichWithLastFM(artistName, info)

	return info, nil
}

func (e *ExternalMetadata) enrichWithLastFM(name string, info *ArtistInfo) {
	apiURL := fmt.Sprintf("http://ws.audioscrobbler.com/2.0/?method=artist.getinfo&artist=%s&api_key=%s&format=json&lang=%s", 
		url.QueryEscape(name), LASTFM_API_KEY, e.Config.Language)

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
		found := false
		for _, existing := range info.Tags {
			if existing == t.Name { found = true; break }
		}
		if !found {
			info.Tags = append(info.Tags, t.Name)
		}
	}

	// Extraer Similares
	for _, s := range result.Artist.Similar.Artist {
		found := false
		for _, existing := range info.Similar {
			if existing == s.Name { found = true; break }
		}
		if !found {
			info.Similar = append(info.Similar, s.Name)
		}
	}
}

func (e *ExternalMetadata) searchBestTitle(name string) string {
	searchURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&format=json&srlimit=1", url.QueryEscape(name))
	client := &http.Client{Timeout: 5 * time.Second}
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
	client := &http.Client{Timeout: 5 * time.Second}
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
