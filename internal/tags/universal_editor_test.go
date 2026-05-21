package tags

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createTestAudio(t *testing.T, path string, format string) {
	// Usar FFmpeg para crear archivos de audio válidos pero vacíos
	args := []string{"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.1", "-y"}
	if format == "m4a" {
		args = append(args, "-c:a", "aac", path)
	} else if format == "ogg" || format == "opus" {
		args = append(args, "-c:a", "libvorbis", path)
	} else if format == "wav" {
		args = append(args, path)
	} else {
		args = append(args, path)
	}

	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create test %s: %v\nOutput: %s", format, err, string(out))
	}
}

func TestUpdateTags_Universal(t *testing.T) {
	db := setupTestDB()
	defer db.Conn.Close()

	formats := []string{"m4a", "ogg", "wav"}
	
	for _, fmtName := range formats {
		t.Run(fmtName, func(t *testing.T) {
			tempDir := t.TempDir()
			testPath := filepath.Join(tempDir, "test."+fmtName)
			createTestAudio(t, testPath, fmtName)

			trackID := "trk-" + fmtName
			db.Conn.Exec("INSERT INTO artists (id, name) VALUES ('art1', 'Old Artist')")
			db.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('alb1', 'art1', 'Old Album', 2000)")
			db.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, format) VALUES (?, 'alb1', 'Old Title', ?, ?)", 
				trackID, testPath, fmtName)

			editor := NewEditor(db)
			
			updates := map[string]interface{}{
				"title": "Universal Title",
				"artist": "Universal Artist",
				"year": 2026,
			}
			
			_, err := editor.UpdateTags(trackID, updates)
			if err != nil {
				t.Fatalf("UpdateTags failed for %s: %v", fmtName, err)
			}
			
			// Verificar DB
			var newTitle string
			db.Conn.QueryRow("SELECT title FROM tracks WHERE id = ?", trackID).Scan(&newTitle)
			if newTitle != "Universal Title" {
				t.Errorf("DB title not updated for %s: %v", fmtName, newTitle)
			}
			
			// Para verificar el archivo físico necesitamos ffprobe
			checkCmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format_tags:stream_tags", "-of", "default=noprint_wrappers=1:nokey=0", testPath)
			out, err := checkCmd.Output()
			if err != nil {
				t.Fatalf("ffprobe failed for %s: %v", fmtName, err)
			}
			
			outStr := string(out)
			if !containsTag(outStr, "title", "Universal Title") {
				t.Errorf("Physical file title not updated for %s. Output:\n%s", fmtName, outStr)
			}
		})
	}
}

func containsTag(output, key, val string) bool {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(key)+"="+strings.ToLower(val)) {
			return true
		}
	}
	return false
}
