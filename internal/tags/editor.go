package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"github.com/go-flac/flacpicture"
	"sgound/internal/db"
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

func GenerateID(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func NormalizeArtist(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	separators := []string{" feat.", " ft.", " feat ", " ft ", " with "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 {
			name = name[:idx]
		}
	}
	return strings.TrimSpace(name)
}

func NormalizeString(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer(".", "", ",", "", "-", "", "(", "", ")", "", "[", "", "]", "", "'", "", "\"", "")
	s = r.Replace(s)
	return strings.TrimSpace(s)
}

// TagChange representa un cambio en un metadato con validación
type TagChange struct {
	Field    string `json:"field"`
	Original string `json:"original"`
	New      string `json:"new"`
	Actual   string `json:"actual"`
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
		
		// Limpiar si existiera uno viejo para evitar duplicados
		tag.DeleteFrames("TXXX") 
		
		tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
			Encoding:    id3v2.EncodingUTF8,
			Description: "SGOUND_ID",
			Value:       id,
		})
		// Forzar guardado completo si es necesario
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

// ReadTagsFromFile lee todos los metadatos soportados de un archivo
func ReadTagsFromFile(path string) (map[string]string, error) {
	tags := make(map[string]string)
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".mp3" {
		tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
		if err != nil { return nil, err }
		defer tag.Close()

		tags["title"] = tag.Title()
		tags["artist"] = tag.Artist()
		tags["album"] = tag.Album()
		tags["genre"] = tag.Genre()
		tags["year"] = tag.Year()
		
		// Album Artist en MP3 (TPE2)
		tags["albumArtist"] = tag.GetTextFrame("TPE2").Text

		trck := tag.GetTextFrame("TRCK").Text
		if trck != "" {
			parts := strings.Split(trck, "/")
			tags["trackNum"] = parts[0]
			if len(parts) > 1 { tags["trackTotal"] = parts[1] }
		}

		tpos := tag.GetTextFrame("TPOS").Text
		if tpos != "" {
			parts := strings.Split(tpos, "/")
			tags["discNum"] = parts[0]
			if len(parts) > 1 { tags["discTotal"] = parts[1] }
		}

		tags["composer"] = tag.GetTextFrame("TCOM").Text
		
		comments := tag.GetFrames(tag.CommonID("Comments"))
		if len(comments) > 0 {
			if cf, ok := comments[0].(id3v2.CommentFrame); ok {
				tags["comment"] = cf.Text
			}
		}

		// Rating
		rating := tag.GetTextFrame("Rating").Text
		if rating == "" {
			popms := tag.GetFrames("POPM")
			if len(popms) > 0 {
				if uf, ok := popms[0].(id3v2.UnknownFrame); ok && len(uf.Body) > 0 {
					parts := strings.Split(string(uf.Body), "\x00")
					if len(parts) > 1 {
						val := int(uf.Body[len(uf.Body)-1])
						if val >= 252 { tags["rating"] = "5" } else if val >= 193 { tags["rating"] = "4" } else if val >= 124 { tags["rating"] = "3" } else if val >= 60 { tags["rating"] = "2" } else if val >= 1 { tags["rating"] = "1" }
					}
				}
			}
		} else {
			tags["rating"] = rating
		}

		txxxs := tag.GetFrames(tag.CommonID("User defined text information"))
		for _, f := range txxxs {
			if tf, ok := f.(id3v2.UserDefinedTextFrame); ok {
				if tf.Description == "INSTRUMENTAL" {
					tags["instrumental"] = tf.Value
				}
			}
		}

	} else if ext == ".flac" {
		f, err := flac.ParseFile(path)
		if err != nil { return nil, err }
		for _, block := range f.Meta {
			if block.Type == flac.VorbisComment {
				cm, _ := flacvorbis.ParseFromMetaDataBlock(*block)
				if cm == nil { continue }
				
				get := func(k string) string {
					v, _ := cm.Get(k)
					if len(v) > 0 { return v[0] }
					return ""
				}

				tags["title"] = get("TITLE")
				tags["artist"] = get("ARTIST")
				tags["album"] = get("ALBUM")
				tags["albumArtist"] = get("ALBUMARTIST")
				tags["genre"] = get("GENRE")
				tags["year"] = get("DATE")
				tags["trackNum"] = get("TRACKNUMBER")
				tags["trackTotal"] = get("TRACKTOTAL")
				tags["discNum"] = get("DISCNUMBER")
				tags["discTotal"] = get("DISCTOTAL")
				tags["composer"] = get("COMPOSER")
				tags["comment"] = get("COMMENT")
				tags["rating"] = get("RATING")
				tags["instrumental"] = get("INSTRUMENTAL")
			}
		}
	}
	return tags, nil
}

type Editor struct {
	DB *db.MasterDatabase
}

func NewEditor(database *db.MasterDatabase) *Editor {
	return &Editor{DB: database}
}

// UpdateTags escribe metadatos en el archivo físico y sincroniza el título localmente.
func (e *Editor) UpdateTags(trackID string, data map[string]interface{}) ([]TagChange, error) {
	var path string
	var currentAlbumID string
	err := e.DB.Conn.QueryRow("SELECT path, album_id FROM tracks WHERE id = ?", trackID).Scan(&path, &currentAlbumID)
	if err != nil {
		return nil, fmt.Errorf("could not find track id: %w", err)
	}

	// 1. Leer etiquetas originales
	originalTags, _ := ReadTagsFromFile(path)
	if originalTags == nil { originalTags = make(map[string]string) }

	// 2. Escribir etiquetas físicas
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		if err := updateMP3(path, trackID, data); err != nil {
			return nil, fmt.Errorf("failed to rewrite mp3 ID3 tags: %w", err)
		}
	case ".flac":
		if err := updateFLAC(path, trackID, data); err != nil {
			return nil, fmt.Errorf("failed to rewrite flac tags: %w", err)
		}
	default:
		fmt.Printf("Physical tag editing not yet supported for format %s, only DB will be updated\n", ext)
	}

	WritePersistentID(path, trackID)

	// 3. Validar cambios
	actualTags, _ := ReadTagsFromFile(path)
	if actualTags == nil { actualTags = make(map[string]string) }

	var changes []TagChange
	mismatches := []string{}

	for field, requestedValueRaw := range data {
		if field == "id" { continue }

		requestedValue := ""
		switch v := requestedValueRaw.(type) {
		case string: requestedValue = v
		case float64: requestedValue = fmt.Sprintf("%d", int(v))
		case int: requestedValue = fmt.Sprintf("%d", v)
		case bool: 
			if v { requestedValue = "1" } else { requestedValue = "0" }
		}

		actualValue := actualTags[field]
		originalValue := originalTags[field]

		change := TagChange{
			Field:    field,
			Original: originalValue,
			New:      requestedValue,
			Actual:   actualValue,
		}
		changes = append(changes, change)

		if requestedValue != "" && requestedValue != "0" && requestedValue != actualValue {
			if !isSoftMatch(field, requestedValue, actualValue) {
				mismatches = append(mismatches, fmt.Sprintf("%s (solicitado '%s', en archivo '%s')", field, requestedValue, actualValue))
			}
		}
	}

	if len(mismatches) > 0 {
		return changes, fmt.Errorf("tag verification failed for: %s", strings.Join(mismatches, ", "))
	}

	// 4. Sincronizar cambios en la BD
	newAlbumTitle := getString(data, "album")
	newAlbumArtist := getString(data, "albumArtist")
	
	// Si cambió el álbum o el artista del álbum, debemos mover el track de album_id
	if (newAlbumTitle != "" && newAlbumTitle != originalTags["album"]) || 
	   (newAlbumArtist != "" && newAlbumArtist != originalTags["albumArtist"]) {
		
		if newAlbumTitle == "" { newAlbumTitle = originalTags["album"] }
		if newAlbumArtist == "" { newAlbumArtist = originalTags["albumArtist"] }
		if newAlbumArtist == "" { newAlbumArtist = getString(data, "artist") }
		if newAlbumArtist == "" { newAlbumArtist = originalTags["artist"] }

		// Lógica de agrupación consistente con el scanner
		groupingKey := "AA:" + NormalizeArtist(newAlbumArtist)
		newAlbumID := GenerateID(groupingKey + "|" + NormalizeString(newAlbumTitle))

		if newAlbumID != currentAlbumID {
			artistID := GenerateID(newAlbumArtist)
			e.DB.Conn.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistID, newAlbumArtist)
			
			e.DB.Conn.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", 
				newAlbumID, artistID, newAlbumTitle, getInt(data, "year"))
			
			e.DB.Conn.Exec("UPDATE tracks SET album_id = ? WHERE id = ?", newAlbumID, trackID)
			currentAlbumID = newAlbumID
		}
	}

	query := `UPDATE tracks SET 
		title = ?, artist = ?, genre = ?, track_num = ?, track_total = ?, disc_num = ?, disc_total = ?,
		composer = ?, comment = ?, rating = ?, instrumental = ?
		WHERE id = ?`
	
	_, err = e.DB.Conn.Exec(query, 
		getString(data, "title"), getString(data, "artist"), getString(data, "genre"), 
		getInt(data, "trackNum"), getInt(data, "trackTotal"), getInt(data, "discNum"), getInt(data, "discTotal"), 
		getString(data, "composer"), getString(data, "comment"), getInt(data, "rating"), 
		getBoolInt(data, "instrumental"), trackID)
	
	if err != nil { return changes, err }

	if currentAlbumID != "" {
		year := getInt(data, "year")
		if year > 0 {
			e.DB.Conn.Exec("UPDATE albums SET year = ? WHERE id = ?", year, currentAlbumID)
		}
	}

	return changes, nil
}

func getString(data map[string]interface{}, k string) string {
	if v, ok := data[k].(string); ok { return v }
	return ""
}
func getInt(data map[string]interface{}, k string) int {
	if v, ok := data[k].(float64); ok { return int(v) }
	if v, ok := data[k].(int); ok { return v }
	return 0
}
func getBoolInt(data map[string]interface{}, k string) int {
	if v, ok := data[k].(bool); ok && v { return 1 }
	return 0
}

func isSoftMatch(field, requested, actual string) bool {
	if requested == actual { return true }
	if field == "trackNum" || field == "discNum" || field == "year" {
		r := strings.TrimLeft(requested, "0")
		a := strings.TrimLeft(actual, "0")
		return r == a
	}
	return false
}

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
	if err != nil { return "", err }

	clean := func(s string) string {
		r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
		return strings.TrimSpace(r.Replace(s))
	}

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
	}

	newDirPath := filepath.Dir(newPath)
	if err := os.MkdirAll(newDirPath, 0755); err != nil { return "", err }
	moveSidecars(oldPath, newPath)
	if err := os.Rename(oldPath, newPath); err != nil { return "", err }

	_, err = e.DB.Conn.Exec("UPDATE tracks SET path = ? WHERE id = ?", newPath, trackID)
	return newPath, err
}

func moveSidecars(oldPath, newPath string) {
	oldDir := filepath.Dir(oldPath)
	newDir := filepath.Dir(newPath)
	oldBase := strings.TrimSuffix(filepath.Base(oldPath), filepath.Ext(oldPath))
	newBase := strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath))

	entries, _ := os.ReadDir(oldDir)
	commonSidecars := []string{"cover", "folder", "front", "album", "artwork"}

	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if strings.HasPrefix(name, oldBase+".") && name != filepath.Base(oldPath) {
			os.Rename(filepath.Join(oldDir, name), filepath.Join(newDir, newBase+ext))
			continue
		}
		isAlbumSidecar := false
		for _, cs := range commonSidecars {
			if strings.HasPrefix(strings.ToLower(name), cs) { isAlbumSidecar = true; break }
		}
		if isAlbumSidecar {
			if _, err := os.Stat(filepath.Join(newDir, name)); os.IsNotExist(err) {
				os.Rename(filepath.Join(oldDir, name), filepath.Join(newDir, name))
			}
		}
	}
}

func (e *Editor) EmbedCover(trackID string, imgData []byte, mimeType string) error {
	var path string
	e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
		if err != nil { return err }
		defer tag.Close()
		tag.AddAttachedPicture(id3v2.PictureFrame{
			Encoding: id3v2.EncodingUTF8, MimeType: mimeType,
			PictureType: id3v2.PTFrontCover, Description: "Front Cover", Picture: imgData,
		})
		return tag.Save()
	case ".flac":
		f, err := flac.ParseFile(path)
		if err != nil { return err }
		var newMeta []*flac.MetaDataBlock
		for _, block := range f.Meta { if block.Type != flac.Picture { newMeta = append(newMeta, block) } }
		f.Meta = newMeta
		picture, _ := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front Cover", imgData, mimeType)
		pictureMeta := picture.Marshal()
		f.Meta = append(f.Meta, &pictureMeta)
		return f.Save(path)
	}
	return nil
}

func (e *Editor) UpdateLyrics(trackID string, lyrics string) error {
	var path string
	e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3": return updateMP3Lyrics(path, lyrics)
	case ".flac", ".ogg": return updateVorbisLyrics(path, lyrics)
	}
	return nil
}

func (e *Editor) ReadLyrics(trackID string) (string, error) {
	var path string
	e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		tag, _ := id3v2.Open(path, id3v2.Options{Parse: true})
		if tag == nil { return "", nil }
		defer tag.Close()
		frames := tag.GetFrames(tag.CommonID("Unsynchronised lyrics/text transcription"))
		for _, f := range frames { if lf, ok := f.(id3v2.UnsynchronisedLyricsFrame); ok { return lf.Lyrics, nil } }
	case ".flac":
		f, _ := flac.ParseFile(path)
		if f == nil { return "", nil }
		for _, block := range f.Meta {
			if block.Type == flac.VorbisComment {
				cm, _ := flacvorbis.ParseFromMetaDataBlock(*block)
				for _, key := range []string{"LYRICS", "UNSYNCEDLYRICS", "TEXT"} {
					vals, _ := cm.Get(key)
					if len(vals) > 0 { return vals[0], nil }
				}
			}
		}
	}

	return "", nil
}

func updateVorbisLyrics(path string, lyrics string) error {
	f, _ := flac.ParseFile(path)
	var cm *flacvorbis.MetaDataBlockVorbisComment
	var cmIdx = -1
	for i, block := range f.Meta { if block.Type == flac.VorbisComment { cm, _ = flacvorbis.ParseFromMetaDataBlock(*block); cmIdx = i; break } }
	if cm == nil { cm = flacvorbis.New() }
	cm.Add("LYRICS", lyrics)
	block := cm.Marshal()
	if cmIdx != -1 { f.Meta[cmIdx] = &block } else { f.Meta = append(f.Meta, &block) }
	return f.Save(path)
}

func updateMP3Lyrics(path string, lyrics string) error {
	tag, _ := id3v2.Open(path, id3v2.Options{Parse: true})
	defer tag.Close()
	tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{Encoding: id3v2.EncodingUTF8, Language: "eng", Lyrics: lyrics})
	return tag.Save()
}

func updateMP3(path string, trackID string, data map[string]interface{}) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil { return err }
	defer tag.Close()

	tag.SetTitle(getString(data, "title"))
	tag.SetArtist(getString(data, "artist"))
	tag.SetAlbum(getString(data, "album"))
	tag.SetGenre(getString(data, "genre"))
	if y := getString(data, "year"); y != "" { tag.SetYear(y) }
	
	// ALBUM ARTIST (TPE2)
	if aa := getString(data, "albumArtist"); aa != "" {
		tag.AddTextFrame("TPE2", id3v2.EncodingUTF8, aa)
	}

	trck := getString(data, "trackNum")
	if tt := getString(data, "trackTotal"); tt != "" { trck = trck + "/" + tt }
	tag.AddTextFrame("TRCK", id3v2.EncodingUTF8, trck)

	tpos := getString(data, "discNum")
	if dt := getString(data, "discTotal"); dt != "" { tpos = tpos + "/" + dt }
	tag.AddTextFrame("TPOS", id3v2.EncodingUTF8, tpos)
	
	tag.AddTextFrame("TCOM", id3v2.EncodingUTF8, getString(data, "composer"))
	tag.AddCommentFrame(id3v2.CommentFrame{Encoding: id3v2.EncodingUTF8, Language: "eng", Text: getString(data, "comment")})

	rating := getInt(data, "rating")
	if rating > 0 {
		tag.AddTextFrame("Rating", id3v2.EncodingUTF8, fmt.Sprintf("%d", rating))
		popmVal := byte(0)
		switch rating { case 1: popmVal = 1; case 2: popmVal = 64; case 3: popmVal = 128; case 4: popmVal = 196; case 5: popmVal = 255 }
		tag.AddFrame("POPM", id3v2.UnknownFrame{Body: append([]byte("sgound\x00"), popmVal)})
	}

	instr := "0"
	if v, ok := data["instrumental"].(bool); ok && v { instr = "1" }
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{Encoding: id3v2.EncodingUTF8, Description: "INSTRUMENTAL", Value: instr})
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{Encoding: id3v2.EncodingUTF8, Description: "SGOUND_ID", Value: trackID})

	return tag.Save()
}

func updateFLAC(path string, trackID string, data map[string]interface{}) error {
	f, err := flac.ParseFile(path)
	if err != nil { return err }
	var cm *flacvorbis.MetaDataBlockVorbisComment
	var cmIdx = -1
	for i, block := range f.Meta { if block.Type == flac.VorbisComment { cm, _ = flacvorbis.ParseFromMetaDataBlock(*block); cmIdx = i; break } }
	if cm == nil { cm = flacvorbis.New() }

	setField := func(field string, value string) {
		if value == "" { return }
		var newComments []string
		for _, c := range cm.Comments { if !strings.HasPrefix(strings.ToUpper(c), field+"=") { newComments = append(newComments, c) } }
		cm.Comments = newComments
		cm.Add(field, value)
	}

	setField("TITLE", getString(data, "title"))
	setField("ARTIST", getString(data, "artist"))
	setField("ALBUM", getString(data, "album"))
	setField("ALBUMARTIST", getString(data, "albumArtist"))
	setField("GENRE", getString(data, "genre"))
	setField("DATE", getString(data, "year"))
	setField("TRACKNUMBER", getString(data, "trackNum"))
	setField("TRACKTOTAL", getString(data, "trackTotal"))
	setField("DISCNUMBER", getString(data, "discNum"))
	setField("DISCTOTAL", getString(data, "discTotal"))
	setField("COMPOSER", getString(data, "composer"))
	setField("COMMENT", getString(data, "comment"))
	if r := getInt(data, "rating"); r > 0 { setField("RATING", fmt.Sprintf("%d", r)) }
	instr := "0"; if v, ok := data["instrumental"].(bool); ok && v { instr = "1" }; setField("INSTRUMENTAL", instr)
	setField("SGOUND_ID", trackID)

	block := cm.Marshal()
	if cmIdx != -1 { f.Meta[cmIdx] = &block } else { f.Meta = append(f.Meta, &block) }
	return f.Save(path)
}
