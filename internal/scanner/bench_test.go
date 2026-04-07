package scanner

import (
	"os"
	"path/filepath"
	"sgound/internal/db"
	"testing"
)

func BenchmarkParseAndSaveFile(b *testing.B) {
	// Setup: BD en memoria para máxima velocidad de test
	database, _ := db.Connect(":memory:")
	s := NewScanner(database)

	// Crear un archivo de prueba (aunque no sea un audio real, el parser fallará rápido
	// pero mediremos la lógica de guardado y extracción fallida vs exitosa)
	tmpDir, _ := os.MkdirTemp("", "bench-sgound-*")
	defer os.RemoveAll(tmpDir)
	
	fPath := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(fPath, []byte("fake mp3 content"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ParseAndSaveFile(fPath)
	}
}
