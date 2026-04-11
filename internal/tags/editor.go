package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"crypto/rand"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"github.com/go-flac/flacpicture"
	"sgound/internal/db"
	"log"
)

// GenerateUUID genera un ID único para cada track
func GenerateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ReadPersistentID lee el SGOUND_ID del archivo físico
func ReadPersistentID(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp3" {
		tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
		if err != nil { return "" }
		defer tag.Close()
		frames := tag.GetFrames(tag.CommonID("User defined text information"))
		for _, f := range frames {
			if tf, ok := f.(id3v2.UserDefinedTextFrame); ok && tf.Description == "SGOUND_ID" {
				return tf.Value
			}
		}
	} else if ext == ".flac" {
		f, err := flac.ParseFile(path)
		if err != nil { return "" }
		for _, block := range f.Meta {
			if block.Type == flac.VorbisComment {
				cm, _ := flacvorbis.ParseFromMetaDataBlock(*block)
				if cm == nil { continue }
				vals, _ := cm.Get("SGOUND_ID")
				if len(vals) > 0 { return vals[0] }
			}
		}
	}
	return ""
}

// WritePersistentID escribe el SGOUND_ID en el archivo físico
func WritePersistentID(path string, id string) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp3" {
		tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
		if err != nil { return err }
		defer tag.Close()
		tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
			Encoding:    id3v2.EncodingUTF8,
			Description: "SGOUND_ID",
			Value:       id,
		})
		return tag.Save()
	} else if ext == ".flac" {
		f, err := flac.ParseFile(path)
		if err != nil { return err }
		var cm *flacvorbis.MetaDataBlockVorbisComment
		var cmIdx = -1
		for i, block := range f.Meta {
			if block.Type == flac.VorbisComment {
				cm, _ = flacvorbis.ParseFromMetaDataBlock(*block)
				cmIdx = i
				break
			}
		}
		if cm == nil { cm = flacvorbis.New() }
		cm.Comments = append(cm.Comments, "SGOUND_ID="+id)
		block := cm.Marshal()
		if cmIdx != -1 { f.Meta[cmIdx] = &block } else { f.Meta = append(f.Meta, &block) }
		return f.Save(path)
	}
	return nil
}

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
		title = ?, genre = ?, track_num = ?, track_total = ?, disc_num = ?, disc_total = ?,
		composer = ?, comment = ?, rating = ?, instrumental = ?
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
	getBoolInt := func(k string) int {
		if v, ok := data[k].(bool); ok && v { return 1 }
		return 0
	}

	_, err = e.DB.Conn.Exec(query, 
		getString("title"), getString("genre"), getInt("trackNum"), getInt("trackTotal"),
		getInt("discNum"), getInt("discTotal"), getString("composer"), getString("comment"), 
		getInt("rating"), getBoolInt("instrumental"), trackID)
	
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
		SELECT 
			t.path, 
			COALESCE(t.title, 'Unknown Title'), 
			COALESCE(a.name, 'Unknown Artist'), 
			COALESCE(al.title, 'Unknown Album'), 
			COALESCE(t.track_num, 0), 
			COALESCE(t.format, 'mp3'), 
			COALESCE(t.genre, 'Unknown'), 
			COALESCE(al.year, 0), 
			COALESCE(t.disc_num, 0), 
			COALESCE(t.bit_rate, 0)
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

	// Valores por defecto para tags vacíos
	if artist == "" { artist = "Unknown Artist" }
	if album == "" { album = "Unknown Album" }
	if genre == "" { genre = "Unknown" }
	yearStr := fmt.Sprintf("%d", year)
	if year == 0 { yearStr = "Unknown Year" }

	// Reemplazar marcadores en el patrón
	newRelPath := pattern
	newRelPath = strings.ReplaceAll(newRelPath, "{artist}", clean(artist))
	newRelPath = strings.ReplaceAll(newRelPath, "{album}", clean(album))
	newRelPath = strings.ReplaceAll(newRelPath, "{title}", clean(title))
	newRelPath = strings.ReplaceAll(newRelPath, "{genre}", clean(genre))
	newRelPath = strings.ReplaceAll(newRelPath, "{format}", strings.ToLower(format))
	newRelPath = strings.ReplaceAll(newRelPath, "{year}", yearStr)
	newRelPath = strings.ReplaceAll(newRelPath, "{track}", fmt.Sprintf("%02d", trackNum))
	newRelPath = strings.ReplaceAll(newRelPath, "{disc}", fmt.Sprintf("%d", discNum))
	newRelPath = strings.ReplaceAll(newRelPath, "{bitrate}", fmt.Sprintf("%dk", bitrate/1000))

	newPath := filepath.Join(musicRoot, newRelPath+"."+format)
	
	// Prevenir colisiones: si el archivo existe y no es el mismo, añadir sufijo (1), (2)...
	if _, err := os.Stat(newPath); err == nil && newPath != oldPath {
		ext := "." + format
		base := strings.TrimSuffix(newPath, ext)
		counter := 1
		for {
			candidate := fmt.Sprintf("%s (%d)%s", base, counter, ext)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				newPath = candidate
				break
			}
			counter++
		}
		log.Printf("[Organize] Collision detected for %s, renaming to %s", trackID, filepath.Base(newPath))
	}

	newDirPath := filepath.Dir(newPath)
	oldDirPath := filepath.Dir(oldPath)

	if oldPath == newPath {
		return newPath, nil
	}

	// Crear directorios si no existen
	if err := os.MkdirAll(newDirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Mover archivos adjuntos antes de mover el principal (covers, lyrics, etc)
	moveSidecars(oldPath, newPath)

	// Mover archivo principal
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to move file: %w", err)
	}

	// Limpieza de carpetas vacías (subiendo por el árbol)
	go func(dir string, root string) {
		for dir != root && dir != "." && dir != "/" {
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) > 0 {
				break // Hay archivos o error, paramos limpieza
			}
			os.Remove(dir)
			dir = filepath.Dir(dir)
		}
	}(oldDirPath, musicRoot)

	// Actualizar base de datos con la nueva ruta
	_, err = e.DB.Conn.Exec("UPDATE tracks SET path = ? WHERE id = ?", newPath, trackID)
	
	return newPath, err
}

// moveSidecars busca y mueve archivos relacionados (letras, carátulas) al mover un track
func moveSidecars(oldPath, newPath string) {
	oldDir := filepath.Dir(oldPath)
	newDir := filepath.Dir(newPath)
	
	oldBase := strings.TrimSuffix(filepath.Base(oldPath), filepath.Ext(oldPath))
	newBase := strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath))

	entries, err := os.ReadDir(oldDir)
	if err != nil { return }

	commonSidecars := []string{"cover", "folder", "front", "album", "artwork"}

	for _, e := range entries {
		if e.IsDir() { continue }
		name := e.Name()
		lowName := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(name))

		// 1. Letras o archivos con el mismo nombre que el audio (ej: song.lrc, song.txt)
		if strings.HasPrefix(name, oldBase+".") && name != filepath.Base(oldPath) {
			os.Rename(filepath.Join(oldDir, name), filepath.Join(newDir, newBase+ext))
			continue
		}

		// 2. Archivos de carátula comunes en la carpeta (ej: cover.jpg, folder.png)
		isAlbumSidecar := false
		for _, cs := range commonSidecars {
			if strings.HasPrefix(lowName, cs) {
				isAlbumSidecar = true
				break
			}
		}

		if isAlbumSidecar {
			// Solo lo movemos si el destino no lo tiene ya (ej: ya se movió con otro track del mismo álbum)
			destPath := filepath.Join(newDir, name)
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				os.Rename(filepath.Join(oldDir, name), destPath)
			}
		}
	}
}

// EmbedCover incrusta físicamente una imagen en el archivo de audio
func (e *Editor) EmbedCover(trackID string, imgData []byte, mimeType string) error {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil { return err }

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mp3" {
		tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
		if err != nil { return err }
		defer tag.Close()

		pic := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    mimeType,
			PictureType: id3v2.PTFrontCover,
			Description: "Front Cover",
			Picture:     imgData,
		}
		tag.AddAttachedPicture(pic)
		return tag.Save()
	} else if ext == ".flac" {
		f, err := flac.ParseFile(path)
		if err != nil { return err }

		// Eliminar bloques de imagen existentes para no duplicar
		var newMeta []*flac.MetaDataBlock
		for _, block := range f.Meta {
			if block.Type != flac.Picture {
				newMeta = append(newMeta, block)
			}
		}
		f.Meta = newMeta

		picture, err := flacpicture.NewFromImageData(
			flacpicture.PictureTypeFrontCover,
			"Front Cover",
			imgData,
			mimeType,
		)
		if err != nil { return err }

		pictureMeta := picture.Marshal()
		f.Meta = append(f.Meta, &pictureMeta)
		return f.Save(path)
	}
	return fmt.Errorf("format not supported for embedding covers: %s", ext)
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
	
	trck := getIntStr("trackNum")
	if tt := getIntStr("trackTotal"); tt != "" {
		trck = trck + "/" + tt
	}
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, trck)

	tpos := getIntStr("discNum")
	if dt := getIntStr("discTotal"); dt != "" {
		tpos = tpos + "/" + dt
	}
	tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, tpos)
	tag.AddTextFrame(tag.CommonID("Composer"), id3v2.EncodingUTF8, getString("composer"))
	
	tag.AddCommentFrame(id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    "eng",
		Description: "",
		Text:        getString("comment"),
	})

	// Rating (POPM and RATING text frame)
	rating := 0
	if v, ok := data["rating"].(float64); ok { rating = int(v) }
	if v, ok := data["rating"].(int); ok { rating = v }
	
	if rating > 0 {
		// 1. Modern / MusicBee standard (Text Frame)
		tag.AddTextFrame(tag.CommonID("Rating"), id3v2.EncodingUTF8, fmt.Sprintf("%d", rating))
		
		// 2. Tradicional / Windows standard (POPM)
		// Map 1-5 to 0-255 scale
		popmVal := byte(0)
		switch rating {
			case 1: popmVal = 1
			case 2: popmVal = 64
			case 3: popmVal = 128
			case 4: popmVal = 196
			case 5: popmVal = 255
		}
		
		// Construcción manual del frame POPM si no hay helper directo
		// Cuerpo: Email (null terminated) + Rating (1 byte)
		email := "sgound\x00"
		body := append([]byte(email), popmVal)
		tag.AddFrame("POPM", id3v2.UnknownFrame{Body: body})
	}

	// Instrumental tag (TXXX:INSTRUMENTAL)
	instrumental := "0"
	if v, ok := data["instrumental"].(bool); ok && v { instrumental = "1" }
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
		Encoding:    id3v2.EncodingUTF8,
		Description: "INSTRUMENTAL",
		Value:       instrumental,
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
	setField("TRACKTOTAL", getIntStr("trackTotal"))
	setField("DISCNUMBER", getIntStr("discNum"))
	setField("DISCTOTAL", getIntStr("discTotal"))
	setField("COMPOSER", getString("composer"))
	setField("COMMENT", getString("comment"))
	
	rating := 0
	if v, ok := data["rating"].(float64); ok { rating = int(v) }
	if v, ok := data["rating"].(int); ok { rating = v }
	if rating > 0 {
		setField("RATING", fmt.Sprintf("%d", rating))
	}

	instrumental := "0"
	if v, ok := data["instrumental"].(bool); ok && v { instrumental = "1" }
	setField("INSTRUMENTAL", instrumental)

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
