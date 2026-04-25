package api

import (
    "context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
    "log/slog"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var (
	logBuffer []LogEntry
	logMu     sync.Mutex
	maxLogs   = 100
)

type MemoryHandler struct {
	slog.Handler
}

func NewMemoryHandler(base slog.Handler) *MemoryHandler {
	return &MemoryHandler{base}
}

func (h *MemoryHandler) Handle(ctx context.Context, r slog.Record) error {
	entry := LogEntry{
		Timestamp: r.Time,
		Level:     r.Level.String(),
		Message:   r.Message,
		Fields:    make(map[string]interface{}),
	}

	r.Attrs(func(a slog.Attr) bool {
		entry.Fields[a.Key] = a.Value.Any()
		return true
	})

	logMu.Lock()
	logBuffer = append(logBuffer, entry)
	if len(logBuffer) > maxLogs {
		logBuffer = logBuffer[1:]
	}
	logMu.Unlock()

	return h.Handler.Handle(ctx, r)
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	logMu.Lock()
	defer logMu.Unlock()

    // Enviar los últimos logs (pueden ser muchos, así que limitamos si es necesario)
    logs := logBuffer
    if len(logs) == 0 {
        logs = []LogEntry{}
    }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
