package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// GenerateID genera un ID SHA1 determinista a partir de una cadena
func GenerateID(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

// NormalizeArtist limpia el nombre de un artista para agrupamiento
func NormalizeArtist(name string) string {
	name = strings.ToLower(name)
	// Eliminar caracteres no imprimibles o corruptos (común en tags mal codificados)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 { return -1 }
		return r
	}, name)
	name = strings.TrimSpace(name)
	
	separators := []string{" feat.", " ft.", " feat ", " ft ", " with "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 {
			name = name[:idx]
		}
	}
	// Prefijo al inicio ("feat. Someone"): no hay artista principal,
	// solo invitado -> cadena vacía.
	for _, pre := range []string{"feat.", "feat ", "ft.", "ft ", "with "} {
		if strings.HasPrefix(name, pre) {
			return ""
		}
	}
	return strings.TrimSpace(name)
}

// SanitizeString limpia una cadena de caracteres no imprimibles o corruptos
func SanitizeString(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 { return -1 }
		return r
	}, s))
}

// NormalizeString limpia una cadena general (mantiene compatibilidad)
func NormalizeString(s string) string {
	return SanitizeString(s)
}
