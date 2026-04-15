package models

// Basic models for the application.

type Artist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AlbumCount int    `json:"albumCount"`
	ImageURL   string `json:"imageUrl,omitempty"`
}

type Album struct {
	ID       string `json:"id"`
	ArtistID string `json:"artistId"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Artist   string `json:"artist,omitempty"`
	CoverID  string `json:"coverId,omitempty"`
}

type Track struct {
	ID         string `json:"id"`
	AlbumID    string `json:"albumId"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	TrackNum   int    `json:"trackNum"`
	TrackTotal int    `json:"trackTotal,omitempty"`
	DiscNum    int    `json:"discNum,omitempty"`
	DiscTotal  int    `json:"discTotal,omitempty"`
	Path       string `json:"path"`
	Duration   int    `json:"duration"`
	Genre      string `json:"genre"`
	Size       int64  `json:"size"`
	Format     string `json:"format"`
	BitRate    int    `json:"bit_rate"`
	SampleRate int    `json:"sample_rate"`
	Status     string  `json:"status"`
	Rating     int     `json:"rating"`
	Instrumental bool  `json:"instrumental"`
	ReplayGain float64 `json:"replayGain"`
	FileHash   string  `json:"fileHash"`
	CreatedAt  string `json:"createdAt"`
}

type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsSmart     bool   `json:"isSmart"`
	RuleSQL     string `json:"ruleSql,omitempty"`
	CreatedAt   string `json:"createdAt"`
	TrackCount  int    `json:"trackCount"`
}

