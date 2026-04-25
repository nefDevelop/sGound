package main

import (
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"sgound/internal/api"
	"sgound/internal/config"
	"sgound/internal/db"
	"sgound/internal/watcher"
)

func main() {
	// Initialize logger
	level := slog.LevelDebug
	if envLevel := os.Getenv("SGOUND_LOG_LEVEL"); envLevel != "" {
		switch strings.ToLower(envLevel) {
		case "debug": level = slog.LevelDebug
		case "info":  level = slog.LevelInfo
		case "warn":  level = slog.LevelWarn
		case "error": level = slog.LevelError
		}
	}

	baseHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(api.NewMemoryHandler(baseHandler))
	slog.SetDefault(logger)

	slog.Info("Starting sgound server")

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Master Database
	masterDB, err := db.ConnectMaster(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize master database", "error", err)
		os.Exit(1)
	}
	defer masterDB.Conn.Close()

	slog.Info("Master Database connected and schema initialized")

	apiServer := api.NewServer(masterDB, cfg)
	mux := apiServer.SetupRoutes()

	// Initialize Watcher
	w, err := watcher.NewWatcher(cfg.MusicPath, apiServer.HandleFileEvent, apiServer.HandleFileDelete)
	if err != nil {
		slog.Error("Failed to initialize file watcher", "error", err)
	} else {
		apiServer.Watcher = w
		if err := w.Start(); err != nil {
			slog.Error("Failed to start file watcher", "error", err)
		} else {
			slog.Info("File watcher started", "path", cfg.MusicPath)
			defer w.Close()
		}
	}

	// Trigger initial scan at startup
	go apiServer.TriggerGlobalScan()

	// Memory Janitor: Free memory periodically to keep idle usage low
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			slog.Debug("🧹 Janitor: Freeing OS memory")
			debug.FreeOSMemory()
		}
	}()

	slog.Info("Listening on HTTP port", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		slog.Error("ListenAndServe failed", "err", err)
	}
}
