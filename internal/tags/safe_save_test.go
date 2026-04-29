package tags

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSafeUpdateFLAC(t *testing.T) {
	// 1. Crear un FLAC de prueba real usando FFmpeg
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.flac")
	
	// Generar 1 segundo de silencio en FLAC
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-y", testFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test FLAC: %v", err)
	}

	initialInfo, _ := os.Stat(testFile)
	initialSize := initialInfo.Size()

	// 2. Intentar actualizar etiquetas
	data := map[string]interface{}{
		"title":  "Test Title",
		"artist": "Test Artist",
	}

	err := updateFLAC(testFile, "test-id", data)
	if err != nil {
		t.Fatalf("updateFLAC failed: %v", err)
	}

	// 3. Verificar integridad
	newInfo, _ := os.Stat(testFile)
	if newInfo.Size() < initialSize {
		t.Errorf("File shrunk! Initial: %d, New: %d", initialSize, newInfo.Size())
	}

	// 4. Verificar que las etiquetas se grabaron (usando ffprobe o nuestra propia lectura)
	tags, err := ReadTagsFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read tags back: %v", err)
	}

	if tags["title"] != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", tags["title"])
	}
	if tags["artist"] != "Test Artist" {
		t.Errorf("Expected artist 'Test Artist', got '%s'", tags["artist"])
	}
}

func TestSafeUpdateMP3(t *testing.T) {
	// 1. Crear un MP3 de prueba real
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp3")
	
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo", "-t", "1", "-y", testFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	initialInfo, _ := os.Stat(testFile)
	initialSize := initialInfo.Size()

	// 2. Intentar actualizar etiquetas
	data := map[string]interface{}{
		"title":  "MP3 Title",
		"artist": "MP3 Artist",
	}

	err := updateMP3(testFile, "test-id", data)
	if err != nil {
		t.Fatalf("updateMP3 failed: %v\nCheck server logs or run with -v to see slog output if available.", err)
	}

	// 3. Verificar integridad
	newInfo, _ := os.Stat(testFile)
	if newInfo.Size() < initialSize {
		t.Errorf("File shrunk! Initial: %d, New: %d", initialSize, newInfo.Size())
	}

	// 4. Verificar etiquetas
	tags, err := ReadTagsFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read tags back: %v", err)
	}

	if tags["title"] != "MP3 Title" {
		t.Errorf("Expected title 'MP3 Title', got '%s'", tags["title"])
	}
}
