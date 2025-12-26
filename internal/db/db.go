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
		name TEXT NOT NULL,
		started BOOLEAN DEFAULT FALSE,
		current_round INTEGER DEFAULT 0
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
		round_number INTEGER DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS guesses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		guesser_id INTEGER NOT NULL,
		song_id INTEGER NOT NULL,
		guessed_user_id INTEGER NOT NULL,
		FOREIGN KEY (guesser_id) REFERENCES users(id),
		FOREIGN KEY (song_id) REFERENCES songs(id),
		FOREIGN KEY (guessed_user_id) REFERENCES users(id),
		UNIQUE(guesser_id, song_id)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return db, nil
}
