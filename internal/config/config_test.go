package config

import (
	"os"
	"path/filepath"
	"testing"
)

func cleanupEnv(keys []string) {
	for _, k := range keys {
		os.Unsetenv(k)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	envKeys := []string{
		"SGOUND_DB_PATH", "SGOUND_MUSIC_PATH", "SGOUND_PORT",
		"SGOUND_AUTO_RENAME", "SGOUND_RENAME_PATTERN", "SGOUND_THEME_COLOR",
		"SGOUND_LANGUAGE", "SGOUND_REPLAYGAIN", "SGOUND_USERS", "SGOUND_CACHE_PATH",
	}
	cleanupEnv(envKeys)
	defer cleanupEnv(envKeys)

	cfg := LoadConfig()

	if cfg.DBPath != "sgound.db" {
		t.Errorf("Expected default DBPath 'sgound.db', got %q", cfg.DBPath)
	}
	if cfg.Port != "8088" {
		t.Errorf("Expected default Port '8088', got %q", cfg.Port)
	}
	if cfg.ThemeColor != "#ff3366" {
		t.Errorf("Expected default ThemeColor '#ff3366', got %q", cfg.ThemeColor)
	}
	if cfg.Language != "es" {
		t.Errorf("Expected default Language 'es', got %q", cfg.Language)
	}
	if cfg.AutoRename != false {
		t.Error("Expected default AutoRename false")
	}
	if cfg.RenamePattern != "{artist}/{album}/{track} - {title}" {
		t.Errorf("Expected default RenamePattern, got %q", cfg.RenamePattern)
	}
	if cfg.ReplayGainEnabled != false {
		t.Error("Expected default ReplayGainEnabled false")
	}
	if cfg.CachePath != "./.cache" {
		t.Errorf("Expected default CachePath './.cache', got %q", cfg.CachePath)
	}
}

func TestLoadConfig_DBPath(t *testing.T) {
	cleanupEnv([]string{"SGOUND_DB_PATH"})
	defer os.Unsetenv("SGOUND_DB_PATH")

	os.Setenv("SGOUND_DB_PATH", "/custom/path.db")
	cfg := LoadConfig()

	if cfg.DBPath != "/custom/path.db" {
		t.Errorf("Expected '/custom/path.db', got %q", cfg.DBPath)
	}
}

func TestLoadConfig_MusicPath(t *testing.T) {
	cleanupEnv([]string{"SGOUND_MUSIC_PATH"})
	defer os.Unsetenv("SGOUND_MUSIC_PATH")

	os.Setenv("SGOUND_MUSIC_PATH", "/music/library")
	cfg := LoadConfig()

	if cfg.MusicPath != "/music/library" {
		t.Errorf("Expected '/music/library', got %q", cfg.MusicPath)
	}
}

func TestLoadConfig_Port(t *testing.T) {
	cleanupEnv([]string{"SGOUND_PORT"})
	defer os.Unsetenv("SGOUND_PORT")

	os.Setenv("SGOUND_PORT", "9090")
	cfg := LoadConfig()

	if cfg.Port != "9090" {
		t.Errorf("Expected '9090', got %q", cfg.Port)
	}
}

func TestLoadConfig_AutoRename_True(t *testing.T) {
	cleanupEnv([]string{"SGOUND_AUTO_RENAME"})
	defer os.Unsetenv("SGOUND_AUTO_RENAME")

	os.Setenv("SGOUND_AUTO_RENAME", "true")
	cfg := LoadConfig()

	if !cfg.AutoRename {
		t.Error("Expected AutoRename true")
	}
}

func TestLoadConfig_AutoRename_False(t *testing.T) {
	cleanupEnv([]string{"SGOUND_AUTO_RENAME"})
	defer os.Unsetenv("SGOUND_AUTO_RENAME")

	os.Setenv("SGOUND_AUTO_RENAME", "false")
	cfg := LoadConfig()

	if cfg.AutoRename {
		t.Error("Expected AutoRename false")
	}
}

func TestLoadConfig_RenamePattern(t *testing.T) {
	cleanupEnv([]string{"SGOUND_RENAME_PATTERN"})
	defer os.Unsetenv("SGOUND_RENAME_PATTERN")

	os.Setenv("SGOUND_RENAME_PATTERN", "{year}/{artist}/{title}")
	cfg := LoadConfig()

	if cfg.RenamePattern != "{year}/{artist}/{title}" {
		t.Errorf("Expected custom pattern, got %q", cfg.RenamePattern)
	}
}

func TestLoadConfig_ThemeColor(t *testing.T) {
	cleanupEnv([]string{"SGOUND_THEME_COLOR"})
	defer os.Unsetenv("SGOUND_THEME_COLOR")

	os.Setenv("SGOUND_THEME_COLOR", "#00ff00")
	cfg := LoadConfig()

	if cfg.ThemeColor != "#00ff00" {
		t.Errorf("Expected '#00ff00', got %q", cfg.ThemeColor)
	}
}

func TestLoadConfig_Language(t *testing.T) {
	cleanupEnv([]string{"SGOUND_LANGUAGE"})
	defer os.Unsetenv("SGOUND_LANGUAGE")

	os.Setenv("SGOUND_LANGUAGE", "en")
	cfg := LoadConfig()

	if cfg.Language != "en" {
		t.Errorf("Expected 'en', got %q", cfg.Language)
	}
}

func TestLoadConfig_ReplayGain_True(t *testing.T) {
	cleanupEnv([]string{"SGOUND_REPLAYGAIN"})
	defer os.Unsetenv("SGOUND_REPLAYGAIN")

	os.Setenv("SGOUND_REPLAYGAIN", "true")
	cfg := LoadConfig()

	if !cfg.ReplayGainEnabled {
		t.Error("Expected ReplayGainEnabled true")
	}
}

func TestLoadConfig_ReplayGain_False(t *testing.T) {
	cleanupEnv([]string{"SGOUND_REPLAYGAIN"})
	defer os.Unsetenv("SGOUND_REPLAYGAIN")

	os.Setenv("SGOUND_REPLAYGAIN", "false")
	cfg := LoadConfig()

	if cfg.ReplayGainEnabled {
		t.Error("Expected ReplayGainEnabled false")
	}
}

func TestLoadConfig_Users_SingleReadOnly(t *testing.T) {
	cleanupEnv([]string{"SGOUND_USERS"})
	defer os.Unsetenv("SGOUND_USERS")

	os.Setenv("SGOUND_USERS", "user1")
	cfg := LoadConfig()

	if len(cfg.AllowedUsers) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(cfg.AllowedUsers))
	}
	if cfg.AllowedUsers["user1"] != "r" {
		t.Errorf("Expected permissions 'r', got %q", cfg.AllowedUsers["user1"])
	}
}

func TestLoadConfig_Users_SingleReadWrite(t *testing.T) {
	cleanupEnv([]string{"SGOUND_USERS"})
	defer os.Unsetenv("SGOUND_USERS")

	os.Setenv("SGOUND_USERS", "admin:rw")
	cfg := LoadConfig()

	if len(cfg.AllowedUsers) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(cfg.AllowedUsers))
	}
	if cfg.AllowedUsers["admin"] != "rw" {
		t.Errorf("Expected permissions 'rw', got %q", cfg.AllowedUsers["admin"])
	}
}

func TestLoadConfig_Users_Multiple(t *testing.T) {
	cleanupEnv([]string{"SGOUND_USERS"})
	defer os.Unsetenv("SGOUND_USERS")

	os.Setenv("SGOUND_USERS", "alice,bob:r,carol:rw")
	cfg := LoadConfig()

	if len(cfg.AllowedUsers) != 3 {
		t.Fatalf("Expected 3 users, got %d", len(cfg.AllowedUsers))
	}
	if cfg.AllowedUsers["alice"] != "r" {
		t.Errorf("Expected alice permissions 'r', got %q", cfg.AllowedUsers["alice"])
	}
	if cfg.AllowedUsers["bob"] != "r" {
		t.Errorf("Expected bob permissions 'r', got %q", cfg.AllowedUsers["bob"])
	}
	if cfg.AllowedUsers["carol"] != "rw" {
		t.Errorf("Expected carol permissions 'rw', got %q", cfg.AllowedUsers["carol"])
	}
}

func TestLoadConfig_Users_Empty(t *testing.T) {
	cleanupEnv([]string{"SGOUND_USERS"})
	defer os.Unsetenv("SGOUND_USERS")

	cfg := LoadConfig()

	if len(cfg.AllowedUsers) != 0 {
		t.Errorf("Expected 0 users, got %d", len(cfg.AllowedUsers))
	}
}

func TestLoadConfig_Users_WithSpaces(t *testing.T) {
	cleanupEnv([]string{"SGOUND_USERS"})
	defer os.Unsetenv("SGOUND_USERS")

	os.Setenv("SGOUND_USERS", " alice : r , bob : rw ")
	cfg := LoadConfig()

	if cfg.AllowedUsers["alice"] != "r" {
		t.Errorf("Expected alice permissions 'r', got %q", cfg.AllowedUsers["alice"])
	}
	if cfg.AllowedUsers["bob"] != "rw" {
		t.Errorf("Expected bob permissions 'rw', got %q", cfg.AllowedUsers["bob"])
	}
}

func TestLoadConfig_CachePath(t *testing.T) {
	cleanupEnv([]string{"SGOUND_CACHE_PATH"})
	defer os.Unsetenv("SGOUND_CACHE_PATH")

	os.Setenv("SGOUND_CACHE_PATH", "/tmp/sgound-cache")
	cfg := LoadConfig()

	if cfg.CachePath != "/tmp/sgound-cache" {
		t.Errorf("Expected '/tmp/sgound-cache', got %q", cfg.CachePath)
	}
}

func TestLoadConfig_CachePath_Created(t *testing.T) {
	cleanupEnv([]string{"SGOUND_CACHE_PATH"})
	defer os.Unsetenv("SGOUND_CACHE_PATH")

	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "test-cache")
	os.Setenv("SGOUND_CACHE_PATH", cacheDir)
	LoadConfig()

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("CachePath directory should be created by LoadConfig")
	}
}
