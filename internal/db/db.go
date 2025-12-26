package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func Init(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS parties (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		party_id TEXT NOT NULL,
		name TEXT NOT NULL,
		FOREIGN KEY (party_id) REFERENCES parties(id)
	);

	CREATE TABLE IF NOT EXISTS songs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return db, nil
}
