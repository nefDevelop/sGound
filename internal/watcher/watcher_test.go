package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	tempDir := t.TempDir()

	var eventCalled bool
	w, err := NewWatcher(tempDir, func(path string) {
		eventCalled = true
	}, func(path string) {})

	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	if w.root != tempDir {
		t.Errorf("Expected root %q, got %q", tempDir, w.root)
	}
	if w.eventQueue == nil {
		t.Error("eventQueue should not be nil")
	}
	if w.mutedPaths == nil {
		t.Error("mutedPaths should not be nil")
	}
	_ = eventCalled
}

func TestIsAudioFile_Supported(t *testing.T) {
	supported := []string{
		"track.mp3", "song.flac", "music.ogg",
		"audio.m4a", "clip.mp4", "voice.opus", "recording.wav",
	}
	for _, name := range supported {
		if !isAudioFile(name) {
			t.Errorf("isAudioFile(%q) should be true", name)
		}
	}
}

func TestIsAudioFile_Unsupported(t *testing.T) {
	unsupported := []string{
		"doc.txt", "image.jpg", "video.mkv", "archive.zip",
	}
	for _, name := range unsupported {
		if isAudioFile(name) {
			t.Errorf("isAudioFile(%q) should be false", name)
		}
	}
}

func TestIsAudioFile_CaseInsensitive(t *testing.T) {
	cases := []string{"track.MP3", "song.FLAC", "music.OGG"}
	for _, name := range cases {
		if !isAudioFile(name) {
			t.Errorf("isAudioFile(%q) should be case-insensitive", name)
		}
	}
}

func TestIsDir(t *testing.T) {
	tempDir := t.TempDir()
	if !isDir(tempDir) {
		t.Error("isDir should return true for directory")
	}

	filePath := filepath.Join(tempDir, "file.txt")
	os.WriteFile(filePath, []byte("content"), 0644)
	if isDir(filePath) {
		t.Error("isDir should return false for file")
	}
}

func TestIsDir_Nonexistent(t *testing.T) {
	if isDir("/nonexistent/path") {
		t.Error("isDir should return false for nonexistent path")
	}
}

func TestMutePath(t *testing.T) {
	tempDir := t.TempDir()
	w, _ := NewWatcher(tempDir, func(path string) {}, func(path string) {})
	defer w.Close()

	w.MutePath("/test/file.mp3", 5*time.Second)

	if !w.isMuted("/test/file.mp3") {
		t.Error("Path should be muted after MutePath")
	}
}

func TestMutePath_Expired(t *testing.T) {
	tempDir := t.TempDir()
	w, _ := NewWatcher(tempDir, func(path string) {}, func(path string) {})
	defer w.Close()

	w.MutePath("/test/file.mp3", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	if w.isMuted("/test/file.mp3") {
		t.Error("Path should not be muted after duration expires")
	}
}

func TestMutePath_DifferentPath(t *testing.T) {
	tempDir := t.TempDir()
	w, _ := NewWatcher(tempDir, func(path string) {}, func(path string) {})
	defer w.Close()

	w.MutePath("/test/file.mp3", 5*time.Second)

	if w.isMuted("/test/other.mp3") {
		t.Error("Different path should not be muted")
	}
}

func TestStart_Close(t *testing.T) {
	tempDir := t.TempDir()
	w, err := NewWatcher(tempDir, func(path string) {}, func(path string) {})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	err = w.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Allow goroutines to start
	time.Sleep(50 * time.Millisecond)

	err = w.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestStart_WatchesSubdirectories(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	os.MkdirAll(subDir, 0755)

	w, _ := NewWatcher(tempDir, func(path string) {}, func(path string) {})
	defer w.Close()

	err := w.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// The watcher should now be watching both root and subdirectory
	// We can't directly inspect the internal watcher, but we can verify
	// that Start didn't error on recursive directory walk
}

func TestWatcher_CreateEvent(t *testing.T) {
	tempDir := t.TempDir()

	eventReceived := make(chan string, 1)
	w, _ := NewWatcher(tempDir, func(path string) {
		select {
		case eventReceived <- path:
		default:
		}
	}, func(path string) {})

	defer w.Close()
	w.Start()

	// Create an audio file
	audioFile := filepath.Join(tempDir, "new_song.mp3")
	os.WriteFile(audioFile, []byte("audio"), 0644)

	// Wait for event (debounced ~1s)
	select {
	case path := <-eventReceived:
		if path != audioFile {
			t.Errorf("Expected event for %q, got %q", audioFile, path)
		}
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for create event")
	}
}

func TestWatcher_DeleteEvent(t *testing.T) {
	tempDir := t.TempDir()

	deleteReceived := make(chan string, 1)
	w, _ := NewWatcher(tempDir, func(path string) {}, func(path string) {
		select {
		case deleteReceived <- path:
		default:
		}
	})

	defer w.Close()
	w.Start()

	// Create and then delete a file
	audioFile := filepath.Join(tempDir, "to_delete.mp3")
	os.WriteFile(audioFile, []byte("audio"), 0644)
	time.Sleep(100 * time.Millisecond)
	os.Remove(audioFile)

	select {
	case <-deleteReceived:
		// Delete event received
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for delete event")
	}
}

func TestWatcher_IgnoresNonAudio(t *testing.T) {
	tempDir := t.TempDir()

	eventReceived := make(chan string, 1)
	w, _ := NewWatcher(tempDir, func(path string) {
		select {
		case eventReceived <- path:
		default:
		}
	}, func(path string) {})

	defer w.Close()
	w.Start()

	// Create a non-audio file
	txtFile := filepath.Join(tempDir, "readme.txt")
	os.WriteFile(txtFile, []byte("text"), 0644)

	select {
	case <-eventReceived:
		t.Error("Should not receive event for non-audio file")
	case <-time.After(2 * time.Second):
		// Expected: no event
	}
}

func TestIsMuted_CleanExpired(t *testing.T) {
	tempDir := t.TempDir()
	w, _ := NewWatcher(tempDir, func(path string) {}, func(path string) {})
	defer w.Close()

	// Mute with very short duration
	w.MutePath("/test/file.mp3", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	// After expiry, isMuted should return false AND clean up the entry
	if w.isMuted("/test/file.mp3") {
		t.Error("Expired muted path should not be muted")
	}
	w.muMuted.Lock()
	_, exists := w.mutedPaths["/test/file.mp3"]
	w.muMuted.Unlock()

	if exists {
		t.Error("Expired muted path should be cleaned up from map")
	}
}
