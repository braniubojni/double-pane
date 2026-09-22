package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	_ "modernc.org/sqlite"
)

// DB wraps the application SQLite database (bookmarks + encrypted prefs).
type DB struct {
	sql *sql.DB
	dir string
	key []byte
}

// EnvConfigDir matches config.EnvConfigDir — shared env for e2e isolation.
const EnvConfigDir = "GFM_CONFIG_DIR"

// Open creates (if needed) and migrates the app database under the user config dir.
// If GFM_CONFIG_DIR is set, the DB is stored there as app.db.
func Open(appName string) (*DB, error) {
	var dir string
	if override := os.Getenv(EnvConfigDir); override != "" {
		dir = override
	} else {
		cfg, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(cfg, appName)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "app.db")
	return OpenPath(path)
}

// OpenPath opens a SQLite database at an explicit path (useful for tests).
func OpenPath(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	key, err := loadOrCreateKey(dir)
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}
	// modernc.org/sqlite: enable WAL + foreign keys via DSN query
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	db := &DB{sql: sqlDB, dir: dir, key: key}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Dir returns the config directory that holds app.db and app.key.
func (db *DB) Dir() string {
	if db == nil {
		return ""
	}
	return db.dir
}

// Close closes the database.
func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	return db.sql.Close()
}

func (db *DB) migrate() error {
	_, err := db.sql.Exec(`
CREATE TABLE IF NOT EXISTS bookmarks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  path       TEXT NOT NULL UNIQUE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS kv (
  key   TEXT PRIMARY KEY,
  value BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS remote_recent (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  session_key TEXT NOT NULL,
  path        TEXT NOT NULL UNIQUE,
  label       TEXT NOT NULL,
  visited_at  TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS search_history (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  field   TEXT NOT NULL,
  value   TEXT NOT NULL,
  used_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_search_history_field_value
  ON search_history(field, value);
`)
	return err
}

const maxSearchHistoryPerField = 500

// AddSearchHistory upserts a history value for field (query|replace|include|exclude)
// and caps stored rows at maxSearchHistoryPerField per field.
func (db *DB) AddSearchHistory(field, value string) error {
	field = strings.TrimSpace(field)
	value = strings.TrimSpace(value)
	if field == "" || value == "" {
		return nil
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	_, err := db.sql.Exec(`
INSERT INTO search_history(field, value, used_at)
VALUES(?, ?, ?)
ON CONFLICT(field, value) DO UPDATE SET used_at = excluded.used_at
`, field, value, now)
	if err != nil {
		return err
	}
	_, err = db.sql.Exec(`
DELETE FROM search_history
WHERE field = ? AND id NOT IN (
  SELECT id FROM search_history
  WHERE field = ?
  ORDER BY used_at DESC, id DESC
  LIMIT ?
)
`, field, field, maxSearchHistoryPerField)
	return err
}

// ListSearchHistory returns newest-first history values for field (max limit).
func (db *DB) ListSearchHistory(field string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = maxSearchHistoryPerField
	}
	if limit > maxSearchHistoryPerField {
		limit = maxSearchHistoryPerField
	}
	rows, err := db.sql.Query(`
SELECT value FROM search_history
WHERE field = ?
ORDER BY used_at DESC, id DESC
LIMIT ?
`, field, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

// GetKV returns decrypted plaintext for key, or nil if missing.
func (db *DB) GetKV(key string) ([]byte, error) {
	var blob []byte
	err := db.sql.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return open(db.key, blob)
}

// SetKV encrypts and stores plaintext under key.
func (db *DB) SetKV(key string, plaintext []byte) error {
	blob, err := seal(db.key, plaintext)
	if err != nil {
		return err
	}
	_, err = db.sql.Exec(`
INSERT INTO kv(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, key, blob)
	return err
}

// HasKV reports whether key exists (without decrypting).
func (db *DB) HasKV(key string) (bool, error) {
	var n int
	err := db.sql.QueryRow(`SELECT COUNT(1) FROM kv WHERE key = ?`, key).Scan(&n)
	return n > 0, err
}

// ListBookmarks returns bookmarks ordered by sort_order then id.
func (db *DB) ListBookmarks() ([]domain.Bookmark, error) {
	rows, err := db.sql.Query(`
SELECT id, name, path, sort_order, created_at
FROM bookmarks
ORDER BY sort_order ASC, id ASC
`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.Bookmark
	for rows.Next() {
		var b domain.Bookmark
		if err := rows.Scan(&b.ID, &b.Name, &b.Path, &b.SortOrder, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	if list == nil {
		list = []domain.Bookmark{}
	}
	return list, rows.Err()
}

// AddBookmark inserts a bookmark.
func (db *DB) AddBookmark(name, path string) (domain.Bookmark, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.sql.Exec(`
INSERT INTO bookmarks(name, path, sort_order, created_at)
VALUES(?, ?, COALESCE((SELECT MAX(sort_order)+1 FROM bookmarks), 0), ?)
`, name, path, now)
	if err != nil {
		return domain.Bookmark{}, fmt.Errorf("add bookmark: %w", err)
	}
	id, _ := res.LastInsertId()
	return domain.Bookmark{
		ID:        id,
		Name:      name,
		Path:      path,
		SortOrder: 0,
		CreatedAt: now,
	}, nil
}

// RemoveBookmark deletes a bookmark by id.
func (db *DB) RemoveBookmark(id int64) error {
	_, err := db.sql.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}

// AddRemoteRecent upserts a recently visited remote path.
// Per session_key, only the 10 most recently visited paths are kept.
func (db *DB) AddRemoteRecent(sessionKey, path, label string) error {
	// Nanosecond precision with fixed-width format so SQLite string ordering is correct.
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	_, err := db.sql.Exec(`
INSERT INTO remote_recent(session_key, path, label, visited_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET label = excluded.label, visited_at = excluded.visited_at
`, sessionKey, path, label, now)
	if err != nil {
		return err
	}
	// Cap at 10 per session_key (delete oldest beyond the limit)
	_, err = db.sql.Exec(`
DELETE FROM remote_recent
WHERE session_key = ? AND id NOT IN (
    SELECT id FROM remote_recent
    WHERE session_key = ?
    ORDER BY visited_at DESC, id DESC
    LIMIT 10
)
`, sessionKey, sessionKey)
	return err
}

// DeleteRemoteRecent drops one remembered remote path.
func (db *DB) DeleteRemoteRecent(path string) error {
	_, err := db.sql.Exec(`DELETE FROM remote_recent WHERE path = ?`, path)
	return err
}

// GetRemoteRecent returns recently visited paths for a session_key, newest first.
func (db *DB) GetRemoteRecent(sessionKey string) ([]domain.RemoteRecent, error) {
	rows, err := db.sql.Query(`
SELECT session_key, path, label, visited_at
FROM remote_recent
WHERE session_key = ?
ORDER BY visited_at DESC, id DESC
`, sessionKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []domain.RemoteRecent
	for rows.Next() {
		var r domain.RemoteRecent
		if err := rows.Scan(&r.SessionKey, &r.Path, &r.Label, &r.LastVisited); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if list == nil {
		list = []domain.RemoteRecent{}
	}
	return list, rows.Err()
}
