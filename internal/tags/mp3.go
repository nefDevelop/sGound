package tags

import (
	"fmt"
	"strconv"

	"github.com/bogem/id3v2/v2"
)

type mp3Metadata struct {
	tag  *id3v2.Tag
	path string
}

func OpenMP3(path string) (Metadata, error) {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return nil, err
	}
	return &mp3Metadata{tag: tag, path: path}, nil
}

func (m *mp3Metadata) Title() string       { return m.tag.Title() }
func (m *mp3Metadata) Artist() string      { return m.tag.Artist() }
func (m *mp3Metadata) Album() string       { return m.tag.Album() }
func (m *mp3Metadata) Genre() string       { return m.tag.Genre() }
func (m *mp3Metadata) Composer() string    { return m.tag.GetTextFrame("TCOM").Text }
func (m *mp3Metadata) Comment() string     { 
	comments := m.tag.GetFrames("COMM")
	if len(comments) > 0 {
		if cf, ok := comments[0].(id3v2.CommentFrame); ok {
			return cf.Text
		}
	}
	return "" 
}

func (m *mp3Metadata) AlbumArtist() string { return m.tag.GetTextFrame("TPE2").Text }
func (m *mp3Metadata) Year() int           { y, _ := strconv.Atoi(m.tag.Year()); return y }

func (m *mp3Metadata) Track() (int, int) {
	tStr := m.tag.GetTextFrame("TRCK").Text
	if tStr == "" { return 0, 0 }
	var n, total int
	fmt.Sscanf(tStr, "%d/%d", &n, &total)
	if total == 0 { fmt.Sscanf(tStr, "%d", &n) }
	return n, total
}

func (m *mp3Metadata) Disc() (int, int) {
	tStr := m.tag.GetTextFrame("TPOS").Text
	if tStr == "" { return 0, 0 }
	var n, total int
	fmt.Sscanf(tStr, "%d/%d", &n, &total)
	if total == 0 { fmt.Sscanf(tStr, "%d", &n) }
	return n, total
}

// Escritura
func (m *mp3Metadata) SetTitle(s string)       { m.tag.SetTitle(s) }
func (m *mp3Metadata) SetArtist(s string)      { m.tag.SetArtist(s) }
func (m *mp3Metadata) SetAlbum(s string)       { m.tag.SetAlbum(s) }
func (m *mp3Metadata) SetGenre(s string)       { m.tag.SetGenre(s) }
func (m *mp3Metadata) SetComposer(s string)    { m.tag.AddTextFrame("TCOM", m.tag.DefaultEncoding(), s) }
func (m *mp3Metadata) SetComment(s string)     { m.tag.AddCommentFrame(id3v2.CommentFrame{Encoding: m.tag.DefaultEncoding(), Text: s}) }
func (m *mp3Metadata) SetAlbumArtist(s string) { m.tag.AddTextFrame("TPE2", m.tag.DefaultEncoding(), s) }
func (m *mp3Metadata) SetYear(year int)        { m.tag.SetYear(strconv.Itoa(year)) }

func (m *mp3Metadata) SetTrack(n, total int) {
	val := strconv.Itoa(n)
	if total > 0 { val = fmt.Sprintf("%d/%d", n, total) }
	m.tag.AddTextFrame("TRCK", m.tag.DefaultEncoding(), val)
}

func (m *mp3Metadata) SetDisc(n, total int) {
	val := strconv.Itoa(n)
	if total > 0 { val = fmt.Sprintf("%d/%d", n, total) }
	m.tag.AddTextFrame("TPOS", m.tag.DefaultEncoding(), val)
}

func (m *mp3Metadata) Save() error  { return m.tag.Save() }
func (m *mp3Metadata) Close() error { return m.tag.Close() }
