package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DBPath    string
	MusicPath string
	Port      string
}

func LoadConfig() *Config {
	dbPath := os.Getenv("SGOUND_DB_PATH")
	if dbPath == "" {
		dbPath = "sgound.db"
	}

	musicPath := os.Getenv("SGOUND_MUSIC_PATH")
	if musicPath == "" {
		musicPath = "./music"
	}

	port := os.Getenv("SGOUND_PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("Configuration loaded", "dbPath", dbPath, "musicPath", musicPath, "port", port)

	return &Config{
		DBPath:    dbPath,
		MusicPath: musicPath,
		Port:      port,
	}
}
