package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

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

// GetArtistBiography consulta Wikipedia para obtener un resumen del artista
func (e *ExternalMetadata) GetArtistBiography(artistName string) (string, error) {
	// 1. Buscar la página más probable en Wikipedia
	searchURL := fmt.Sprintf("https://es.wikipedia.org/api/rest_v1/page/summary/%s", url.PathEscape(strings.ReplaceAll(artistName, " ", "_")))
	
	resp, err := http.Get(searchURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Reintento en inglés si no hay en español
		searchURL = fmt.Sprintf("https://en.wikipedia.org/api/rest_v1/page/summary/%s", url.PathEscape(strings.ReplaceAll(artistName, " ", "_")))
		resp, err = http.Get(searchURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			return "No biography available.", nil
		}
		defer resp.Body.Close()
	}

	var result struct {
		Extract string `json:"extract"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Extract, nil
}

// GetArtistImage (Simulado por ahora, se integrará Last.fm después)
func (e *ExternalMetadata) GetArtistImage(artistName string) string {
	// Por ahora devolvemos un placeholder o nada
	return ""
}
