package main

import (
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"sgound/internal/api"
	"sgound/internal/config"
	"sgound/internal/db"
	"sgound/internal/utils"
	"sgound/internal/watcher"
)

func main() {
	// Optimización de hilos y RAM para contenedores
	runtime.GOMAXPROCS(2)
	debug.SetGCPercent(50)

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
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			heapMB := m.HeapAlloc / 1024 / 1024
			sysMB := m.Sys / 1024 / 1024
			releasedMB := m.HeapReleased / 1024 / 1024
			
			slog.Info("🧹 Janitor: Memory Status", 
				"heap_alloc_mb", heapMB, 
				"sys_mb", sysMB, 
				"released_mb", releasedMB,
				"in_use_mb", sysMB - releasedMB)

			// Leer caché del sistema para diagnóstico en Docker
			if data, err := os.ReadFile("/proc/meminfo"); err == nil {
				lines := strings.Split(string(data), "\n")
				var cached, sReclaimable string
				for _, line := range lines {
					if strings.HasPrefix(line, "Cached:") {
						cached = strings.TrimSpace(strings.TrimPrefix(line, "Cached:"))
					}
					if strings.HasPrefix(line, "SReclaimable:") {
						sReclaimable = strings.TrimSpace(strings.TrimPrefix(line, "SReclaimable:"))
					}
				}
				slog.Info("📦 OS Cache (PageCache)", "cached", cached, "reclaimable", sReclaimable)
			}
			
			utils.PurgeMemory(masterDB.Conn)
		}
	}()

	slog.Info("Listening on HTTP port", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		slog.Error("ListenAndServe failed", "err", err)
	}
}
