package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DBPath        string
	MusicPath     string
	Port          string
	AutoRename    bool
	RenamePattern string
	ThemeColor    string
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

	slog.Info("Configuration loaded", "dbPath", dbPath, "musicPath", musicPath, "port", port, "autoRename", autoRename, "themeColor", themeColor)

	return &Config{
		DBPath:        dbPath,
		MusicPath:     musicPath,
		Port:          port,
		AutoRename:    autoRename,
		RenamePattern: renamePattern,
		ThemeColor:    themeColor,
	}
}
