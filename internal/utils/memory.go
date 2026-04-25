package utils

import (
	"runtime"
	"runtime/debug"
	"log/slog"
	"database/sql"
)

// PurgeMemory fuerza al recolector de basura y devuelve la memoria al sistema operativo.
func PurgeMemory(db *sql.DB) {
	// 1. Limpiar SQLite si se proporciona
	if db != nil {
		_, _ = db.Exec("PRAGMA shrink_memory")
	}

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	inUseBefore := (m1.Sys - m1.HeapReleased) / 1024 / 1024

	runtime.GC()
	debug.FreeOSMemory()

	runtime.ReadMemStats(&m2)
	inUseAfter := (m2.Sys - m2.HeapReleased) / 1024 / 1024
	
	gain := int64(inUseBefore) - int64(inUseAfter)
	if gain > 0 {
		slog.Debug("🧹 Memory purged", "before_mb", inUseBefore, "after_mb", inUseAfter, "freed_mb", gain)
	}
}
