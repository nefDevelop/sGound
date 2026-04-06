package models

// Basic models for the application.

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID       string `json:"id"`
	ArtistID string `json:"artistId"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
}

type Track struct {
	ID       string `json:"id"`
	AlbumID  string `json:"albumId"`
	Title    string `json:"title"`
	TrackNum int    `json:"trackNum"`
	Path     string `json:"-"`
	Duration int    `json:"duration"`
}
