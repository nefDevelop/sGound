package tags

import (
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"
	"strings"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

type flacMetadata struct {
	file   *flac.File
	path   string
	vorbis *flacvorbis.MetaDataBlockVorbisComment
}

// ParseFLAC es un helper para editor.go que devuelve el archivo FLAC decodificado
func ParseFLAC(path string) (*flac.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return flac.ParseMetadata(f)
}

func OpenFLAC(path string) (Metadata, error) {
	decoded, err := ParseFLAC(path)
	if err != nil {
		return nil, err
	}

	m := &flacMetadata{file: decoded, path: path}
	for _, block := range decoded.Meta {
		if block.Type == flac.VorbisComment {
			if vc, err := flacvorbis.ParseFromMetaDataBlock(*block); err == nil {
				m.vorbis = vc
			}
		}
	}

	if m.vorbis == nil {
		m.vorbis = flacvorbis.New()
	}

	return m, nil
}

func (f *flacMetadata) getFirst(key string) string {
	val, _ := f.vorbis.Get(key)
	if len(val) > 0 {
		return val[0]
	}
	return ""
}

func (f *flacMetadata) Title() string       { return f.getFirst("TITLE") }
func (f *flacMetadata) Artist() string      { return f.getFirst("ARTIST") }
func (f *flacMetadata) Album() string       { return f.getFirst("ALBUM") }
func (f *flacMetadata) Genre() string       { return f.getFirst("GENRE") }
func (f *flacMetadata) Composer() string    { return f.getFirst("COMPOSER") }
func (f *flacMetadata) Comment() string     { return f.getFirst("COMMENT") }
func (f *flacMetadata) AlbumArtist() string { return f.getFirst("ALBUMARTIST") }

func (f *flacMetadata) Year() int {
	y, _ := strconv.Atoi(f.getFirst("DATE"))
	if y == 0 {
		y, _ = strconv.Atoi(f.getFirst("YEAR"))
	}
	return y
}

func (f *flacMetadata) Track() (int, int) {
	n, _ := strconv.Atoi(f.getFirst("TRACKNUMBER"))
	t, _ := strconv.Atoi(f.getFirst("TRACKTOTAL"))
	return n, t
}

func (f *flacMetadata) Disc() (int, int) {
	n, _ := strconv.Atoi(f.getFirst("DISCNUMBER"))
	t, _ := strconv.Atoi(f.getFirst("DISCTOTAL"))
	return n, t
}

func (f *flacMetadata) Picture() *Picture {
	for _, block := range f.file.Meta {
		if block.Type == flac.Picture {
			if pic, err := flacpicture.ParseFromMetaDataBlock(*block); err == nil {
				return &Picture{
					MIME:        pic.MIME,
					Data:        pic.ImageData,
					Description: pic.Description,
				}
			}
		}
	}
	return nil
}

// setTag añade o actualiza un comentario Vorbis
func (f *flacMetadata) setTag(key string, value string) {
	prefix := strings.ToUpper(key) + "="
	newComment := prefix + value

	for i, comment := range f.vorbis.Comments {
		if strings.HasPrefix(strings.ToUpper(comment), prefix) {
			f.vorbis.Comments[i] = newComment
			return
		}
	}
	f.vorbis.Comments = append(f.vorbis.Comments, newComment)
}

// Implementación de setters (Vorbis Comment)
func (f *flacMetadata) SetTitle(v string)       { f.setTag("TITLE", v) }
func (f *flacMetadata) SetArtist(v string)      { f.setTag("ARTIST", v) }
func (f *flacMetadata) SetAlbum(v string)       { f.setTag("ALBUM", v) }
func (f *flacMetadata) SetGenre(v string)       { f.setTag("GENRE", v) }
func (f *flacMetadata) SetYear(v int)           { f.setTag("DATE", strconv.Itoa(v)) }
func (f *flacMetadata) SetComposer(v string)    { f.setTag("COMPOSER", v) }
func (f *flacMetadata) SetAlbumArtist(v string) { f.setTag("ALBUMARTIST", v) }
func (f *flacMetadata) SetComment(v string)     { f.setTag("COMMENT", v) }

func (f *flacMetadata) SetTrack(n, total int) {
	f.setTag("TRACKNUMBER", strconv.Itoa(n))
	if total > 0 {
		f.setTag("TRACKTOTAL", strconv.Itoa(total))
	}
}

func (f *flacMetadata) SetDisc(n, total int) {
	f.setTag("DISCNUMBER", strconv.Itoa(n))
	if total > 0 {
		f.setTag("DISCTOTAL", strconv.Itoa(total))
	}
}

func (f *flacMetadata) Save() error {
	newBlock := f.vorbis.Marshal()
	found := false
	for i, block := range f.file.Meta {
		if block.Type == flac.VorbisComment {
			f.file.Meta[i] = &newBlock
			found = true
			break
		}
	}
	if !found {
		f.file.Meta = append(f.file.Meta, &newBlock)
	}

	return f.file.Save(f.path)
}

func (f *flacMetadata) Close() error {
	f.file = nil
	f.vorbis = nil
	return nil
}
