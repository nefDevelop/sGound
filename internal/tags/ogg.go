package tags

import (
	"os"
	"strconv"
	"strings"

	"github.com/jfreymuth/oggvorbis"
)

type oggMetadata struct {
	path     string
	comments map[string]string
}

func OpenOGG(path string) (Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := oggvorbis.NewReader(f)
	if err != nil {
		return nil, err
	}

	m := &oggMetadata{path: path, comments: make(map[string]string)}
	for _, comment := range r.CommentList() {
		parts := strings.SplitN(comment, "=", 2)
		if len(parts) == 2 {
			m.comments[strings.ToUpper(parts[0])] = parts[1]
		}
	}

	return m, nil
}

func (m *oggMetadata) Title() string       { return m.comments["TITLE"] }
func (m *oggMetadata) Artist() string      { return m.comments["ARTIST"] }
func (m *oggMetadata) Album() string       { return m.comments["ALBUM"] }
func (m *oggMetadata) Genre() string       { return m.comments["GENRE"] }
func (m *oggMetadata) Composer() string    { return m.comments["COMPOSER"] }
func (m *oggMetadata) Comment() string     { return m.comments["COMMENT"] }
func (m *oggMetadata) AlbumArtist() string { return m.comments["ALBUMARTIST"] }
func (m *oggMetadata) Year() int           { y, _ := strconv.Atoi(m.comments["DATE"]); return y }

func (m *oggMetadata) Track() (int, int) {
	n, _ := strconv.Atoi(m.comments["TRACKNUMBER"])
	t, _ := strconv.Atoi(m.comments["TRACKTOTAL"])
	return n, t
}

func (m *oggMetadata) Disc() (int, int) {
	n, _ := strconv.Atoi(m.comments["DISCNUMBER"])
	t, _ := strconv.Atoi(m.comments["DISCTOTAL"])
	return n, t
}

func (m *oggMetadata) Picture() *Picture {
	// OGG Vorbis usa METADATA_BLOCK_PICTURE en base64
	// Por ahora devolvemos nil, implementaremos el base64 decode luego si es necesario
	return nil
}

// Escritura (OGG suele ser solo lectura en muchos reproductores Go)
func (m *oggMetadata) SetTitle(s string)       { m.comments["TITLE"] = s }
func (m *oggMetadata) SetArtist(s string)      { m.comments["ARTIST"] = s }
func (m *oggMetadata) SetAlbum(s string)       { m.comments["ALBUM"] = s }
func (m *oggMetadata) SetGenre(s string)       { m.comments["GENRE"] = s }
func (m *oggMetadata) SetComposer(s string)    { m.comments["COMPOSER"] = s }
func (m *oggMetadata) SetComment(s string)     { m.comments["COMMENT"] = s }
func (m *oggMetadata) SetAlbumArtist(s string) { m.comments["ALBUMARTIST"] = s }
func (m *oggMetadata) SetYear(y int)           { m.comments["DATE"] = strconv.Itoa(y) }
func (m *oggMetadata) SetTrack(n, t int)       {}
func (m *oggMetadata) SetDisc(n, t int)        {}

func (m *oggMetadata) Save() error  { return nil }
func (m *oggMetadata) Close() error { return nil }
