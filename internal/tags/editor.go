package tags

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"log/slog"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
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
		f, err := ParseFLAC(path)
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

// SafeSave realiza un guardado atómico con verificación de integridad
func SafeSave(path string, saveFn func(tempPath string) error) error {
	slog.Info("Starting SafeSave", "path", path)
	// 1. Obtener tamaño original
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not stat original file: %w", err)
	}
	originalSize := info.Size()

	// 2. Crear archivo temporal en la misma carpeta
	tempPath := path + ".tmp"
	
	// Verificar permisos de escritura en el directorio
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".sgound_perm_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("no write permission in directory %s: %w", dir, err)
	}
	os.Remove(testFile)

	// Limpiar temporal si ya existe
	os.Remove(tempPath)

	// 3. Ejecutar la función de guardado sobre el temporal
	slog.Debug("Executing save function", "tempPath", tempPath)
	if err := saveFn(tempPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("save function failed: %w", err)
	}

	// 4. VERIFICACIÓN DE INTEGRIDAD
	tempInfo, err := os.Stat(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("could not stat temporary file: %w", err)
	}
	
	slog.Info("SafeSave integrity check", "originalSize", originalSize, "newSize", tempInfo.Size())

	if tempInfo.Size() < originalSize/2 && originalSize > 1024*1024 {
		os.Remove(tempPath)
		return fmt.Errorf("integrity check failed: file truncated from %d to %d bytes", originalSize, tempInfo.Size())
	}

	// 5. Reemplazo atómico
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	slog.Info("SafeSave completed successfully", "path", path)
	return nil
}

// WritePersistentID escribe el SGOUND_ID de forma segura usando FFmpeg para preservar el audio
func WritePersistentID(path string, id string) error {
	return SafeSave(path, func(tempPath string) error {
		// Detectar formato
		format := "mp3"
		if strings.ToLower(filepath.Ext(path)) == ".flac" { format = "flac" }

		args := []string{
			"-i", path,
			"-metadata", "SGOUND_ID=" + id,
			"-codec", "copy",
			"-map_metadata", "0",
			"-f", format,
			"-y",
			tempPath,
		}
		
		slog.Debug("Executing FFmpeg (ID write)", "args", args)
		cmd := exec.Command("ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			slog.Error("FFmpeg ID write failed", "error", err, "output", string(output))
			return fmt.Errorf("ffmpeg error: %v", err)
		}
		return nil
	})
}

func updateMP3(path string, _ string, data map[string]interface{}) error {
	return SafeSave(path, func(tempPath string) error {
		args := []string{"-i", path}
		
		if title := getString(data, "title"); title != "" { args = append(args, "-metadata", "title="+title) }
		if artist := getString(data, "artist"); artist != "" { args = append(args, "-metadata", "artist="+artist) }
		if album := getString(data, "album"); album != "" { args = append(args, "-metadata", "album="+album) }
		if genre := getString(data, "genre"); genre != "" { args = append(args, "-metadata", "genre="+genre) }
		if year := getString(data, "year"); year != "" { args = append(args, "-metadata", "date="+year) }
		if aa := getString(data, "albumArtist"); aa != "" { args = append(args, "-metadata", "album_artist="+aa) }
		
		// Forzar formato mp3 para el temporal (.tmp)
		args = append(args, "-codec", "copy", "-map_metadata", "0", "-f", "mp3", "-y", tempPath)
		
		slog.Debug("Executing FFmpeg (MP3 tag update)", "args", args)
		cmd := exec.Command("ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Error("FFmpeg MP3 update failed", "error", err, "output", string(out))
			return fmt.Errorf("ffmpeg mp3 update failed (%v): %s", err, string(out))
		}
		return nil
	})
}

func updateFLAC(path string, _ string, data map[string]interface{}) error {
	return SafeSave(path, func(tempPath string) error {
		args := []string{"-i", path}
		
		if title := getString(data, "title"); title != "" { args = append(args, "-metadata", "TITLE="+title) }
		if artist := getString(data, "artist"); artist != "" { args = append(args, "-metadata", "ARTIST="+artist) }
		if album := getString(data, "album"); album != "" { args = append(args, "-metadata", "ALBUM="+album) }
		if aa := getString(data, "albumArtist"); aa != "" { args = append(args, "-metadata", "ALBUMARTIST="+aa) }
		if genre := getString(data, "genre"); genre != "" { args = append(args, "-metadata", "GENRE="+genre) }
		if year := getString(data, "year"); year != "" { args = append(args, "-metadata", "DATE="+year) }
		
		// Forzar formato flac para el temporal (.tmp)
		args = append(args, "-codec", "copy", "-map_metadata", "0", "-f", "flac", "-y", tempPath)
		
		slog.Debug("Executing FFmpeg (FLAC tag update)", "args", args)
		cmd := exec.Command("ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Error("FFmpeg FLAC update failed", "error", err, "output", string(out))
			return fmt.Errorf("ffmpeg flac update failed (%v): %s", err, string(out))
		}
		return nil
	})
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
		f, err := ParseFLAC(path)
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

func (e *Editor) executeTransaction(fn func(tx *sql.Tx) error) error {
	var err error
	for i := 0; i < 10; i++ { // Aumentado a 10 reintentos
		tx, errTx := e.DB.Conn.Begin()
		if errTx != nil {
			if strings.Contains(errTx.Error(), "database is locked") {
				time.Sleep(time.Duration(200*(i+1)) * time.Millisecond) // Más espera
				continue
			}
			return errTx
		}

		err = fn(tx)
		if err == nil {
			return tx.Commit()
		}

		tx.Rollback()
		if strings.Contains(err.Error(), "database is locked") {
			time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("transaction failed after 10 retries: %w", err)
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

	// WritePersistentID(path, trackID) // DESACTIVADO POR SEGURIDAD

	// 3. Validar cambios
	actualTags, _ := ReadTagsFromFile(path)
	if actualTags == nil { actualTags = make(map[string]string) }

	var changes []TagChange
	mismatches := []string{}

	for field, requestedValueRaw := range data {
		if field == "id" || field == "duration" || field == "size" || field == "format" || field == "bitrate" || field == "sampleRate" {
			continue 
		}

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
	err = e.executeTransaction(func(tx *sql.Tx) error {
		newAlbumTitle := getString(data, "album")
		newAlbumArtist := getString(data, "albumArtist")
		
		if (newAlbumTitle != "" && newAlbumTitle != originalTags["album"]) || 
		   (newAlbumArtist != "" && newAlbumArtist != originalTags["albumArtist"]) {
			
			if newAlbumTitle == "" { newAlbumTitle = originalTags["album"] }
			if newAlbumArtist == "" { newAlbumArtist = originalTags["albumArtist"] }
			if newAlbumArtist == "" { newAlbumArtist = getString(data, "artist") }
			if newAlbumArtist == "" { newAlbumArtist = originalTags["artist"] }

			groupingKey := "AA:" + NormalizeArtist(newAlbumArtist)
			newAlbumID := GenerateID(groupingKey + "|" + NormalizeString(newAlbumTitle))

			if newAlbumID != currentAlbumID {
				artistID := GenerateID(newAlbumArtist)
				if _, err := tx.Exec("INSERT OR IGNORE INTO artists (id, name) VALUES (?, ?)", artistID, newAlbumArtist); err != nil {
					return err
				}
				
				if _, err := tx.Exec("INSERT OR IGNORE INTO albums (id, artist_id, title, year) VALUES (?, ?, ?, ?)", 
					newAlbumID, artistID, newAlbumTitle, getInt(data, "year")); err != nil {
					return err
				}
				
				if _, err := tx.Exec("UPDATE tracks SET album_id = ? WHERE id = ?", newAlbumID, trackID); err != nil {
					return err
				}
				currentAlbumID = newAlbumID
			}
		}

		query := `UPDATE tracks SET 
			title = ?, artist = ?, genre = ?, track_num = ?, track_total = ?, disc_num = ?, disc_total = ?,
			composer = ?, comment = ?, rating = ?, instrumental = ?
			WHERE id = ?`
		
		_, err = tx.Exec(query, 
			getString(data, "title"), getString(data, "artist"), getString(data, "genre"), 
			getInt(data, "trackNum"), getInt(data, "trackTotal"), getInt(data, "discNum"), getInt(data, "discTotal"), 
			getString(data, "composer"), getString(data, "comment"), getInt(data, "rating"), 
			getBoolInt(data, "instrumental"), trackID)
		if err != nil {
			return err
		}

		if currentAlbumID != "" {
			year := getInt(data, "year")
			if year > 0 {
				if _, err := tx.Exec("UPDATE albums SET year = ? WHERE id = ?", year, currentAlbumID); err != nil {
					return err
				}
			}
		}
		return nil
	})

	return changes, err
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

func moveFile(sourcePath, destPath string) error {
	if sourcePath == destPath {
		return nil
	}

	slog.Info("Moving file", "from", sourcePath, "to", destPath)

	// Intentar renombre directo (rápido, misma partición)
	err := os.Rename(sourcePath, destPath)
	if err == nil {
		return nil
	}

	slog.Warn("Rename failed, falling back to copy+delete", "error", err)

	// Si falla por ser diferentes dispositivos, copiar y borrar
	s, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer d.Close()

	if _, err := io.Copy(d, s); err != nil {
		os.Remove(destPath) // Limpiar si falla la copia
		return err
	}

	// Cerrar archivos antes de borrar el origen
	s.Close()
	d.Close()

	if err := os.Remove(sourcePath); err != nil {
		slog.Error("Failed to remove source file after copy", "path", sourcePath, "error", err)
		return err
	}

	return nil
}

func (e *Editor) OrganizeFile(trackID string, musicRoot string, pattern string) (string, error) {
	return e.organizeFileInternal(trackID, musicRoot, pattern, false)
}

func (e *Editor) OrganizeFilePreview(trackID string, musicRoot string, pattern string) (string, error) {
	return e.organizeFileInternal(trackID, musicRoot, pattern, true)
}

func (e *Editor) organizeFileInternal(trackID string, musicRoot string, pattern string, dryRun bool) (string, error) {
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

	clean := func(s string, fallback string) string {
		if s == "" || s == "0" {
			return fallback
		}
		r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
		res := strings.TrimSpace(r.Replace(s))
		if res == "" {
			return fallback
		}
		return res
	}

	newRelPath := pattern
	newRelPath = strings.ReplaceAll(newRelPath, "{artist}", clean(artist, "Unknown Artist"))
	newRelPath = strings.ReplaceAll(newRelPath, "{album}", clean(album, "Unknown Album"))
	newRelPath = strings.ReplaceAll(newRelPath, "{title}", clean(title, "Unknown Title"))
	newRelPath = strings.ReplaceAll(newRelPath, "{genre}", clean(genre, "Unknown"))
	newRelPath = strings.ReplaceAll(newRelPath, "{format}", strings.ToLower(format))

	yearStr := fmt.Sprintf("%d", year)
	if year <= 0 {
		yearStr = "Unknown Year"
	}
	newRelPath = strings.ReplaceAll(newRelPath, "{year}", yearStr)

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

	if dryRun {
		return newPath, nil
	}

	if oldPath == newPath {
		return newPath, nil
	}

	// Verificar que el archivo origen existe
	if _, err := os.Stat(oldPath); err != nil {
		return "", fmt.Errorf("source file does not exist: %s", oldPath)
	}

	newDirPath := filepath.Dir(newPath)
	if err := os.MkdirAll(newDirPath, 0755); err != nil {
		return "", err
	}
	oldDir := filepath.Dir(oldPath)

	// Mover sidecars primero
	moveSidecars(oldPath, newPath)

	// Mover archivo principal
	if err := moveFile(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to move file: %w", err)
	}

	// Actualizar base de datos
	err = e.executeTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE tracks SET path = ? WHERE id = ?", newPath, trackID)
		return err
	})

	if err != nil {
		// ROLLBACK: Intentar mover el archivo de vuelta
		moveFile(newPath, oldPath)
		return "", fmt.Errorf("database update failed, file rolled back: %w", err)
	}

	// Limpiar directorios antiguos si quedan vacíos
	go func(dirPath string, root string) {
		time.Sleep(100 * time.Millisecond)
		d := dirPath
		for d != root && d != "." && d != "/" && d != "" {
			entries, err := os.ReadDir(d)
			if err != nil || len(entries) > 0 {
				break
			}
			if err := os.Remove(d); err != nil {
				break
			}
			d = filepath.Dir(d)
		}
	}(oldDir, musicRoot)

	return newPath, nil
}

func moveSidecars(oldPath, newPath string) {
	oldDir := filepath.Dir(oldPath)
	newDir := filepath.Dir(newPath)
	if oldDir == newDir {
		return
	}

	oldBase := strings.TrimSuffix(filepath.Base(oldPath), filepath.Ext(oldPath))
	newBase := strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath))

	entries, _ := os.ReadDir(oldDir)
	commonSidecars := []string{"cover", "folder", "front", "album", "artwork"}

	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))

		// Sidecars específicos del track
		if strings.HasPrefix(name, oldBase+".") && name != filepath.Base(oldPath) {
			moveFile(filepath.Join(oldDir, name), filepath.Join(newDir, newBase+ext))
			continue
		}

		// Sidecars de album
		isAlbumSidecar := false
		for _, cs := range commonSidecars {
			if strings.HasPrefix(strings.ToLower(name), cs) {
				isAlbumSidecar = true
				break
			}
		}
		if isAlbumSidecar {
			destSidecar := filepath.Join(newDir, name)
			if _, err := os.Stat(destSidecar); os.IsNotExist(err) {
				moveFile(filepath.Join(oldDir, name), destSidecar)
			}
		}
	}
}


func (e *Editor) EmbedCover(trackID string, imgData []byte, mimeType string) error {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil || path == "" { return fmt.Errorf("track not found") }

	return SafeSave(path, func(tempPath string) error {
		// Escribir la imagen a un temporal para dársela a FFmpeg
		tempImgPath := path + ".img.tmp"
		if err := os.WriteFile(tempImgPath, imgData, 0644); err != nil { return err }
		defer os.Remove(tempImgPath)

		// Detectar formato
		format := "mp3"
		if strings.ToLower(filepath.Ext(path)) == ".flac" { format = "flac" }

		args := []string{
			"-i", path,
			"-i", tempImgPath,
			"-map", "0:a",
			"-map", "1:v",
			"-c", "copy",
			"-disposition:v:0", "attached_pic",
			"-f", format,
			"-y", tempPath,
		}

		cmd := exec.Command("ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ffmpeg cover embed failed: %v, output: %s", err, string(out))
		}
		return nil
	})
}

func (e *Editor) UpdateLyrics(trackID string, lyrics string) error {
	var path string
	err := e.DB.Conn.QueryRow("SELECT path FROM tracks WHERE id = ?", trackID).Scan(&path)
	if err != nil || path == "" { return fmt.Errorf("track not found") }

	return SafeSave(path, func(tempPath string) error {
		// FFmpeg puede insertar letras en el campo LYRICS (Vorbis) o USLT (ID3v2)
		// Detectar formato
		format := "mp3"
		if strings.ToLower(filepath.Ext(path)) == ".flac" { format = "flac" }

		args := []string{
			"-i", path,
			"-metadata", "LYRICS=" + lyrics,
			"-metadata", "UNSYNCEDLYRICS=" + lyrics,
			"-codec", "copy",
			"-map_metadata", "0",
			"-f", format,
			"-y", tempPath,
		}
		cmd := exec.Command("ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ffmpeg lyrics update failed: %v, output: %s", err, string(out))
		}
		return nil
	})
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
		f, _ := ParseFLAC(path)
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
