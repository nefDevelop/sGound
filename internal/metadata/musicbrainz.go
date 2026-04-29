package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type MBClient struct {
	BaseURL string
	Client  *http.Client
}

func NewMBClient() *MBClient {
	return &MBClient{
		BaseURL: "https://musicbrainz.org/ws/2",
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type MBRelease struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
	Media []struct {
		Tracks []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Number string `json:"number"`
		} `json:"tracks"`
	} `json:"media"`
}

func (m *MBClient) SearchRelease(artist, album string) ([]MBRelease, error) {
	// 1. Limpiar el título del álbum de "ruido" común (Bootleg, Expanded, etc)
	cleanAlbum := album
	noiseRegex := []string{
		`\(Expanded.*\)`, `\(Bootleg.*\)`, `\(Remastered.*\)`, `\(Deluxe.*\)`,
		`\[Expanded.*\]`, `\[Bootleg.*\]`, `\[Remastered.*\]`, `\[Deluxe.*\]`,
		`\(by .*\)`, 
	}
	
	for _, pattern := range noiseRegex {
		re := regexp.MustCompile("(?i)" + pattern)
		cleanAlbum = re.ReplaceAllString(cleanAlbum, "")
	}
	cleanAlbum = strings.TrimSpace(cleanAlbum)
	
	// Limpieza de caracteres extraños al final
	cleanAlbum = strings.TrimFunc(cleanAlbum, func(r rune) bool {
		return r < 32 || r > 126 // Solo ASCII imprimible básico para la búsqueda inicial
	})

	// Ejecutar búsqueda estricta primero
	releases, err := m.doSearch(artist, cleanAlbum, true)
	if err == nil && len(releases) > 0 {
		return releases, nil
	}

	// 2. Si falla, intentar limpiar más agresivamente (quitar todo lo que esté entre paréntesis)
	aggressiveAlbum := cleanAlbum
	if openParen := strings.Index(aggressiveAlbum, "("); openParen > 0 {
		aggressiveAlbum = strings.TrimSpace(aggressiveAlbum[:openParen])
	}

	if aggressiveAlbum != cleanAlbum {
		releases, err = m.doSearch(artist, aggressiveAlbum, false)
		if err == nil && len(releases) > 0 {
			return releases, nil
		}
	}

	// 3. Fallback final: búsqueda difusa sin comillas
	return m.doSearch(artist, aggressiveAlbum, false)
}

func (m *MBClient) doSearch(artist, album string, strict bool) ([]MBRelease, error) {
	var parts []string
	if artist != "" {
		parts = append(parts, fmt.Sprintf("artist:\"%s\"", artist))
	}
	if album != "" {
		if strict {
			parts = append(parts, fmt.Sprintf("release:\"%s\"", album))
		} else {
			parts = append(parts, fmt.Sprintf("release:%s", album))
		}
	}
	
	query := strings.Join(parts, " AND ")
	u := fmt.Sprintf("%s/release?query=%s&fmt=json", m.BaseURL, url.QueryEscape(query))
	
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "sGound/1.0 ( https://github.com/mmmbuto/sgound )")

	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MusicBrainz API error: %d", resp.StatusCode)
	}

	var result struct {
		Releases []MBRelease `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Releases, nil
}

func (m *MBClient) GetReleaseDetails(releaseID string) (*MBRelease, error) {
	u := fmt.Sprintf("%s/release/%s?inc=recordings&fmt=json", m.BaseURL, releaseID)
	
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "sGound/1.0 ( https://github.com/mmmbuto/sgound )")

	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release MBRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
