package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	query := fmt.Sprintf("artist:\"%s\" AND release:\"%s\"", artist, album)
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
