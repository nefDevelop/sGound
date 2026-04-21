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
	// ... (rest of OpenMP4 remains same)
	return m, nil
}

// ... (existing methods)

func (m *mp4Metadata) Picture() *Picture { return nil }

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
