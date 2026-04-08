package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"sgound/internal/models"

	_ "modernc.org/sqlite"
	"strings"
)

type Database struct {
	Conn *sql.DB
}

func Connect(dbPath string) (*Database, error) {
	slog.Info("Connecting to database", "path", dbPath)
	// Añadimos parámetros para mejorar la concurrencia y evitar bloqueos (Busy Timeout y modo WAL)
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	conn, err := sql.Open("sqlite", dsn)
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
		status TEXT DEFAULT 'active',
		rating INTEGER DEFAULT 0,
		instrumental INTEGER DEFAULT 0,
		replay_gain REAL DEFAULT 0.0,
		play_count INTEGER DEFAULT 0,
		file_mtime INTEGER,
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
		slog.Error("Schema creation failed", "error", err)
		return err
	}
	slog.Info("Database schema initialized successfully")

	// Migraciones "en caliente" por si se añadieron columnas después
	cols := []string{
		"ALTER TABLE tracks ADD COLUMN last_played DATETIME",
		"ALTER TABLE tracks ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE tracks ADD COLUMN composer TEXT",
		"ALTER TABLE tracks ADD COLUMN comment TEXT",
		"ALTER TABLE tracks ADD COLUMN status TEXT DEFAULT 'active'",
		"ALTER TABLE tracks ADD COLUMN rating INTEGER DEFAULT 0",
		"ALTER TABLE tracks ADD COLUMN instrumental INTEGER DEFAULT 0",
		"ALTER TABLE tracks ADD COLUMN replay_gain REAL DEFAULT 0.0",
		"ALTER TABLE tracks ADD COLUMN play_count INTEGER DEFAULT 0",
		"ALTER TABLE tracks ADD COLUMN file_mtime INTEGER",
	}

	for _, sql := range cols {
		if _, err := db.Conn.Exec(sql); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "already exists") {
				slog.Error("Migration error", "sql", sql, "error", err)
			} else {
				slog.Debug("Migration skipped (already applied)", "sql", sql)
			}
		} else {
			slog.Info("Migration applied successfully", "sql", sql)
		}
	}
	
	return nil
}

func (db *Database) GetDuplicates() ([]models.Track, error) {
	query := `
	SELECT t.id, t.title, a.title as album, art.name as artist, t.path, t.bit_rate
	FROM tracks t
	JOIN albums a ON t.album_id = a.id
	JOIN artists art ON a.artist_id = art.id
	WHERE t.status = 'active' AND (art.id, a.id, t.title) IN (
		SELECT a2.artist_id, t2.album_id, t2.title
		FROM tracks t2
		JOIN albums a2 ON t2.album_id = a2.id
		WHERE t2.status = 'active'
		GROUP BY a2.artist_id, t2.album_id, t2.title
		HAVING COUNT(*) > 1
	)
	ORDER BY art.name, a.title, t.title`

	rows, err := db.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Album, &t.Artist, &t.Path, &t.BitRate); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (db *Database) GetOrphanedTracks() ([]models.Track, error) {
	rows, err := db.Conn.Query("SELECT id, path, title FROM tracks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orphans []models.Track
	for rows.Next() {
		var t models.Track
		if err := rows.Scan(&t.ID, &t.Path, &t.Title); err != nil {
			return nil, err
		}
		
		if _, err := os.Stat(t.Path); os.IsNotExist(err) {
			orphans = append(orphans, t)
		}
	}
	return orphans, nil
}

func (db *Database) GetLowQualityTracks(threshold int) ([]models.Track, error) {
	query := `
	SELECT t.id, t.title, a.title as album, art.name as artist, t.path, t.bit_rate
	FROM tracks t
	JOIN albums a ON t.album_id = a.id
	JOIN artists art ON a.artist_id = art.id
	WHERE t.status = 'active' AND t.bit_rate > 0 AND t.bit_rate < ?
	ORDER BY t.bit_rate ASC`

	rows, err := db.Conn.Query(query, threshold*1000)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Album, &t.Artist, &t.Path, &t.BitRate); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (db *Database) DeleteTracks(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM tracks WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := db.Conn.Exec(query, args...)
	return err
}

func (db *Database) CreatePlaylist(id, name, description string) error {
	_, err := db.Conn.Exec("INSERT INTO playlists (id, name, description) VALUES (?, ?, ?)", id, name, description)
	return err
}

func (db *Database) DeletePlaylist(id string) error {
	_, err := db.Conn.Exec("DELETE FROM playlists WHERE id = ?", id)
	return err
}

func (db *Database) AddToPlaylist(playlistID, trackID string) error {
	var count int
	err := db.Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?", playlistID).Scan(&count)
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec("INSERT INTO playlist_tracks (playlist_id, track_id, position) VALUES (?, ?, ?)", playlistID, trackID, count)
	return err
}
