package db

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func setupTestDB(t *testing.T) *MasterDatabase {
	t.Helper()
	database, err := ConnectMaster(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	t.Cleanup(func() { database.Conn.Close() })
	return database
}

func TestConnectAndInitSchema(t *testing.T) {
	f, err := os.CreateTemp("", "sgound-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp test db: %v", err)
	}
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	database, err := ConnectMaster(dbPath)
	if err != nil {
		t.Fatalf("Expected no error connecting to DB, got %v", err)
	}
	defer database.Conn.Close()

	tables := []string{"users", "artists", "albums", "tracks", "user_tracks", "playlists"}
	for _, table := range tables {
		var name string
		err := database.Conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name = ?;", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created", table)
		}
	}
}

func TestConnectMaster_WALMode(t *testing.T) {
	database := setupTestDB(t)
	var mode string
	database.Conn.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("Expected WAL mode, got %q", mode)
	}
}

func TestCreateUser_Success(t *testing.T) {
	database := setupTestDB(t)
	err := database.CreateUser("alice", "hash123", "rw", "/tmp/test.db", "*")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	user, err := database.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("Expected username 'alice', got %q", user.Username)
	}
	if user.PasswordHash != "hash123" {
		t.Errorf("Expected hash 'hash123', got %q", user.PasswordHash)
	}
	if user.Permissions != "rw" {
		t.Errorf("Expected permissions 'rw', got %q", user.Permissions)
	}
	if user.DBPath != "/tmp/test.db" {
		t.Errorf("Expected DBPath '/tmp/test.db', got %q", user.DBPath)
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	err := database.CreateUser("alice", "hash2", "rw", "", "*")
	if err == nil {
		t.Error("Expected error for duplicate user")
	}
}

func TestCreateUser_EmptyAllowedPaths(t *testing.T) {
	database := setupTestDB(t)
	err := database.CreateUser("alice", "hash", "r", "", "")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	user, _ := database.GetUserByUsername("alice")
	if user.AllowedPaths != "*" {
		t.Errorf("Expected AllowedPaths '*' when empty, got %q", user.AllowedPaths)
	}
}

func TestGetUserByUsername_Found(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "rw", "/db", "path1,path2")
	user, err := database.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if user.AllowedPaths != "path1,path2" {
		t.Errorf("Expected 'path1,path2', got %q", user.AllowedPaths)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	database := setupTestDB(t)
	_, err := database.GetUserByUsername("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent user")
	}
}

func TestGetUsers(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "h1", "r", "", "*")
	database.CreateUser("bob", "h2", "rw", "", "*")

	users, err := database.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestGetUsers_Empty(t *testing.T) {
	database := setupTestDB(t)
	users, err := database.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("Expected 0 users, got %d", len(users))
	}
}

func TestGetUserCount(t *testing.T) {
	database := setupTestDB(t)
	count, _ := database.GetUserCount()
	if count != 0 {
		t.Errorf("Expected 0 users initially, got %d", count)
	}

	database.CreateUser("u1", "h", "r", "", "*")
	database.CreateUser("u2", "h", "r", "", "*")
	count, _ = database.GetUserCount()
	if count != 2 {
		t.Errorf("Expected 2 users, got %d", count)
	}
}

func TestCreateAPIKey_Success(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")

	err := database.CreateAPIKey("key123", "alice", "rw", "My Label")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	keyInfo, err := database.GetAPIKeyInfo("key123")
	if err != nil {
		t.Fatalf("GetAPIKeyInfo failed: %v", err)
	}
	if keyInfo.Key != "key123" {
		t.Errorf("Expected key 'key123', got %q", keyInfo.Key)
	}
	if keyInfo.Username != "alice" {
		t.Errorf("Expected username 'alice', got %q", keyInfo.Username)
	}
	if keyInfo.Label != "My Label" {
		t.Errorf("Expected label 'My Label', got %q", keyInfo.Label)
	}
	if !keyInfo.IsActive {
		t.Error("Expected key to be active by default")
	}
}

func TestGetAPIKeys(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreateKey := func(key, label string) {
		database.CreateAPIKey(key, "alice", "r", label)
	}
	database.CreateKey("k1", "Label1")
	database.CreateKey("k2", "Label2")

	keys, err := database.GetAPIKeys("alice")
	if err != nil {
		t.Fatalf("GetAPIKeys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestGetAPIKeys_OtherUser(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreateUser("bob", "hash", "r", "", "*")
	database.CreateAPIKey("key-alice", "alice", "r", "Alice Key")
	database.CreateAPIKey("key-bob", "bob", "r", "Bob Key")

	keys, _ := database.GetAPIKeys("alice")
	if len(keys) != 1 {
		t.Errorf("Expected 1 key for alice, got %d", len(keys))
	}
	if keys[0].Key != "key-alice" {
		t.Errorf("Expected 'key-alice', got %q", keys[0].Key)
	}
}

func TestDeleteAPIKey(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreateAPIKey("key1", "alice", "r", "Test")

	err := database.DeleteAPIKey("key1", "alice")
	if err != nil {
		t.Fatalf("DeleteAPIKey failed: %v", err)
	}

	_, err = database.GetAPIKeyInfo("key1")
	if err == nil {
		t.Error("Expected error for deleted key")
	}
}

func TestUpdateAPIKeyUsage(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreateAPIKey("key1", "alice", "r", "Test")

	err := database.UpdateAPIKeyUsage("key1", "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("UpdateAPIKeyUsage failed: %v", err)
	}

	keyInfo, _ := database.GetAPIKeyInfo("key1")
	if keyInfo.LastIP != "192.168.1.1" {
		t.Errorf("Expected last_ip '192.168.1.1', got %q", keyInfo.LastIP)
	}
	if keyInfo.UserAgent != "Mozilla/5.0" {
		t.Errorf("Expected user_agent 'Mozilla/5.0', got %q", keyInfo.UserAgent)
	}
}

func TestToggleAPIKeyStatus(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreateAPIKey("key1", "alice", "r", "Test")

	// Deactivate
	err := database.ToggleAPIKeyStatus("key1", "alice", false)
	if err != nil {
		t.Fatalf("ToggleAPIKeyStatus failed: %v", err)
	}
	keyInfo, _ := database.GetAPIKeyInfo("key1")
	if keyInfo.IsActive {
		t.Error("Expected key to be inactive")
	}

	// Reactivate
	database.ToggleAPIKeyStatus("key1", "alice", true)
	keyInfo, _ = database.GetAPIKeyInfo("key1")
	if !keyInfo.IsActive {
		t.Error("Expected key to be active")
	}
}

func TestGetAPIKeyInfo_NotFound(t *testing.T) {
	database := setupTestDB(t)
	_, err := database.GetAPIKeyInfo("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestGetDuplicates_NoDuplicates(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist1')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album1', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, status, file_hash) VALUES ('t1', 'al1', 'Song1', 1, 1, '/path/1', 'active', 'hash1')")

	duplicates, err := database.GetDuplicates()
	if err != nil {
		t.Fatalf("GetDuplicates failed: %v", err)
	}
	if len(duplicates) != 0 {
		t.Errorf("Expected 0 duplicates, got %d", len(duplicates))
	}
}

func TestGetDuplicates_HashDuplicates(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist1')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album1', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, status, file_hash) VALUES ('t1', 'al1', 'Song1', 1, 1, '/path/1', 'active', 'samehash')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, status, file_hash) VALUES ('t2', 'al1', 'Song2', 2, 1, '/path/2', 'active', 'samehash')")

	duplicates, err := database.GetDuplicates()
	if err != nil {
		t.Fatalf("GetDuplicates failed: %v", err)
	}
	if len(duplicates) < 2 {
		t.Errorf("Expected at least 2 hash duplicates, got %d", len(duplicates))
	}
}

func TestGetDuplicates_TagDuplicates(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist1')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album1', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, status, duration, file_hash) VALUES ('t1', 'al1', 'Same Song', 1, 1, '/path/1', 'active', 300, 'h1')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, track_num, disc_num, path, status, duration, file_hash) VALUES ('t2', 'al1', 'Same Song', 2, 1, '/path/2', 'active', 300, 'h2')")

	duplicates, err := database.GetDuplicates()
	if err != nil {
		t.Fatalf("GetDuplicates failed: %v", err)
	}
	if len(duplicates) < 2 {
		t.Errorf("Expected at least 2 tag duplicates, got %d", len(duplicates))
	}
}

func TestGetOrphanedTracks(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")

	// Create a real file for one track, leave the other as non-existent
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.mp3")
	os.WriteFile(realFile, []byte("data"), 0644)

	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Real', ?, 'active')", realFile)
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t2', 'al1', 'Orphan', '/nonexistent/path.mp3', 'active')")

	orphans, err := database.GetOrphanedTracks()
	if err != nil {
		t.Fatalf("GetOrphanedTracks failed: %v", err)
	}
	if len(orphans) != 1 {
		t.Errorf("Expected 1 orphan, got %d", len(orphans))
	}
	if len(orphans) > 0 && orphans[0].ID != "t2" {
		t.Errorf("Expected orphan t2, got %q", orphans[0].ID)
	}
}

func TestGetLowQualityTracks(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, bit_rate, format) VALUES ('t1', 'al1', 'Low', '/1', 'active', 96000, 'mp3')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, bit_rate, format) VALUES ('t2', 'al1', 'High', '/2', 'active', 320000, 'mp3')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status, bit_rate, format) VALUES ('t3', 'al1', 'Lossless', '/3', 'active', 1000000, 'flac')")

	tracks, err := database.GetLowQualityTracks(192)
	if err != nil {
		t.Fatalf("GetLowQualityTracks failed: %v", err)
	}
	if len(tracks) != 1 {
		t.Errorf("Expected 1 low quality track, got %d", len(tracks))
	}
	if len(tracks) > 0 && tracks[0].ID != "t1" {
		t.Errorf("Expected track t1, got %q", tracks[0].ID)
	}
}

func TestDeleteTracks_Batch(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song1', '/1', 'active')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t2', 'al1', 'Song2', '/2', 'active')")

	err := database.DeleteTracks([]string{"t1", "t2"})
	if err != nil {
		t.Fatalf("DeleteTracks failed: %v", err)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 tracks after delete, got %d", count)
	}
}

func TestDeleteTracks_Empty(t *testing.T) {
	database := setupTestDB(t)
	err := database.DeleteTracks([]string{})
	if err != nil {
		t.Errorf("DeleteTracks with empty slice should not error: %v", err)
	}
}

func TestDeleteTracks_TransactionRollback(t *testing.T) {
	database := setupTestDB(t)
	// DeleteTracks with invalid IDs should not panic
	err := database.DeleteTracks([]string{"nonexistent1", "nonexistent2"})
	if err != nil {
		t.Logf("DeleteTracks returned error (expected for missing IDs): %v", err)
	}
}

func TestGetTrackUserData_Default(t *testing.T) {
	database := setupTestDB(t)
	rating, playCount, lastPlayed := database.GetTrackUserData("user1", "track1")
	if rating != 0 {
		t.Errorf("Expected default rating 0, got %d", rating)
	}
	if playCount != 0 {
		t.Errorf("Expected default playCount 0, got %d", playCount)
	}
	if lastPlayed != "" {
		t.Errorf("Expected empty lastPlayed, got %q", lastPlayed)
	}
}

func TestUpdateTrackUserData_Rating(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song', '/path', 'active')")

	err := database.UpdateTrackUserData("alice", "t1", 5, false)
	if err != nil {
		t.Fatalf("UpdateTrackUserData failed: %v", err)
	}

	rating, _, _ := database.GetTrackUserData("alice", "t1")
	if rating != 5 {
		t.Errorf("Expected rating 5, got %d", rating)
	}
}

func TestUpdateTrackUserData_PlayIncrement(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song', '/path', 'active')")

	database.UpdateTrackUserData("alice", "t1", -1, true)
	_, playCount1, _ := database.GetTrackUserData("alice", "t1")
	if playCount1 != 1 {
		t.Errorf("Expected playCount 1, got %d", playCount1)
	}

	database.UpdateTrackUserData("alice", "t1", -1, true)
	_, playCount2, _ := database.GetTrackUserData("alice", "t1")
	if playCount2 != 2 {
		t.Errorf("Expected playCount 2, got %d", playCount2)
	}
}

func TestUpdateTrackUserData_Upsert(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song', '/path', 'active')")

	// First insert
	database.UpdateTrackUserData("alice", "t1", 3, false)
	r1, _, _ := database.GetTrackUserData("alice", "t1")
	if r1 != 3 {
		t.Errorf("Expected rating 3, got %d", r1)
	}

	// Update
	database.UpdateTrackUserData("alice", "t1", 5, false)
	r2, _, _ := database.GetTrackUserData("alice", "t1")
	if r2 != 5 {
		t.Errorf("Expected rating 5 after update, got %d", r2)
	}
}

func TestCreatePlaylist_Success(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")

	err := database.CreatePlaylist("alice", "pl1", "My Playlist", "A description", 0, "")
	if err != nil {
		t.Fatalf("CreatePlaylist failed: %v", err)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlists WHERE id = 'pl1'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 playlist, got %d", count)
	}
}

func TestCreatePlaylist_Smart(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")

	err := database.CreatePlaylist("alice", "sp1", "Smart Playlist", "Auto", 1, "SELECT * FROM tracks")
	if err != nil {
		t.Fatalf("CreatePlaylist failed: %v", err)
	}

	var isSmart int
	database.Conn.QueryRow("SELECT is_smart FROM playlists WHERE id = 'sp1'").Scan(&isSmart)
	if isSmart != 1 {
		t.Errorf("Expected is_smart=1, got %d", isSmart)
	}
}

func TestAddToPlaylist(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreatePlaylist("alice", "pl1", "Playlist", "", 0, "")

	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Song1', '/1', 'active')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t2', 'al1', 'Song2', '/2', 'active')")

	database.AddToPlaylist("pl1", "t1")
	database.AddToPlaylist("pl1", "t2")

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = 'pl1'").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 tracks in playlist, got %d", count)
	}

	// Verify positions
	var pos1, pos2 int
	database.Conn.QueryRow("SELECT position FROM playlist_tracks WHERE playlist_id = 'pl1' AND track_id = 't1'").Scan(&pos1)
	database.Conn.QueryRow("SELECT position FROM playlist_tracks WHERE playlist_id = 'pl1' AND track_id = 't2'").Scan(&pos2)
	if pos1 != 0 || pos2 != 1 {
		t.Errorf("Expected positions 0,1 got %d,%d", pos1, pos2)
	}
}

func TestDeletePlaylist(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreatePlaylist("alice", "pl1", "Playlist", "", 0, "")

	err := database.DeletePlaylist("pl1", "alice")
	if err != nil {
		t.Fatalf("DeletePlaylist failed: %v", err)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlists WHERE id = 'pl1'").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 playlists after delete, got %d", count)
	}
}

func TestDeletePlaylist_WrongUser(t *testing.T) {
	database := setupTestDB(t)
	database.CreateUser("alice", "hash", "r", "", "*")
	database.CreateUser("bob", "hash", "r", "", "*")
	database.CreatePlaylist("alice", "pl1", "Alice's Playlist", "", 0, "")

	err := database.DeletePlaylist("pl1", "bob")
	if err != nil {
		t.Logf("DeletePlaylist with wrong user returned: %v", err)
	}

	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM playlists WHERE id = 'pl1'").Scan(&count)
	if count != 1 {
		t.Error("Playlist should not be deleted by wrong user")
	}
}

func TestGetJWTSecret_SetGet(t *testing.T) {
	database := setupTestDB(t)

	err := database.SetJWTSecret("my-secret-key")
	if err != nil {
		t.Fatalf("SetJWTSecret failed: %v", err)
	}

	secret, err := database.GetJWTSecret()
	if err != nil {
		t.Fatalf("GetJWTSecret failed: %v", err)
	}
	if secret != "my-secret-key" {
		t.Errorf("Expected 'my-secret-key', got %q", secret)
	}
}

func TestGetJWTSecret_NotFound(t *testing.T) {
	database := setupTestDB(t)
	_, err := database.GetJWTSecret()
	if err == nil {
		t.Error("Expected error when no JWT secret set")
	}
}

func TestGetJWTSecret_Overwrite(t *testing.T) {
	database := setupTestDB(t)
	database.SetJWTSecret("secret1")
	database.SetJWTSecret("secret2")

	secret, _ := database.GetJWTSecret()
	if secret != "secret2" {
		t.Errorf("Expected overwritten secret 'secret2', got %q", secret)
	}
}

func TestAddColumnIfNotExists_NewColumn(t *testing.T) {
	database := setupTestDB(t)
	// This should not error (column doesn't exist, will be added)
	database.addColumnIfNotExists("users", "test_new_col", "TEXT DEFAULT 'default'")
}

func TestAddColumnIfNotExists_ExistingColumn(t *testing.T) {
	database := setupTestDB(t)
	// This should not error (column already exists)
	database.addColumnIfNotExists("users", "username", "TEXT")
}

func TestConcurrentUserCreation(t *testing.T) {
	database := setupTestDB(t)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			username := "user" + string(rune('0'+n))
			database.CreateUser(username, "hash", "r", "", "*")
		}(i)
	}
	wg.Wait()

	count, _ := database.GetUserCount()
	if count != 10 {
		t.Errorf("Expected 10 concurrent users, got %d", count)
	}
}

func TestFTS5SearchIndex(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Radiohead')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'OK Computer', 1997)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, artist, path, status) VALUES ('t1', 'al1', 'Karma Police', 'Radiohead', '/path', 'active')")

	// FTS5 should be populated by triggers
	var count int
	database.Conn.QueryRow("SELECT COUNT(*) FROM search_idx WHERE search_idx MATCH 'Karma*'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected FTS5 to find 1 result for 'Karma', got %d", count)
	}
}

func TestInboxCount_Queries(t *testing.T) {
	database := setupTestDB(t)
	database.Conn.Exec("INSERT INTO artists (id, name) VALUES ('a1', 'Artist')")
	database.Conn.Exec("INSERT INTO albums (id, artist_id, title, year) VALUES ('al1', 'a1', 'Album', 2024)")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t1', 'al1', 'Pending', '/1', 'pending')")
	database.Conn.Exec("INSERT INTO tracks (id, album_id, title, path, status) VALUES ('t2', 'al1', 'Active', '/2', 'active')")

	var pending int
	database.Conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE status = 'pending'").Scan(&pending)
	if pending != 1 {
		t.Errorf("Expected 1 pending track, got %d", pending)
	}
}
