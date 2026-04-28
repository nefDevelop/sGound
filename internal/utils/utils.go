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
	name = strings.TrimSpace(name)
	separators := []string{" feat.", " ft.", " feat ", " ft ", " with "}
	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 {
			name = name[:idx]
		}
	}
	return strings.TrimSpace(name)
}

// NormalizeString limpia una cadena general
func NormalizeString(s string) string {
	return strings.TrimSpace(s)
}
