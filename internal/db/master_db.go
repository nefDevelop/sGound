package db

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
)

type MasterDatabase struct {
	Conn *sql.DB
}

type User struct {
	Username     string
	PasswordHash string
	Permissions  string // "r" or "rw"
	DBPath       string
}

type APIKeyInfo struct {
	Key         string `json:"key"`
	Username    string `json:"username"`
	Permissions string `json:"permissions"`
	Label       string `json:"label"`
}

type Settings struct {
	JWTSecret string
}

func ConnectMaster(dbPath string) (*MasterDatabase, error) {
	slog.Info("Connecting to master database", "path", dbPath)
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

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
		db_path TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS api_keys (
		key TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		permissions TEXT NOT NULL,
		label TEXT NOT NULL,
		FOREIGN KEY(username) REFERENCES users(username)
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`
	_, err := db.Conn.Exec(schema)
	if err != nil {
		return err
	}

	// 2. Migración: Si la columna 'permissions' no existe, añadirla
	// Intentamos añadirla (si ya existe, el error será ignorado)
	db.Conn.Exec("ALTER TABLE users ADD COLUMN permissions TEXT DEFAULT 'r'")
	
	return nil
}

func (db *MasterDatabase) CreateUser(username, passwordHash, permissions, userDBPath string) error {
	// Asegurar que el directorio de la DB del usuario existe
	dir := filepath.Dir(userDBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	_, err := db.Conn.Exec("INSERT INTO users (username, password_hash, permissions, db_path) VALUES (?, ?, ?, ?)", 
		username, passwordHash, permissions, userDBPath)
	return err
}

func (db *MasterDatabase) CreateAPIKey(key, username, permissions, label string) error {
	_, err := db.Conn.Exec("INSERT INTO api_keys (key, username, permissions, label) VALUES (?, ?, ?, ?)",
		key, username, permissions, label)
	return err
}

func (db *MasterDatabase) GetAPIKeys(username string) ([]APIKeyInfo, error) {
	rows, err := db.Conn.Query("SELECT key, username, permissions, label FROM api_keys WHERE username = ?", username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKeyInfo
	for rows.Next() {
		var k APIKeyInfo
		if err := rows.Scan(&k.Key, &k.Username, &k.Permissions, &k.Label); err != nil {
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

func (db *MasterDatabase) GetAPIKeyInfo(key string) (*APIKeyInfo, error) {
	k := &APIKeyInfo{}
	err := db.Conn.QueryRow("SELECT key, username, permissions, label FROM api_keys WHERE key = ?", key).
		Scan(&k.Key, &k.Username, &k.Permissions, &k.Label)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (db *MasterDatabase) GetUsers() ([]User, error) {
	rows, err := db.Conn.Query("SELECT username, permissions, db_path FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.Permissions, &u.DBPath); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *MasterDatabase) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := db.Conn.QueryRow("SELECT username, password_hash, permissions, db_path FROM users WHERE username = ?", username).
		Scan(&u.Username, &u.PasswordHash, &u.Permissions, &u.DBPath)
	if err != nil {
		return nil, err
	}
	return u, nil
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
