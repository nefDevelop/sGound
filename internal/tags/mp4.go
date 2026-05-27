package tags

import (
	"os"
	"strconv"
	"github.com/dhowden/tag"
)

type mp4Metadata struct {
	path   string
	meta   tag.Metadata
	file   *os.File
	fields map[string]string
}

func OpenMP4(path string) (Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	m, err := tag.ReadFrom(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &mp4Metadata{
		path:   path,
		meta:   m,
		file:   f,
		fields: make(map[string]string),
	}, nil
}

func (m *mp4Metadata) Title() string       { if val, ok := m.fields["title"]; ok { return val }; return m.meta.Title() }
func (m *mp4Metadata) Artist() string      { if val, ok := m.fields["artist"]; ok { return val }; return m.meta.Artist() }
func (m *mp4Metadata) Album() string       { if val, ok := m.fields["album"]; ok { return val }; return m.meta.Album() }
func (m *mp4Metadata) Genre() string       { if val, ok := m.fields["genre"]; ok { return val }; return m.meta.Genre() }
func (m *mp4Metadata) Composer() string    { if val, ok := m.fields["composer"]; ok { return val }; return m.meta.Composer() }
func (m *mp4Metadata) Comment() string     { if val, ok := m.fields["comment"]; ok { return val }; return m.meta.Comment() }
func (m *mp4Metadata) AlbumArtist() string { if val, ok := m.fields["albumartist"]; ok { return val }; return m.meta.AlbumArtist() }
func (m *mp4Metadata) Year() int {
	if val, ok := m.fields["year"]; ok {
		y, _ := strconv.Atoi(val)
		return y
	}
	return m.meta.Year()
}
func (m *mp4Metadata) Track() (int, int) {
	if val, ok := m.fields["track"]; ok {
		t, _ := strconv.Atoi(val)
		return t, 0
	}
	return m.meta.Track()
}
func (m *mp4Metadata) Disc() (int, int) {
	if val, ok := m.fields["disc"]; ok {
		d, _ := strconv.Atoi(val)
		return d, 0
	}
	return m.meta.Disc()
}

func (m *mp4Metadata) Picture() *Picture {
	pic := m.meta.Picture()
	if pic == nil {
		return nil
	}
	return &Picture{
		MIME:        pic.MIMEType,
		Data:        pic.Data,
		Description: pic.Description,
	}
}

// Escritura
func (m *mp4Metadata) SetTitle(s string)       { m.fields["title"] = s }
func (m *mp4Metadata) SetArtist(s string)      { m.fields["artist"] = s }
func (m *mp4Metadata) SetAlbum(s string)       { m.fields["album"] = s }
func (m *mp4Metadata) SetGenre(s string)       { m.fields["genre"] = s }
func (m *mp4Metadata) SetComposer(s string)    { m.fields["composer"] = s }
func (m *mp4Metadata) SetComment(s string)     { m.fields["comment"] = s }
func (m *mp4Metadata) SetAlbumArtist(s string) { m.fields["albumartist"] = s }
func (m *mp4Metadata) SetYear(y int)           { m.fields["year"] = strconv.Itoa(y) }
func (m *mp4Metadata) SetTrack(n, t int)       { m.fields["track"] = strconv.Itoa(n) }
func (m *mp4Metadata) SetDisc(n, t int)        { m.fields["disc"] = strconv.Itoa(n) }

func (m *mp4Metadata) Save() error {
	// TODO: dhowden/tag solo lee. Para escritura en mp4 se requiere otra librería.
	return nil
}

func (m *mp4Metadata) Close() error {
	if m.file != nil {
		return m.file.Close()
	}
	return nil
}
