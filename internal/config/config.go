package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DBPath        string
	MusicPath     string
	Port          string
	AutoRename        bool
	RenamePattern     string
	ThemeColor        string
	ReplayGainEnabled bool
	Language          string `json:"language"`
	CachePath         string
	AllowedUsers      map[string]string `json:"-"` // username -> permissions (r/rw)
	// CookieSecure activa Secure en la cookie de sesión. Solo tras proxy TLS
	// (con http interno debe ser false o el navegador la rechaza).
	CookieSecure bool
}

func LoadConfig() *Config {
	dbPath := os.Getenv("SGOUND_DB_PATH")
	if dbPath == "" {
		dbPath = "sgound.db"
	}

	themeColor := os.Getenv("SGOUND_THEME_COLOR")
	if themeColor == "" {
		themeColor = "#ff3366"
	}

	language := os.Getenv("SGOUND_LANGUAGE")
	if language == "" {
		language = "es"
	}

	musicPath := os.Getenv("SGOUND_MUSIC_PATH")
	if musicPath == "" {
		home, _ := os.UserHomeDir()
		musicPath = filepath.Join(home, "Música")
	}

	port := os.Getenv("SGOUND_PORT")
	if port == "" {
		port = "8088"
	}

	autoRename := os.Getenv("SGOUND_AUTO_RENAME") == "true"
	renamePattern := os.Getenv("SGOUND_RENAME_PATTERN")
	if renamePattern == "" {
		renamePattern = "{artist}/{album}/{track} - {title}"
	}

	replayGain := os.Getenv("SGOUND_REPLAYGAIN") == "true"

	cookieSecure := os.Getenv("COOKIE_SECURE") == "1"

	allowedUsersRaw := os.Getenv("SGOUND_USERS")
	allowedUsers := make(map[string]string)
	if allowedUsersRaw != "" {
		parts := strings.Split(allowedUsersRaw, ",")
		for _, p := range parts {
			kv := strings.Split(p, ":")
			if len(kv) == 2 {
				allowedUsers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			} else if len(kv) == 1 {
				allowedUsers[strings.TrimSpace(kv[0])] = "r" // default to read-only
			}
		}
	}

	cachePath := os.Getenv("SGOUND_CACHE_PATH")
	if cachePath == "" {
		cachePath = "./.cache"
	}
	os.MkdirAll(cachePath, 0755)

	slog.Info("Configuration loaded", "dbPath", dbPath, "musicPath", musicPath, "cachePath", cachePath, "users", len(allowedUsers))

	return &Config{
		DBPath:        dbPath,
		MusicPath:     musicPath,
		Port:          port,
		AutoRename:        autoRename,
		RenamePattern:     renamePattern,
		ThemeColor:        themeColor,
		Language:          language,
		ReplayGainEnabled: replayGain,
		CachePath:         cachePath,
		AllowedUsers:      allowedUsers,
		CookieSecure:      cookieSecure,
	}
}
