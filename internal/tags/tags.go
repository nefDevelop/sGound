package tags

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Picture representa una carátula embebida
type Picture struct {
	MIME        string
	Data        []byte
	Description string
}

// Metadata define la interfaz universal para leer y escribir metadatos
type Metadata interface {
	// Lectura
	Title() string
	Artist() string
	Album() string
	AlbumArtist() string
	Year() int
	Track() (int, int) // numero, total
	Disc() (int, int)  // numero, total
	Genre() string
	Composer() string
	Comment() string
	Picture() *Picture
	
	// Escritura
	SetTitle(string)
	SetArtist(string)
	SetAlbum(string)
	SetAlbumArtist(string)
	SetYear(int)
	SetTrack(int, int)
	SetDisc(int, int)
	SetGenre(string)
	SetComposer(string)
	SetComment(string)

	// Persistencia
	Save() error
	Close() error
}

// Open abre un archivo de audio y devuelve el motor de metadatos adecuado
func Open(path string) (Metadata, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return OpenMP3(path)
	case ".flac":
		return OpenFLAC(path)
	case ".m4a", ".mp4":
		return OpenMP4(path)
	case ".ogg", ".opus":
		return OpenOGG(path)
	default:
		return nil, fmt.Errorf("formato no soportado: %s", ext)
	}
}
