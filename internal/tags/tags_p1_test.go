package tags

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sgound/internal/db"
)

func TestReadLyrics_NoLyrics(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "no-lyrics.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'mp3')", mp3Path)

	editor := NewEditor(database)
	lyrics, err := editor.ReadLyrics("t1")
	if err != nil {
		t.Fatalf("ReadLyrics failed: %v", err)
	}
	if lyrics != "" {
		t.Errorf("Expected empty lyrics, got %q", lyrics)
	}
}

func TestReadLyrics_NonexistentTrack(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	editor := NewEditor(database)
	lyrics, err := editor.ReadLyrics("nonexistent")
	if err != nil {
		t.Logf("ReadLyrics returned error (acceptable): %v", err)
	}
	if lyrics != "" {
		t.Errorf("Expected empty lyrics for nonexistent track, got %q", lyrics)
	}
}

func TestReadLyrics_SidecarLRC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "song.mp3")
	lrcPath := filepath.Join(tempDir, "song.lrc")

	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Create sidecar .lrc file
	lrcContent := "[00:00.00]Test Lyrics\n[00:05.00]Second line"
	os.WriteFile(lrcPath, []byte(lrcContent), 0644)

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'mp3')", mp3Path)

	editor := NewEditor(database)
	lyrics, err := editor.ReadLyrics("t1")
	if err != nil {
		t.Fatalf("ReadLyrics failed: %v", err)
	}

	if !strings.Contains(lyrics, "Test Lyrics") {
		t.Errorf("Expected lyrics to contain 'Test Lyrics', got %q", lyrics)
	}
}

func TestReadLyrics_FLAC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	flacPath := filepath.Join(tempDir, "song.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5",
		"-metadata", "LYRICS=Test FLAC Lyrics",
		"-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'flac')", flacPath)

	editor := NewEditor(database)
	lyrics, err := editor.ReadLyrics("t1")
	if err != nil {
		t.Fatalf("ReadLyrics failed: %v", err)
	}

	if !strings.Contains(lyrics, "Test FLAC Lyrics") {
		t.Errorf("Expected lyrics to contain 'Test FLAC Lyrics', got %q", lyrics)
	}
}

func TestUpdateLyrics_MP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "lyrics-test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'mp3')", mp3Path)

	editor := NewEditor(database)
	err := editor.UpdateLyrics("t1", "These are the lyrics\nLine two")
	if err != nil {
		t.Fatalf("UpdateLyrics failed: %v", err)
	}

	// Read them back
	lyrics, err := editor.ReadLyrics("t1")
	if err != nil {
		t.Fatalf("ReadLyrics after update failed: %v", err)
	}

	if !strings.Contains(lyrics, "These are the lyrics") {
		t.Errorf("Expected lyrics to contain 'These are the lyrics', got %q", lyrics)
	}
}

func TestUpdateLyrics_ClearLyrics(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "clear-test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5",
		"-metadata", "LYRICS=Old lyrics",
		"-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'mp3')", mp3Path)

	editor := NewEditor(database)
	// Clear lyrics by writing empty string
	err := editor.UpdateLyrics("t1", "")
	if err != nil {
		t.Fatalf("UpdateLyrics with empty string failed: %v", err)
	}
}

func TestUpdateLyrics_NonexistentTrack(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	editor := NewEditor(database)
	err := editor.UpdateLyrics("nonexistent", "lyrics")
	if err == nil {
		t.Error("Expected error for nonexistent track")
	}
}

func TestUpdateLyrics_FLAC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	flacPath := filepath.Join(tempDir, "lyrics-test.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'flac')", flacPath)

	editor := NewEditor(database)
	err := editor.UpdateLyrics("t1", "FLAC lyrics content")
	if err != nil {
		t.Fatalf("UpdateLyrics on FLAC failed: %v", err)
	}
}

func TestEmbedCover_MP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()

	// Create test MP3
	mp3Path := filepath.Join(tempDir, "cover-test.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Create test image (1x1 JPEG)
	imgPath := filepath.Join(tempDir, "cover.jpg")
	cmd = exec.Command("ffmpeg", "-f", "lavfi", "-i", "color=c=blue:s=100x100:d=0.1", "-frames:v", "1", "-y", imgPath)
	if err := cmd.Run(); err != nil {
		t.Skip("Could not create test image")
	}

	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		t.Skip("Could not read test image")
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'mp3')", mp3Path)

	editor := NewEditor(database)
	err = editor.EmbedCover("t1", imgData, "image/jpeg")
	if err != nil {
		t.Fatalf("EmbedCover failed: %v", err)
	}

	// Verify the file still exists and is readable
	info, err := os.Stat(mp3Path)
	if err != nil {
		t.Fatalf("MP3 file missing after embed: %v", err)
	}
	if info.Size() == 0 {
		t.Error("MP3 file is empty after embed")
	}
}

func TestEmbedCover_FLAC(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()

	flacPath := filepath.Join(tempDir, "cover-test.flac")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-f", "flac", "-y", flacPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	imgPath := filepath.Join(tempDir, "cover.jpg")
	cmd = exec.Command("ffmpeg", "-f", "lavfi", "-i", "color=c=blue:s=100x100:d=0.1", "-frames:v", "1", "-y", imgPath)
	if err := cmd.Run(); err != nil {
		t.Skip("Could not create test image")
	}

	imgData, _ := os.ReadFile(imgPath)

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'flac')", flacPath)

	editor := NewEditor(database)
	err := editor.EmbedCover("t1", imgData, "image/jpeg")
	if err != nil {
		t.Fatalf("EmbedCover on FLAC failed: %v", err)
	}
}

func TestEmbedCover_NonexistentTrack(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	editor := NewEditor(database)
	err := editor.EmbedCover("nonexistent", []byte("fake-data"), "image/jpeg")
	if err == nil {
		t.Error("Expected error for nonexistent track")
	}
}

func TestMoveSidecars_SameDir(t *testing.T) {
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "song.mp3")
	newPath := filepath.Join(tempDir, "song-new.mp3")
	os.WriteFile(oldPath, []byte("audio"), 0644)
	os.WriteFile(newPath, []byte("audio"), 0644)

	// Should be no-op
	moveSidecars(oldPath, newPath)

	// No sidecar files to check
}

func TestMoveSidecars_DifferentDir(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	oldPath := filepath.Join(oldDir, "song.mp3")
	newPath := filepath.Join(newDir, "song.mp3")
	os.WriteFile(oldPath, []byte("audio"), 0644)
	os.WriteFile(newPath, []byte("audio"), 0644)

	// Create sidecar
	coverPath := filepath.Join(oldDir, "song.jpg")
	os.WriteFile(coverPath, []byte("cover"), 0644)

	moveSidecars(oldPath, newPath)

	// Cover should have been moved
	newCoverPath := filepath.Join(newDir, "song.jpg")
	if _, err := os.Stat(newCoverPath); os.IsNotExist(err) {
		t.Error("Sidecar cover should have been moved to new directory")
	}
}

func TestMoveSidecars_AlbumCover(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	oldPath := filepath.Join(oldDir, "song.mp3")
	newPath := filepath.Join(newDir, "song.mp3")
	os.WriteFile(oldPath, []byte("audio"), 0644)
	os.WriteFile(newPath, []byte("audio"), 0644)

	// Create album-level sidecar
	folderPath := filepath.Join(oldDir, "folder.jpg")
	os.WriteFile(folderPath, []byte("album cover"), 0644)

	moveSidecars(oldPath, newPath)

	newFolderPath := filepath.Join(newDir, "folder.jpg")
	if _, err := os.Stat(newFolderPath); os.IsNotExist(err) {
		t.Error("Album sidecar 'folder.jpg' should have been moved")
	}
}

func TestUpdateTags_Lyrics_MP3(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "lyrics-tag.mp3")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "0.5", "-y", mp3Path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, format) VALUES ('t1', 'al1', 'Song', ?, 'active', 'mp3')", mp3Path)

	editor := NewEditor(database)
	data := map[string]interface{}{
		"title": "Updated Title",
		"year":  2025,
	}
	_, err := editor.UpdateTags("t1", data)
	if err != nil {
		t.Fatalf("UpdateTags failed: %v", err)
	}

	// Verify title was updated
	tags, _ := ReadTagsFromFile(mp3Path)
	if tags["title"] != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %q", tags["title"])
	}
}

func TestOrganizeFile_Preview(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "unsorted.mp3")
	os.MkdirAll(filepath.Dir(oldPath), 0755)
	os.WriteFile(oldPath, []byte("audio data"), 0644)

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Pink Floyd')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'The Wall', 1979)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, path, format) VALUES ('t1', 'al1', 'Comfortably Numb', 1, ?, 'mp3')", oldPath)

	editor := NewEditor(database)
	newPath, err := editor.OrganizeFilePreview("t1", tempDir, "{artist}/{year} - {album}/{track} - {title}")
	if err != nil {
		t.Fatalf("OrganizeFilePreview failed: %v", err)
	}

	expectedRel := "Pink Floyd/1979 - The Wall/01 - Comfortably Numb.mp3"
	expectedPath := filepath.Join(tempDir, expectedRel)
	if newPath != expectedPath {
		t.Errorf("Preview path mismatch.\nExpected: %s\nGot:      %s", expectedPath, newPath)
	}

	// Original file should still exist (dry run)
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Error("Original file should still exist after preview")
	}
}

func TestOrganizeFile_SamePathNoop(t *testing.T) {
	database, _ := db.ConnectMaster(":memory:")
	defer database.Conn.Close()

	tempDir := t.TempDir()
	songPath := filepath.Join(tempDir, "song.mp3")
	os.WriteFile(songPath, []byte("audio"), 0644)

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, format) VALUES ('t1', 'al1', 'Song', ?, 'mp3')", songPath)

	editor := NewEditor(database)
	// Pattern that would result in same path
	newPath, err := editor.OrganizeFile("t1", tempDir, "song")
	if err != nil {
		t.Fatalf("OrganizeFile failed: %v", err)
	}

	if newPath != songPath {
		t.Errorf("Expected same path for noop, got %q", newPath)
	}
}
