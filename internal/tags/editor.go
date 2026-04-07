package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"sgound/internal/db"
	"log"
)

type Editor struct {
	DB *db.Database
}

func NewEditor(database *db.Database) *Editor {
	return &Editor{DB: database}
}

// UpdateTags escribe metadatos en el archivo físico y sincroniza el título localmente
func (e *Editor) UpdateTags(trackID string, data map[string]interface{}) error {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		return fmt.Errorf("coud not find track id: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp3" {
		if err := updateMP3(path, data); err != nil {
			return fmt.Errorf("failed to rewrite mp3 ID3 tags: %w", err)
		}
	} else if ext == ".flac" {
		if err := updateFLAC(path, data); err != nil {
			return fmt.Errorf("failed to rewrite flac tags: %w", err)
		}
	} else {
		// Log warning but continue with DB update
		fmt.Printf("Physical tag editing not yet supported for format %s, only DB will be updated\n", ext)
	}

	// Sincronizar cambios en la BD
	query := `UPDATE tracks SET 
		title = ?, genre = ?, track_num = ?, disc_num = ?, 
		composer = ?, comment = ? 
		WHERE id = ?`
	
	getString := func(k string) string {
		if v, ok := data[k].(string); ok { return v }
		return ""
	}
	getInt := func(k string) int {
		if v, ok := data[k].(float64); ok { return int(v) }
		if v, ok := data[k].(int); ok { return v }
		return 0
	}

	_, err = e.DB.Conn.Exec(query, 
		getString("title"), getString("genre"), getInt("trackNum"), 
		getInt("discNum"), getString("composer"), getString("comment"), 
		trackID)
	
	if err != nil {
		return err
	}

	// Actualizar año en la tabla albums relacionada
	var albumID string
	e.DB.Conn.QueryRow("SELECT album_id FROM tracks WHERE id = ?", trackID).Scan(&albumID)
	if albumID != "" {
		e.DB.Conn.Exec("UPDATE albums SET year = ? WHERE id = ?", getInt("year"), albumID)
	}

	log.Printf("✓ Etiquetas y año actualizados para el track: %s", trackID)
	return nil
}

// OrganizeFile mueve el archivo físico según el patrón configurado
func (e *Editor) OrganizeFile(trackID string, musicRoot string, pattern string) (string, error) {
	var oldPath, title, artist, album, format, genre string
	var trackNum, year, discNum, bitrate int

	query := `
		SELECT t.path, t.title, a.name, al.title, t.track_num, t.format, t.genre, al.year, t.disc_num, t.bit_rate
		FROM tracks t
		JOIN albums al ON t.album_id = al.id
		JOIN artists a ON al.artist_id = a.id
		WHERE t.id = ?`

	err := e.DB.Conn.QueryRow(query, trackID).Scan(
		&oldPath, &title, &artist, &album, &trackNum, &format, &genre, &year, &discNum, &bitrate,
	)
	if err != nil {
		return "", err
	}

	clean := func(s string) string {
		r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
		return strings.TrimSpace(r.Replace(s))
	}

	// Reemplazar marcadores en el patrón
	newRelPath := pattern
	newRelPath = strings.ReplaceAll(newRelPath, "{artist}", clean(artist))
	newRelPath = strings.ReplaceAll(newRelPath, "{album}", clean(album))
	newRelPath = strings.ReplaceAll(newRelPath, "{title}", clean(title))
	newRelPath = strings.ReplaceAll(newRelPath, "{genre}", clean(genre))
	newRelPath = strings.ReplaceAll(newRelPath, "{format}", strings.ToLower(format))
	newRelPath = strings.ReplaceAll(newRelPath, "{year}", fmt.Sprintf("%d", year))
	newRelPath = strings.ReplaceAll(newRelPath, "{track}", fmt.Sprintf("%02d", trackNum))
	newRelPath = strings.ReplaceAll(newRelPath, "{disc}", fmt.Sprintf("%d", discNum))
	newRelPath = strings.ReplaceAll(newRelPath, "{bitrate}", fmt.Sprintf("%dk", bitrate/1000))

	newPath := filepath.Join(musicRoot, newRelPath+"."+format)
	newDirPath := filepath.Dir(newPath)


	if oldPath == newPath {
		return newPath, nil
	}

	// Crear directorios si no existen
	if err := os.MkdirAll(newDirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Mover archivo
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to move file: %w", err)
	}

	// Actualizar base de datos con la nueva ruta
	_, err = e.DB.Conn.Exec("UPDATE tracks SET path = ? WHERE id = ?", newPath, trackID)
	
	return newPath, err
}

func (e *Editor) UpdateLyrics(trackID string, lyrics string) error {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return updateMP3Lyrics(path, lyrics)
	case ".flac", ".ogg":
		return updateVorbisLyrics(path, lyrics)
	default:
		return fmt.Errorf("edicion de letras no soportada para el formato: %s", ext)
	}
}

func (e *Editor) ReadLyrics(trackID string) (string, error) {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp3" {
		tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
		if err != nil {
			return "", err
		}
		defer tag.Close()
		// ID3v2: USLT frame
		frames := tag.GetFrames(tag.CommonID("Unsynchronised lyrics/text transcription"))
		for _, f := range frames {
			if lf, ok := f.(id3v2.UnsynchronisedLyricsFrame); ok {
				return lf.Lyrics, nil
			}
		}
	} else if ext == ".flac" {
		f, err := flac.ParseFile(path)
		if err != nil {
			return "", err
		}
		for _, block := range f.Meta {
			if block.Type == flac.VorbisComment {
				cm, _ := flacvorbis.ParseFromMetaDataBlock(*block)
				if cm == nil { continue }
				for _, key := range []string{"LYRICS", "UNSYNCEDLYRICS", "TEXT"} {
					vals, _ := cm.Get(key)
					if len(vals) > 0 {
						return vals[0], nil
					}
				}
			}
		}
	}
	return "", nil
}

func updateVorbisLyrics(path string, lyrics string) error {
	f, err := flac.ParseFile(path)
	if err != nil {
		return err
	}

	var cm *flacvorbis.MetaDataBlockVorbisComment
	var cmIdx = -1
	for i, block := range f.Meta {
		if block.Type == flac.VorbisComment {
			cm, _ = flacvorbis.ParseFromMetaDataBlock(*block)
			cmIdx = i
			break
		}
	}

	if cm == nil {
		cm = flacvorbis.New()
	}

	// Eliminar letras viejas si existen y añadir nuevas
	// Convención Vorbis: LYRICS o UNSYNCEDLYRICS
	for _, key := range []string{"LYRICS", "UNSYNCEDLYRICS", "TEXT"} {
		for {
			found := false
			for i, comment := range cm.Comments {
				if strings.ToUpper(strings.Split(comment, "=")[0]) == key {
					cm.Comments = append(cm.Comments[:i], cm.Comments[i+1:]...)
					found = true
					break
				}
			}
			if !found { break }
		}
	}
	cm.Add("LYRICS", lyrics)

	block := cm.Marshal()
	if cmIdx != -1 {
		f.Meta[cmIdx] = &block
	} else {
		f.Meta = append(f.Meta, &block)
	}

	err = f.Save(path)
	if err == nil {
		fmt.Printf("✓ Letras guardadas correctamente en FLAC: %s\n", path)
	}
	return err
}

func updateMP3Lyrics(path string, lyrics string) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
		Encoding:          id3v2.EncodingUTF8,
		Language:          "eng",
		ContentDescriptor: "",
		Lyrics:            lyrics,
	})

	err = tag.Save()
	if err == nil {
		fmt.Printf("✓ Letras guardadas correctamente en MP3: %s\n", path)
	}
	return err
}

func updateMP3(path string, data map[string]interface{}) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	getString := func(k string) string {
		if v, ok := data[k].(string); ok { return v }
		return ""
	}
	getIntStr := func(k string) string {
		if v, ok := data[k].(float64); ok { return fmt.Sprintf("%d", int(v)) }
		if v, ok := data[k].(int); ok { return fmt.Sprintf("%d", v) }
		return ""
	}

	tag.SetTitle(getString("title"))
	tag.SetArtist(getString("artist"))
	tag.SetAlbum(getString("album"))
	tag.SetGenre(getString("genre"))
	
	yearStr := getIntStr("year")
	if yearStr != "" {
		tag.SetYear(yearStr)
	}
	
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, getIntStr("trackNum"))
	tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, getIntStr("discNum"))
	tag.AddTextFrame(tag.CommonID("Composer"), id3v2.EncodingUTF8, getString("composer"))
	
	tag.AddCommentFrame(id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    "eng",
		Description: "",
		Text:        getString("comment"),
	})

	err = tag.Save()
	if err == nil {
		fmt.Printf("✓ Etiquetas guardadas correctamente en MP3: %s\n", path)
	}
	return err
}

func updateFLAC(path string, data map[string]interface{}) error {
	f, err := flac.ParseFile(path)
	if err != nil {
		return err
	}

	var cm *flacvorbis.MetaDataBlockVorbisComment
	var cmIdx = -1
	for i, block := range f.Meta {
		if block.Type == flac.VorbisComment {
			cm, _ = flacvorbis.ParseFromMetaDataBlock(*block)
			cmIdx = i
			break
		}
	}

	if cm == nil {
		cm = flacvorbis.New()
	}

	getString := func(k string) string {
		if v, ok := data[k].(string); ok { return v }
		return ""
	}
	getIntStr := func(k string) string {
		if v, ok := data[k].(float64); ok { return fmt.Sprintf("%d", int(v)) }
		if v, ok := data[k].(int); ok { return fmt.Sprintf("%d", v) }
		return ""
	}

	// Para FLAC/Vorbis, lo mejor es limpiar el tag específico y añadir el nuevo
	// para evitar múltiples entradas del mismo tipo (que Vorbis permite)
	setField := func(field string, value string) {
		if value == "" { return }
		// Eliminar existentes
		var newComments []string
		for _, c := range cm.Comments {
			if !strings.HasPrefix(strings.ToUpper(c), field+"=") {
				newComments = append(newComments, c)
			}
		}
		cm.Comments = newComments
		cm.Add(field, value)
	}

	setField("TITLE", getString("title"))
	setField("ARTIST", getString("artist"))
	setField("ALBUM", getString("album"))
	setField("GENRE", getString("genre"))
	setField("DATE", getIntStr("year"))
	setField("TRACKNUMBER", getIntStr("trackNum"))
	setField("DISCNUMBER", getIntStr("discNum"))
	setField("COMPOSER", getString("composer"))
	setField("COMMENT", getString("comment"))

	block := cm.Marshal()
	if cmIdx != -1 {
		f.Meta[cmIdx] = &block
	} else {
		f.Meta = append(f.Meta, &block)
	}

	err = f.Save(path)
	if err == nil {
		fmt.Printf("✓ Etiquetas guardadas correctamente en FLAC: %s\n", path)
	}
	return err
}
