package gateway

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

// Payloads con etiquetas para XML y JSON del protocolo.

type artistPayload struct {
	XMLName    xml.Name `xml:"artist" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Name       string   `xml:"name,attr" json:"name"`
	AlbumCount int      `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
}

type artistIndex struct {
	XMLName xml.Name        `xml:"index" json:"-"`
	Name    string          `xml:"name,attr" json:"name"`
	Artists []artistPayload `xml:"artist" json:"artist"`
}

type artistsResp struct {
	XMLName         xml.Name      `xml:"artists" json:"-"`
	IgnoredArticles string        `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	Index           []artistIndex `xml:"index" json:"index"`
}

type albumPayload struct {
	XMLName   xml.Name `xml:"album" json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	Name      string   `xml:"name,attr" json:"name"`
	Artist    string   `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID  string   `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	SongCount int      `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
	Duration  int      `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	Year      int      `xml:"year,attr,omitempty" json:"year,omitempty"`
	CoverArt  string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
}

type songPayload struct {
	XMLName   xml.Name `xml:"song" json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	Title     string   `xml:"title,attr" json:"title"`
	Album     string   `xml:"album,attr,omitempty" json:"album,omitempty"`
	Artist    string   `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	AlbumID   string   `xml:"albumId,attr,omitempty" json:"albumId,omitempty"`
	Track     int      `xml:"track,attr,omitempty" json:"track,omitempty"`
	Year      int      `xml:"year,attr,omitempty" json:"year,omitempty"`
	Duration  int      `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	Size      int64    `xml:"size,attr,omitempty" json:"size,omitempty"`
	Suffix    string   `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	BitRate   int      `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	CoverArt  string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Path      string   `xml:"path,attr,omitempty" json:"path,omitempty"`
}

type artistDetail struct {
	XMLName    xml.Name       `xml:"artist" json:"-"`
	ID         string         `xml:"id,attr" json:"id"`
	Name       string         `xml:"name,attr" json:"name"`
	AlbumCount int            `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
	Albums     []albumPayload `xml:"album" json:"album"`
}

type albumDetail struct {
	XMLName   xml.Name      `xml:"album" json:"-"`
	ID        string        `xml:"id,attr" json:"id"`
	Name      string        `xml:"name,attr" json:"name"`
	Artist    string        `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID  string        `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	SongCount int           `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
	Duration  int           `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	Year      int           `xml:"year,attr,omitempty" json:"year,omitempty"`
	CoverArt  string        `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Songs     []songPayload `xml:"song" json:"song"`
}

type searchResp struct {
	XMLName xml.Name        `xml:"searchResult3" json:"-"`
	Artists []artistPayload `xml:"artist" json:"artist"`
	Albums  []albumPayload  `xml:"album" json:"album"`
	Songs   []songPayload   `xml:"song" json:"song"`
}

type licenseResp struct {
	XMLName xml.Name `xml:"license" json:"-"`
	Valid   bool     `xml:"valid,attr" json:"valid"`
}

type folderPayload struct {
	XMLName xml.Name `xml:"musicFolder" json:"-"`
	ID      string   `xml:"id,attr" json:"id"`
	Name    string   `xml:"name,attr" json:"name"`
}

type foldersResp struct {
	XMLName xml.Name        `xml:"musicFolders" json:"-"`
	Folders []folderPayload `xml:"musicFolder" json:"musicFolder"`
}

func (g *SubsonicGateway) handleGetLicense(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	p := licenseResp{Valid: true}
	writeOK(w, r, "license", p, p)
}

func (g *SubsonicGateway) handleGetMusicFolders(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	p := foldersResp{Folders: []folderPayload{{ID: "1", Name: "Music"}}}
	writeOK(w, r, "musicFolders", p, p)
}

func indexName(name string) string {
	for _, r := range strings.TrimSpace(name) {
		u := unicode.ToUpper(r)
		if u >= 'A' && u <= 'Z' {
			return string(u)
		}
		if u >= '0' && u <= '9' {
			return "#"
		}
		return "#"
	}
	return "#"
}

func (g *SubsonicGateway) handleGetArtists(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	rows, err := g.DB.Conn.Query(`
		SELECT ar.id, ar.name,
			(SELECT COUNT(*) FROM albums WHERE artist_id = ar.id) AS album_count
		FROM artists ar
		WHERE EXISTS (
			SELECT 1 FROM tracks t
			LEFT JOIN albums al ON t.album_id = al.id
			WHERE (al.artist_id = ar.id OR t.artist = ar.name)
			  AND t.status = 'active'
		)
		ORDER BY ar.name`)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	defer rows.Close()

	byIndex := map[string][]artistPayload{}
	order := []string{}
	for rows.Next() {
		var a artistPayload
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount); err != nil {
			continue
		}
		idx := indexName(a.Name)
		if _, seen := byIndex[idx]; !seen {
			order = append(order, idx)
		}
		byIndex[idx] = append(byIndex[idx], a)
	}
	p := artistsResp{IgnoredArticles: "The El La Los Las Le Les"}
	for _, idx := range order {
		p.Index = append(p.Index, artistIndex{Name: idx, Artists: byIndex[idx]})
	}
	if p.Index == nil {
		p.Index = []artistIndex{}
	}
	writeOK(w, r, "artists", p, p)
}

func (g *SubsonicGateway) queryAlbums(artistID string) ([]albumPayload, error) {
	rows, err := g.DB.Conn.Query(`
		SELECT al.id, al.title, COALESCE(ar.name, ''),
			COALESCE((
				SELECT COUNT(*) FROM tracks t
				WHERE t.album_id = al.id AND t.status = 'active'
			), 0),
			COALESCE((
				SELECT SUM(duration) FROM tracks t
				WHERE t.album_id = al.id AND t.status = 'active'
			), 0),
			COALESCE(al.year, 0)
		FROM albums al
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.artist_id = ?
		ORDER BY COALESCE(al.year, 0), al.title`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []albumPayload
	for rows.Next() {
		var a albumPayload
		if err := rows.Scan(&a.ID, &a.Name, &a.Artist, &a.SongCount, &a.Duration, &a.Year); err != nil {
			continue
		}
		a.ArtistID = artistID
		a.CoverArt = a.ID
		out = append(out, a)
	}
	return out, nil
}

func (g *SubsonicGateway) querySongs(where, order string, args ...any) ([]songPayload, error) {
	rows, err := g.DB.Conn.Query(`
		SELECT t.id, t.title, COALESCE(al.title, ''), COALESCE(t.artist, ''),
			t.album_id, COALESCE(t.track_num, 0), COALESCE(t.duration, 0),
			COALESCE(t.size, 0), COALESCE(t.format, ''), COALESCE(t.bit_rate, 0),
			COALESCE(t.path, '')
		FROM tracks t
		LEFT JOIN albums al ON al.id = t.album_id
		`+where+` `+order, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []songPayload
	for rows.Next() {
		var s songPayload
		if err := rows.Scan(&s.ID, &s.Title, &s.Album, &s.Artist, &s.AlbumID,
			&s.Track, &s.Duration, &s.Size, &s.Suffix, &s.BitRate, &s.Path); err != nil {
			continue
		}
		s.CoverArt = s.AlbumID
		out = append(out, s)
	}
	return out, nil
}

func (g *SubsonicGateway) handleGetArtist(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	var a artistPayload
	err := g.DB.Conn.QueryRow(`
		SELECT ar.id, ar.name,
			(SELECT COUNT(*) FROM albums WHERE artist_id = ar.id)
		FROM artists ar WHERE ar.id = ?`, id).Scan(&a.ID, &a.Name, &a.AlbumCount)
	if err != nil {
		writeErr(w, r, errNotFound, "Artista no encontrado.")
		return
	}
	albums, err := g.queryAlbums(id)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	if albums == nil {
		albums = []albumPayload{}
	}
	p := artistDetail{ID: a.ID, Name: a.Name, AlbumCount: a.AlbumCount, Albums: albums}
	writeOK(w, r, "artist", p, p)
}

func (g *SubsonicGateway) handleGetAlbum(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	var a albumPayload
	err := g.DB.Conn.QueryRow(`
		SELECT al.id, al.title, COALESCE(ar.name, ''), al.artist_id,
			COALESCE(al.year, 0)
		FROM albums al
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.id = ?`, id).Scan(&a.ID, &a.Name, &a.Artist, &a.ArtistID, &a.Year)
	if err != nil {
		writeErr(w, r, errNotFound, "Álbum no encontrado.")
		return
	}
	a.CoverArt = a.ID
	songs, err := g.querySongs("WHERE t.album_id = ? AND t.status = 'active'",
		"ORDER BY COALESCE(t.disc_num, 0), COALESCE(t.track_num, 0)", id)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	if songs == nil {
		songs = []songPayload{}
	}
	a.SongCount = len(songs)
	for _, s := range songs {
		a.Duration += s.Duration
	}
	p := albumDetail{
		ID: a.ID, Name: a.Name, Artist: a.Artist, ArtistID: a.ArtistID,
		SongCount: a.SongCount, Duration: a.Duration, Year: a.Year,
		CoverArt: a.CoverArt, Songs: songs,
	}
	writeOK(w, r, "album", p, p)
}

func (g *SubsonicGateway) handleGetSong(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, r, errMissingParam, "Falta el id.")
		return
	}
	songs, err := g.querySongs("WHERE t.id = ? AND t.status = 'active'", "", id)
	if err != nil {
		writeErr(w, r, errGeneric, "Error de base de datos.")
		return
	}
	if len(songs) == 0 {
		writeErr(w, r, errNotFound, "Canción no encontrada.")
		return
	}
	writeOK(w, r, "song", songs[0], songs[0])
}

func clampCount(v string, def int) int {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	if n > 500 {
		return 500
	}
	return n
}

func (g *SubsonicGateway) handleSearch3(w http.ResponseWriter, r *http.Request) {
	if _, ok := g.authenticate(w, r); !ok {
		return
	}
	q := r.URL.Query()
	query := strings.TrimSpace(q.Get("query"))
	if query == "" {
		writeErr(w, r, errMissingParam, "Falta query.")
		return
	}
	like := "%" + query + "%"
	artistCount := clampCount(q.Get("artistCount"), 20)
	albumCount := clampCount(q.Get("albumCount"), 20)
	songCount := clampCount(q.Get("songCount"), 20)

	p := searchResp{}
	rows, err := g.DB.Conn.Query(`
		SELECT ar.id, ar.name,
			(SELECT COUNT(*) FROM albums WHERE artist_id = ar.id)
		FROM artists ar WHERE ar.name LIKE ? ORDER BY ar.name LIMIT ?`, like, artistCount)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a artistPayload
			if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount); err == nil {
				p.Artists = append(p.Artists, a)
			}
		}
	}
	arows, err := g.DB.Conn.Query(`
		SELECT al.id, al.title, COALESCE(ar.name, ''), al.artist_id,
			COALESCE(al.year, 0)
		FROM albums al LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.title LIKE ? ORDER BY al.title LIMIT ?`, like, albumCount)
	if err == nil {
		defer arows.Close()
		for arows.Next() {
			var a albumPayload
			if err := arows.Scan(&a.ID, &a.Name, &a.Artist, &a.ArtistID, &a.Year); err == nil {
				a.CoverArt = a.ID
				p.Albums = append(p.Albums, a)
			}
		}
	}
	songs, err := g.querySongs("WHERE (t.title LIKE ? OR t.artist LIKE ?) AND t.status = 'active'",
		"ORDER BY t.title LIMIT ?", like, like, songCount)
	if err == nil {
		p.Songs = songs
	}
	if p.Artists == nil {
		p.Artists = []artistPayload{}
	}
	if p.Albums == nil {
		p.Albums = []albumPayload{}
	}
	if p.Songs == nil {
		p.Songs = []songPayload{}
	}
	writeOK(w, r, "searchResult3", p, p)
}
