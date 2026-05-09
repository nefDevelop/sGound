package tags

import (
	"errors"
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

// TestSafeSave_IntegrityFailure verifica que si la función de guardado
// produce un archivo demasiado pequeño (truncado maliciosamente o por error),
// el archivo temporal es borrado y el original se mantiene.
func TestSafeSave_IntegrityFailure(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "original.txt")
	
	// Crear un archivo grande > 1MB para disparar el chequeo de integridad (< 50%)
	largeData := make([]byte, 2*1024*1024) 
	if err := os.WriteFile(testFile, largeData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	initialInfo, _ := os.Stat(testFile)

	err := SafeSave(testFile, func(tempPath string) error {
		// Simular escritura de un archivo muy pequeño
		return os.WriteFile(tempPath, []byte("too small"), 0644)
	})

	if err == nil {
		t.Fatal("Expected SafeSave to fail due to integrity check, but it succeeded")
	}
	
	// Verificar que el archivo temporal ya no existe
	if _, err := os.Stat(testFile + ".tmp"); !os.IsNotExist(err) {
		t.Error("Temporary file should have been deleted")
	}

	// Verificar que el original está intacto
	currentInfo, err := os.Stat(testFile)
	if err != nil || currentInfo.Size() != initialInfo.Size() {
		t.Error("Original file was modified or deleted despite integrity failure")
	}
}

// TestSafeSave_SaveFnError verifica que si la función de guardado 
// devuelve error, se limpie el temporal y no se reemplace el original.
func TestSafeSave_SaveFnError(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "original.txt")
	
	os.WriteFile(testFile, []byte("original content"), 0644)
	initialInfo, _ := os.Stat(testFile)

	expectedErr := errors.New("simulated save error")
	err := SafeSave(testFile, func(tempPath string) error {
		os.WriteFile(tempPath, []byte("partial write"), 0644)
		return expectedErr
	})

	if err == nil || !errors.Is(err, expectedErr) {
		// La implementación usa fmt.Errorf("save function failed: %w", err) 
		// así que errors.Is funcionará.
		if err == nil {
			t.Fatal("Expected error from save function, got nil")
		}
	}

	// Verificar que el archivo temporal ya no existe
	if _, errStat := os.Stat(testFile + ".tmp"); !os.IsNotExist(errStat) {
		t.Error("Temporary file should have been deleted after save function error")
	}

	// Verificar que el original está intacto
	currentInfo, _ := os.Stat(testFile)
	if currentInfo.Size() != initialInfo.Size() {
		t.Error("Original file was modified despite save function error")
	}
}

// TestSafeSave_NoWritePermission verifica el bloqueo temprano si no 
// hay permisos de escritura en el directorio destino.
func TestSafeSave_NoWritePermission(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "original.txt")
	os.WriteFile(testFile, []byte("data"), 0644)
	
	// Quitar permisos de escritura al directorio temporal
	// Nota: En algunos sistemas (como Windows) esto no funciona igual, pero en Linux (SO del usuario) sí.
	os.Chmod(tempDir, 0555)
	
	// Asegurar restaurar permisos al final para que t.TempDir() se pueda limpiar
	defer os.Chmod(tempDir, 0755)

	err := SafeSave(testFile, func(tempPath string) error {
		return os.WriteFile(tempPath, []byte("new data"), 0644)
	})

	if err == nil {
		t.Fatal("Expected SafeSave to fail due to lack of directory permissions, but it succeeded")
	}
}
