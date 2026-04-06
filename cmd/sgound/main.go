package main

import (
	"log/slog"
	"net/http"
	"os"

	"sgound/internal/api"
	"sgound/internal/config"
	"sgound/internal/db"
	"sgound/internal/scanner"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting sgound server")

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Database
	database, err := db.Connect(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Conn.Close()

	slog.Info("Database connected and schema initialized")

	apiServer := api.NewServer(database)
	mux := apiServer.SetupRoutes()

	// Iniciar el escaner de musica en background
	slog.Info("Init background scanner", "music_dir", cfg.MusicPath)
	libScanner := scanner.NewScanner(database)
	go func() {
		if err := libScanner.ScanDirectory(cfg.MusicPath); err != nil {
			slog.Error("Failed background scan", "err", err)
		}
	}()

	slog.Info("Listening on HTTP port", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		slog.Error("ListenAndServe failed", "err", err)
	}
}
