package api

import (
    "context"
	"encoding/json"
	"fmt"
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
	maxLogs   = 200
	clients   = make(map[chan LogEntry]bool)
	clientMu  sync.Mutex
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

	// Broadcast to active clients
	clientMu.Lock()
	for client := range clients {
		select {
		case client <- entry:
		default:
			// Client slow, skip or drop
		}
	}
	clientMu.Unlock()

	return h.Handler.Handle(ctx, r)
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "text/event-stream" || r.URL.Query().Get("stream") == "true" {
		s.handleLogStream(w, r)
		return
	}

	logMu.Lock()
	logs := make([]LogEntry, len(logBuffer))
	copy(logs, logBuffer)
	logMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	logChan := make(chan LogEntry, 10)
	clientMu.Lock()
	clients[logChan] = true
	clientMu.Unlock()

	defer func() {
		clientMu.Lock()
		delete(clients, logChan)
		clientMu.Unlock()
		close(logChan)
	}()

	// Replay history
	logMu.Lock()
	for _, entry := range logBuffer {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	logMu.Unlock()

	// Initial heartbeat/confirm message
	confirmEntry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "🚀 Real-time log stream established",
	}
	confirmData, _ := json.Marshal(confirmEntry)
	fmt.Fprintf(w, "data: %s\n\n", confirmData)
	w.(http.Flusher).Flush()

	for {
		select {
		case entry := <-logChan:
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

