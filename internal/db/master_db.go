package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sgound/internal/models"

	_ "modernc.org/sqlite"
)

type MasterDatabase struct {
	Conn *sql.DB
}

type User struct {
	Username     string
	PasswordHash string
	Permissions  string // "r" or "rw"
	DBPath       string
	AllowedPaths string // Comma-separated list of paths, or "*" for all
}

type APIKeyInfo struct {
	Key         string `json:"key"`
	Username    string `json:"username"`
	Permissions string `json:"permissions"`
	Label       string `json:"label"`
	LastIP      string `json:"last_ip"`
	LastUsedAt  string `json:"last_used_at"`
	UserAgent   string `json:"user_agent"`
	IsActive    bool   `json:"is_active"`
}

type Settings struct {
	JWTSecret string
}

func ConnectMaster(dbPath string) (*MasterDatabase, error) {
	slog.Info("Connecting to master database", "path", dbPath)
	// Aumentamos a 15 segundos el timeout para ser ultra pacientes con los bloqueos
	// Limitamos el cache_size a 2MB (2000 * 1024 bytes aprox si el page_size es 1024)
	// O usando valor negativo -2000 para indicar 2000 KB.
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(15000)&_pragma=journal_mode(WAL)&_pragma=cache_size=-2000&_pragma=temp_store=MEMORY", dbPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// LIMITADOR DE CGO Y BLOQUEOS: Solo 1 conexión abierta para SQLite
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	mdb := &MasterDatabase{Conn: conn}
	if err := mdb.initSchema(); err != nil {
		return nil, err
	}

	return mdb, nil
}

func (db *MasterDatabase) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL,
		permissions TEXT NOT NULL,
		db_path TEXT NOT NULL,
		allowed_paths TEXT DEFAULT '*'
	);

	CREATE TABLE IF NOT EXISTS artists (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		image_url TEXT,
		bio TEXT
	);

	CREATE TABLE IF NOT EXISTS albums (
		id TEXT PRIMARY KEY,
		artist_id TEXT NOT NULL,
		title TEXT NOT NULL,
		year INTEGER,
		cover_id TEXT,
		cover_width INTEGER,
		cover_height INTEGER,
		cover_size INTEGER,
		cover_format TEXT,
		FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tracks (
		id TEXT PRIMARY KEY,
		album_id TEXT NOT NULL,
		artist TEXT,
		title TEXT NOT NULL,
		track_num INTEGER,
		track_total INTEGER DEFAULT 0,
		disc_num INTEGER,
		disc_total INTEGER DEFAULT 0,
		path TEXT NOT NULL,
		duration INTEGER,
		genre TEXT,
		size INTEGER,
		format TEXT,
		composer TEXT,
		comment TEXT,
		bit_rate INTEGER,
		sample_rate INTEGER,
		replay_gain REAL DEFAULT 0.0,
		instrumental INTEGER DEFAULT 0,
		file_mtime INTEGER,
		file_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT DEFAULT 'active',
		FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS user_tracks (
		username TEXT,
		track_id TEXT,
		rating INTEGER DEFAULT 0,
		play_count INTEGER DEFAULT 0,
		last_played DATETIME,
		PRIMARY KEY (username, track_id),
		FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE,
		FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS playlists (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		is_smart INTEGER DEFAULT 0,
		rule_sql TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS playlist_tracks (
		playlist_id TEXT,
		track_id TEXT,
		position INTEGER,
		PRIMARY KEY (playlist_id, track_id),
		FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
		FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		key TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		permissions TEXT NOT NULL,
		label TEXT NOT NULL,
		last_ip TEXT DEFAULT '',
		last_used_at DATETIME,
		user_agent TEXT DEFAULT '',
		is_active BOOLEAN DEFAULT 1,
		FOREIGN KEY(username) REFERENCES users(username)
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	-- Índices de Rendimiento
	CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks(path);
	CREATE INDEX IF NOT EXISTS idx_tracks_album_id ON tracks(album_id);
	CREATE INDEX IF NOT EXISTS idx_tracks_status ON tracks(status);
	CREATE INDEX IF NOT EXISTS idx_albums_artist_id ON albums(artist_id);
	CREATE INDEX IF NOT EXISTS idx_user_tracks_username ON user_tracks(username);
	CREATE INDEX IF NOT EXISTS idx_playlist_tracks_playlist ON playlist_tracks(playlist_id);

	-- Tabla Virtual FTS5 para Búsquedas Instantáneas
	CREATE VIRTUAL TABLE IF NOT EXISTS search_idx USING fts5(
		id UNINDEXED, 
		title, 
		artist, 
		album, 
		genre,
		content='tracks', 
		content_rowid='id'
	);

	-- Trigger para mantener sincronizado el índice de búsqueda
	CREATE TRIGGER IF NOT EXISTS trg_tracks_ai AFTER INSERT ON tracks BEGIN
		INSERT INTO search_idx(id, title, artist, album, genre) 
		VALUES (new.id, new.title, new.artist, (SELECT title FROM albums WHERE id=new.album_id), new.genre);
	END;

	CREATE TRIGGER IF NOT EXISTS trg_tracks_ad AFTER DELETE ON tracks BEGIN
		INSERT INTO search_idx(search_idx, id, title, artist, album, genre) 
		VALUES('delete', old.id, old.title, old.artist, (SELECT title FROM albums WHERE id=old.album_id), old.genre);
	END;

	CREATE TRIGGER IF NOT EXISTS trg_tracks_au AFTER UPDATE ON tracks BEGIN
		INSERT INTO search_idx(search_idx, id, title, artist, album, genre) 
		VALUES('delete', old.id, old.title, old.artist, (SELECT title FROM albums WHERE id=old.album_id), old.genre);
		INSERT INTO search_idx(id, title, artist, album, genre) 
		VALUES (new.id, new.title, new.artist, (SELECT title FROM albums WHERE id=new.album_id), new.genre);
	END;
	`
	_, err := db.Conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migraciones básicas y nuevas columnas
	db.Conn.Exec("ALTER TABLE users ADD COLUMN permissions TEXT DEFAULT 'r'")
	db.Conn.Exec("ALTER TABLE users ADD COLUMN allowed_paths TEXT DEFAULT '*'")
	db.Conn.Exec("ALTER TABLE tracks ADD COLUMN replay_gain REAL DEFAULT 0.0")
	db.Conn.Exec("ALTER TABLE tracks ADD COLUMN instrumental INTEGER DEFAULT 0")
	db.Conn.Exec("ALTER TABLE tracks ADD COLUMN rating INTEGER DEFAULT 0")
	db.Conn.Exec("ALTER TABLE artists ADD COLUMN image_url TEXT")
	db.Conn.Exec("ALTER TABLE artists ADD COLUMN bio TEXT")
	db.Conn.Exec("ALTER TABLE artists ADD COLUMN tags TEXT")
	db.Conn.Exec("ALTER TABLE artists ADD COLUMN similar TEXT")
	db.Conn.Exec("ALTER TABLE albums ADD COLUMN cover_id TEXT")
	db.Conn.Exec("ALTER TABLE albums ADD COLUMN cover_width INTEGER")
	db.Conn.Exec("ALTER TABLE albums ADD COLUMN cover_height INTEGER")
	db.Conn.Exec("ALTER TABLE albums ADD COLUMN cover_size INTEGER")
	db.Conn.Exec("ALTER TABLE albums ADD COLUMN cover_format TEXT")
	
	// Índices para optimización de rendimiento
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks(path)")
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_tracks_album_id ON tracks(album_id)")
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_tracks_status ON tracks(status)")
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_tracks_genre ON tracks(genre)")
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_albums_artist_id ON albums(artist_id)")
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_albums_title ON albums(title)")
	db.Conn.Exec("CREATE INDEX IF NOT EXISTS idx_artists_name ON artists(name)")

	return nil
}

func (db *MasterDatabase) CreateUser(username, passwordHash, permissions, userDBPath, allowedPaths string) error {
	// Asegurar que el directorio de la DB del usuario existe
	if userDBPath != "" {
		dir := filepath.Dir(userDBPath)
		os.MkdirAll(dir, 0755)
	}

	if allowedPaths == "" { allowedPaths = "*" }

	_, err := db.Conn.Exec("INSERT INTO users (username, password_hash, permissions, db_path, allowed_paths) VALUES (?, ?, ?, ?, ?)", 
		username, passwordHash, permissions, userDBPath, allowedPaths)
	return err
}

func (db *MasterDatabase) CreateAPIKey(key, username, permissions, label string) error {
	_, err := db.Conn.Exec("INSERT INTO api_keys (key, username, permissions, label) VALUES (?, ?, ?, ?)",
		key, username, permissions, label)
	return err
}

func (db *MasterDatabase) GetAPIKeys(username string) ([]APIKeyInfo, error) {
	rows, err := db.Conn.Query("SELECT key, username, permissions, label, last_ip, IFNULL(last_used_at, ''), user_agent, is_active FROM api_keys WHERE username = ?", username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKeyInfo
	for rows.Next() {
		var k APIKeyInfo
		if err := rows.Scan(&k.Key, &k.Username, &k.Permissions, &k.Label, &k.LastIP, &k.LastUsedAt, &k.UserAgent, &k.IsActive); err != nil {
			continue
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *MasterDatabase) DeleteAPIKey(key, username string) error {
	_, err := db.Conn.Exec("DELETE FROM api_keys WHERE key = ? AND username = ?", key, username)
	return err
}

func (db *MasterDatabase) UpdateAPIKeyUsage(key, ip, ua string) error {
	_, err := db.Conn.Exec("UPDATE api_keys SET last_ip = ?, last_used_at = CURRENT_TIMESTAMP, user_agent = ? WHERE key = ?",
		ip, ua, key)
	return err
}

func (db *MasterDatabase) ToggleAPIKeyStatus(key, username string, active bool) error {
	_, err := db.Conn.Exec("UPDATE api_keys SET is_active = ? WHERE key = ? AND username = ?",
		active, key, username)
	return err
}

func (db *MasterDatabase) GetAPIKeyInfo(key string) (*APIKeyInfo, error) {
	k := &APIKeyInfo{}
	err := db.Conn.QueryRow("SELECT key, username, permissions, label, last_ip, IFNULL(last_used_at, ''), user_agent, is_active FROM api_keys WHERE key = ?", key).
		Scan(&k.Key, &k.Username, &k.Permissions, &k.Label, &k.LastIP, &k.LastUsedAt, &k.UserAgent, &k.IsActive)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (db *MasterDatabase) GetUsers() ([]User, error) {
	rows, err := db.Conn.Query("SELECT username, permissions, db_path, allowed_paths FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.Permissions, &u.DBPath, &u.AllowedPaths); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *MasterDatabase) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := db.Conn.QueryRow("SELECT username, password_hash, permissions, db_path, allowed_paths FROM users WHERE username = ?", username).
		Scan(&u.Username, &u.PasswordHash, &u.Permissions, &u.DBPath, &u.AllowedPaths)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (db *MasterDatabase) GetDuplicates() ([]models.Track, error) {
	query := `
	SELECT t.id, t.title, a.title as album, art.name as artist, t.path, 
	       COALESCE(t.bit_rate, 0), COALESCE(t.duration, 0), 
	       COALESCE(t.format, 'unknown'), COALESCE(t.file_hash, '')
	FROM tracks t
	JOIN albums a ON t.album_id = a.id
	JOIN artists art ON a.artist_id = art.id
	WHERE t.status = 'active' AND (
		(t.file_hash IS NOT NULL AND t.file_hash IN (SELECT t2.file_hash FROM tracks t2 WHERE t2.status = 'active' AND t2.file_hash IS NOT NULL GROUP BY t2.file_hash HAVING COUNT(*) > 1))
		OR
		(t.title, t.duration) IN (SELECT t3.title, t3.duration FROM tracks t3 WHERE t3.status = 'active' GROUP BY t3.title, t3.duration HAVING COUNT(*) > 1)
		OR
		(art.id, a.id, t.title) IN (
			SELECT a2.artist_id, t2.album_id, t2.title
			FROM tracks t2
			JOIN albums a2 ON t2.album_id = a2.id
			WHERE t2.status = 'active'
			GROUP BY a2.artist_id, t2.album_id, t2.title
			HAVING COUNT(*) > 1
		)
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
		if err := rows.Scan(&t.ID, &t.Title, &t.Album, &t.Artist, &t.Path, &t.BitRate, &t.Duration, &t.Format, &t.FileHash); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (db *MasterDatabase) GetOrphanedTracks() ([]models.Track, error) {
	rows, err := db.Conn.Query("SELECT id, path, title FROM tracks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type item struct {
		ID    string
		Path  string
		Title string
	}
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.Path, &i.Title); err == nil {
			items = append(items, i)
		}
	}

	// Use workers to parallelize disk I/O
	jobs := make(chan item, len(items))
	results := make(chan *models.Track, len(items))
	var wg sync.WaitGroup
	numWorkers := 32 // Higher number for I/O bound tasks
	if len(items) < numWorkers { numWorkers = len(items) }

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range jobs {
				if _, err := os.Stat(it.Path); os.IsNotExist(err) {
					results <- &models.Track{ID: it.ID, Path: it.Path, Title: it.Title}
				}
			}
		}()
	}

	for _, it := range items { jobs <- it }
	close(jobs)
	wg.Wait()
	close(results)

	var orphans []models.Track
	for o := range results {
		orphans = append(orphans, *o)
	}

	return orphans, nil
}

func (db *MasterDatabase) GetLowQualityTracks(threshold int) ([]models.Track, error) {
	query := `
	SELECT t.id, t.title, a.title as album, art.name as artist, t.path, COALESCE(t.bit_rate, 0)
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

func (db *MasterDatabase) DeleteTracks(ids []string) error {
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

func (db *MasterDatabase) GetTrackUserData(username, trackID string) (rating, playCount int, lastPlayed string) {
	db.Conn.QueryRow("SELECT rating, play_count, COALESCE(last_played, '') FROM user_tracks WHERE username = ? AND track_id = ?", 
		username, trackID).Scan(&rating, &playCount, &lastPlayed)
	return
}

func (db *MasterDatabase) UpdateTrackUserData(username, trackID string, rating int, playIncrement bool) error {
	playSQL := ""
	if playIncrement {
		playSQL = ", play_count = play_count + 1, last_played = CURRENT_TIMESTAMP"
	}
	
	_, err := db.Conn.Exec(`
		INSERT INTO user_tracks (username, track_id, rating, play_count, last_played) 
		VALUES (?, ?, ?, CASE WHEN ? THEN 1 ELSE 0 END, CASE WHEN ? THEN CURRENT_TIMESTAMP ELSE NULL END)
		ON CONFLICT(username, track_id) DO UPDATE SET 
			rating = CASE WHEN ? >= 0 THEN ? ELSE rating END
			`+playSQL,
		username, trackID, rating, playIncrement, playIncrement, rating >= 0, rating)
	return err
}

func (db *MasterDatabase) CreatePlaylist(username, id, name, description string, isSmart int, ruleSQL string) error {
	_, err := db.Conn.Exec("INSERT INTO playlists (id, username, name, description, is_smart, rule_sql) VALUES (?, ?, ?, ?, ?, ?)", 
		id, username, name, description, isSmart, ruleSQL)
	return err
}

func (db *MasterDatabase) AddToPlaylist(playlistID, trackID string) error {
	var count int
	err := db.Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?", playlistID).Scan(&count)
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec("INSERT INTO playlist_tracks (playlist_id, track_id, position) VALUES (?, ?, ?)", 
		playlistID, trackID, count)
	return err
}

func (db *MasterDatabase) GetUserCount() (int, error) {
	var count int
	err := db.Conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (db *MasterDatabase) GetJWTSecret() (string, error) {
	var secret string
	err := db.Conn.QueryRow("SELECT value FROM settings WHERE key = 'jwt_secret'").Scan(&secret)
	return secret, err
}

func (db *MasterDatabase) SetJWTSecret(secret string) error {
	_, err := db.Conn.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES ('jwt_secret', ?)", secret)
	return err
}

func (db *MasterDatabase) DeletePlaylist(id, username string) error {
	_, err := db.Conn.Exec("DELETE FROM playlists WHERE id = ? AND username = ?", id, username)
	return err
}
