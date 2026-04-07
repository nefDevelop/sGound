package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
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
	return err
}
