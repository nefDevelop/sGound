package tags

import (
	"os"
	"strconv"

	"github.com/abema/go-mp4"
)

type mp4Metadata struct {
	path string
	tags map[string]string
}

func OpenMP4(path string) (Metadata, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()

	m := &mp4Metadata{path: path, tags: make(map[string]string)}
	
	// Utilizar la API de lectura de boxes de go-mp4 (basado en ReadHandler)
	_, err = mp4.ReadBoxStructure(f, func(h *mp4.ReadHandle) (interface{}, error) {
		if h.BoxInfo.Type == mp4.BoxTypeIlst() {
			// Encontrado contenedor de metadatos de iTunes
			// Por ahora devolvemos nil para que compile, implementaremos el parsing granular luego
		}
		return nil, nil
	})

	return m, nil
}

func (m *mp4Metadata) Title() string       { return m.tags["©nam"] }
func (m *mp4Metadata) Artist() string      { return m.tags["©ART"] }
func (m *mp4Metadata) Album() string       { return m.tags["©alb"] }
func (m *mp4Metadata) Genre() string       { return m.tags["©gen"] }
func (m *mp4Metadata) Composer() string    { return m.tags["©wrt"] }
func (m *mp4Metadata) Comment() string     { return m.tags["©cmt"] }
func (m *mp4Metadata) AlbumArtist() string { return m.tags["aART"] }
func (m *mp4Metadata) Year() int           { y, _ := strconv.Atoi(m.tags["©day"]); return y }

func (m *mp4Metadata) Track() (int, int) { return 0, 0 }
func (m *mp4Metadata) Disc() (int, int)  { return 0, 0 }

// Escritura
func (m *mp4Metadata) SetTitle(s string)       { m.tags["©nam"] = s }
func (m *mp4Metadata) SetArtist(s string)      { m.tags["©ART"] = s }
func (m *mp4Metadata) SetAlbum(s string)       { m.tags["©alb"] = s }
func (m *mp4Metadata) SetGenre(s string)       { m.tags["©gen"] = s }
func (m *mp4Metadata) SetComposer(s string)    { m.tags["©wrt"] = s }
func (m *mp4Metadata) SetComment(s string)     { m.tags["©cmt"] = s }
func (m *mp4Metadata) SetAlbumArtist(s string) { m.tags["aART"] = s }
func (m *mp4Metadata) SetYear(y int)           { m.tags["©day"] = strconv.Itoa(y) }
func (m *mp4Metadata) SetTrack(n, t int)       {}
func (m *mp4Metadata) SetDisc(n, t int)        {}

func (m *mp4Metadata) Save() error  { return nil }
func (m *mp4Metadata) Close() error { return nil }
