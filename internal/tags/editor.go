package tags

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"sgound/internal/db"
)

type Editor struct {
	DB *db.Database
}

func NewEditor(database *db.Database) *Editor {
	return &Editor{DB: database}
}

// UpdateTags escribe metadatos en el archivo físico y sincroniza el título localmente
func (e *Editor) UpdateTags(trackID string, newTitle, newArtist, newAlbum string) error {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		return fmt.Errorf("coud not find track id: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp3" {
		if err := updateMP3(path, newTitle, newArtist, newAlbum); err != nil {
			return fmt.Errorf("faled to rewrite mp3 ID3 tags: %w", err)
		}
	} else {
		// Para el objetivo de MVP, nos limitamos solo a .mp3 físicos
		return fmt.Errorf("edicion fisica solo soportada para .mp3 temporalmente")
	}

	// Sincronizar de forma basica los cambios a la BD en tracks
	if newTitle != "" {
		_, err = e.DB.Conn.Exec("UPDATE tracks SET title = ? WHERE id = ?", newTitle, trackID)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateMP3(path, title, artist, album string) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	if title != "" {
		tag.SetTitle(title)
	}
	if artist != "" {
		tag.SetArtist(artist)
	}
	if album != "" {
		tag.SetAlbum(album)
	}

	return tag.Save()
}
