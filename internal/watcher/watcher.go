package watcher

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher handles recursive file system watching and event debouncing
type Watcher struct {
	watcher    *fsnotify.Watcher
	root       string
	eventQueue chan string
	OnEvent    func(path string)
	
	muMuted    sync.Mutex
	mutedPaths map[string]time.Time
}

func NewWatcher(root string, onEvent func(path string)) (*Watcher, error) {
	nw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		watcher:    nw,
		root:       root,
		eventQueue: make(chan string, 1000), // Buffer para ráfagas de archivos
		OnEvent:    onEvent,
		mutedPaths: make(map[string]time.Time),
	}, nil
}

// MutePath ignora eventos de un archivo durante un tiempo determinado
func (w *Watcher) MutePath(path string, d time.Duration) {
	w.muMuted.Lock()
	defer w.muMuted.Unlock()
	w.mutedPaths[path] = time.Now().Add(d)
}

func (w *Watcher) isMuted(path string) bool {
	w.muMuted.Lock()
	defer w.muMuted.Unlock()
	until, exists := w.mutedPaths[path]
	if !exists { return false }
	if time.Now().After(until) {
		delete(w.mutedPaths, path)
		return false
	}
	return true
}

func (w *Watcher) Start() error {
	// Add root and all subdirectories
	err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			err = w.watcher.Add(path)
			if err != nil {
				slog.Error("Watcher: Failed to add directory", "path", path, "error", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Iniciar worker de procesamiento
	go w.processQueue()
	// Iniciar escucha de eventos del OS
	go w.run()
	return nil
}

func (w *Watcher) Close() error {
	close(w.eventQueue)
	return w.watcher.Close()
}

func (w *Watcher) run() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Add new directories to watch
			if event.Has(fsnotify.Create) {
				if isDir(event.Name) {
					w.watcher.Add(event.Name)
					slog.Info("Watcher: Watching new directory", "path", event.Name)
					
					// Scan contents of new directory
					filepath.WalkDir(event.Name, func(path string, d fs.DirEntry, err error) error {
						if err == nil && !d.IsDir() && isAudioFile(path) {
							w.eventQueue <- path
						}
						if err == nil && d.IsDir() && path != event.Name {
							w.watcher.Add(path)
						}
						return nil
					})
				}
			}

			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
				if !isDir(event.Name) && isAudioFile(event.Name) {
					if w.isMuted(event.Name) {
						continue
					}
					// Enviar a la cola en lugar de procesar directo
					select {
					case w.eventQueue <- event.Name:
					default:
						slog.Warn("Watcher: Event queue full, skipping file", "path", event.Name)
					}
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Watcher error", "error", err)
		}
	}
}

func (w *Watcher) processQueue() {
	// Debounce centralizado: procesar archivos con un pequeño delay
	// para asegurar que la copia terminó
	pending := make(map[string]time.Time)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case path, ok := <-w.eventQueue:
			if !ok {
				return
			}
			pending[path] = time.Now()
		case <-ticker.C:
			now := time.Now()
			for path, lastEvent := range pending {
				if now.Sub(lastEvent) >= 1*time.Second {
					if w.OnEvent != nil {
						w.OnEvent(path)
					}
					delete(pending, path)
				}
			}
		}
	}
}

func isAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := map[string]bool{
		".mp3": true, ".flac": true, ".ogg": true,
		".m4a": true, ".mp4": true, ".opus": true, ".wav": true,
	}
	return supported[ext]
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}
