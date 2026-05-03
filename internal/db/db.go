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
	Type        string    `json:"type"` // "service" or "bookmark"
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
}

func Init(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
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
	`)
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

func (db *DB) GetItems() ([]Item, error) {
	rows, err := db.Query(`SELECT id, name, url, icon, description, type, position, created_at FROM items ORDER BY type, position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name, &it.URL, &it.Icon, &it.Description, &it.Type, &it.Position, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (db *DB) CreateItem(name, url, icon, description, itemType string) (*Item, error) {
	var maxPos int
	db.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items WHERE type = ?`, itemType).Scan(&maxPos)
	res, err := db.Exec(
		`INSERT INTO items (name, url, icon, description, type, position) VALUES (?, ?, ?, ?, ?, ?)`,
		name, url, icon, description, itemType, maxPos+1,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Item{ID: id, Name: name, URL: url, Icon: icon, Description: description, Type: itemType, Position: maxPos + 1}, nil
}

func (db *DB) UpdateItem(id int64, name, url, icon, description, itemType string) error {
	_, err := db.Exec(
		`UPDATE items SET name=?, url=?, icon=?, description=?, type=? WHERE id=?`,
		name, url, icon, description, itemType, id,
	)
	return err
}

func (db *DB) DeleteItem(id int64) error {
	_, err := db.Exec(`DELETE FROM items WHERE id = ?`, id)
	return err
}
