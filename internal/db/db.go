package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
	"strings"
)

type Database struct {
	Conn *sql.DB
}

func Connect(dbPath string) (*Database, error) {
	slog.Info("Connecting to database", "path", dbPath)
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &Database{Conn: conn}
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

func (db *Database) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS artists (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS albums (
		id TEXT PRIMARY KEY,
		artist_id TEXT NOT NULL,
		title TEXT NOT NULL,
		year INTEGER,
		FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
	);
	
	CREATE TABLE IF NOT EXISTS playlists (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		is_smart INTEGER DEFAULT 0,
		rule_sql TEXT, -- Para listas inteligentes
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS tracks (
		id TEXT PRIMARY KEY,
		album_id TEXT NOT NULL,
		title TEXT NOT NULL,
		track_num INTEGER,
		disc_num INTEGER,
		path TEXT NOT NULL,
		duration INTEGER,
		genre TEXT,
		size INTEGER,
		format TEXT,
		composer TEXT,
		comment TEXT,
		bit_rate INTEGER,
		sample_rate INTEGER,
		last_played DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS playlist_tracks (
		playlist_id TEXT,
		track_id TEXT,
		position INTEGER,
		PRIMARY KEY (playlist_id, track_id),
		FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
		FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE
	);`
	_, err := db.Conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migraciones "en caliente" por si se añadieron columnas después
	// Ignorar errores si las columnas ya existen (la mayoría de DBs no tienen IF NOT EXISTS para ADD COLUMN)
	cols := []string{
		"ALTER TABLE tracks ADD COLUMN last_played DATETIME",
		"ALTER TABLE tracks ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE tracks ADD COLUMN composer TEXT",
		"ALTER TABLE tracks ADD COLUMN comment TEXT",
	}

	for _, sql := range cols {
		if _, err := db.Conn.Exec(sql); err != nil {
			// Ignoramos si es un error de "columna ya existe"
			if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "already exists") {
				slog.Error("Migration error", "sql", sql, "error", err)
			}
		} else {
			slog.Info("Migration success", "sql", sql)
		}
	}
	
	return nil
}
