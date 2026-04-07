package main

import (
	"log/slog"
	"net/http"
	"os"

	"sgound/internal/api"
	"sgound/internal/config"
	"sgound/internal/db"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting sgound server")

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Master Database
	masterDB, err := db.ConnectMaster("master.db")
	if err != nil {
		slog.Error("Failed to initialize master database", "error", err)
		os.Exit(1)
	}
	defer masterDB.Conn.Close()

	slog.Info("Master Database connected and schema initialized")

	apiServer := api.NewServer(masterDB, cfg)
	mux := apiServer.SetupRoutes()

	slog.Info("Listening on HTTP port", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		slog.Error("ListenAndServe failed", "err", err)
	}
}
