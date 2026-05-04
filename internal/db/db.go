package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Item struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Icon        string    `json:"icon"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Type        string    `json:"type"` // "service" or "bookmark"
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
}

type ReorderItem struct {
	ID       int64  `json:"id"`
	Position int    `json:"position"`
	Type     string `json:"type"`
}

func Init(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA busy_timeout=5000`,
	} {
		if _, err := conn.Exec(pragma); err != nil {
			conn.Close()
			return nil, fmt.Errorf("pragma: %w", err)
		}
	}
	db := &DB{conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'service',
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);
	`)
	if err != nil {
		return err
	}
	// safe migrations for existing installs
	db.Exec(`ALTER TABLE items ADD COLUMN category TEXT NOT NULL DEFAULT ''`)
	return nil
}

func (db *DB) GetSetting(key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (db *DB) HasUsers() (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count > 0, err
}

func (db *DB) CreateUser(username, passwordHash string) error {
	_, err := db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	return err
}

func (db *DB) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetAllUsers() ([]User, error) {
	rows, err := db.Query(`SELECT id, username, password_hash, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) UpdatePassword(id int64, passwordHash string) error {
	_, err := db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (db *DB) UpdateUsername(id int64, username string) error {
	_, err := db.Exec(`UPDATE users SET username = ? WHERE id = ?`, username, id)
	return err
}

func (db *DB) GetItems() ([]Item, error) {
	rows, err := db.Query(`SELECT id, name, url, icon, description, category, type, position, created_at FROM items ORDER BY type, position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name, &it.URL, &it.Icon, &it.Description, &it.Category, &it.Type, &it.Position, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (db *DB) CreateItem(name, url, icon, description, category, itemType string) (*Item, error) {
	var maxPos int
	db.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items WHERE type = ?`, itemType).Scan(&maxPos)
	res, err := db.Exec(
		`INSERT INTO items (name, url, icon, description, category, type, position) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, url, icon, description, category, itemType, maxPos+1,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Item{ID: id, Name: name, URL: url, Icon: icon, Description: description, Category: category, Type: itemType, Position: maxPos + 1}, nil
}

func (db *DB) UpdateItem(id int64, name, url, icon, description, category, itemType string) error {
	_, err := db.Exec(
		`UPDATE items SET name=?, url=?, icon=?, description=?, category=?, type=? WHERE id=?`,
		name, url, icon, description, category, itemType, id,
	)
	return err
}

func (db *DB) ReorderItems(items []ReorderItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, it := range items {
		if _, err := tx.Exec(`UPDATE items SET position=?, type=? WHERE id=?`, it.Position, it.Type, it.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) DeleteItem(id int64) error {
	_, err := db.Exec(`DELETE FROM items WHERE id = ?`, id)
	return err
}
