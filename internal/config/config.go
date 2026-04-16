package config

import (
	"log/slog"
	"os"
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
	AllowedUsers      map[string]string `json:"-"` // username -> permissions (r/rw)
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
		musicPath = "./music"
	}

	port := os.Getenv("SGOUND_PORT")
	if port == "" {
		port = "8080"
	}

	autoRename := os.Getenv("SGOUND_AUTO_RENAME") == "true"
	renamePattern := os.Getenv("SGOUND_RENAME_PATTERN")
	if renamePattern == "" {
		renamePattern = "{artist}/{album}/{track} - {title}"
	}

	replayGain := os.Getenv("SGOUND_REPLAYGAIN") == "true"

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

	slog.Info("Configuration loaded", "dbPath", dbPath, "musicPath", musicPath, "port", port, "users", len(allowedUsers))

	return &Config{
		DBPath:        dbPath,
		MusicPath:     musicPath,
		Port:          port,
		AutoRename:        autoRename,
		RenamePattern:     renamePattern,
		ThemeColor:        themeColor,
		Language:          language,
		ReplayGainEnabled: replayGain,
		AllowedUsers:      allowedUsers,
	}
}
