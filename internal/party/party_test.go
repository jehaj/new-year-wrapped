package party_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/jehaj/new-year-wrapped/internal/party"
	_ "github.com/mattn/go-sqlite3"
)

func TestJoinParty(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Setup schema
	_, err = db.Exec(`
		CREATE TABLE parties (id TEXT PRIMARY KEY, name TEXT, started BOOLEAN DEFAULT FALSE, current_round INTEGER DEFAULT 0);
		CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, party_id TEXT, name TEXT);
		CREATE TABLE songs (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, title TEXT, round_number INTEGER DEFAULT 0);
		CREATE TABLE guesses (id INTEGER PRIMARY KEY AUTOINCREMENT, guesser_id INTEGER NOT NULL, song_id INTEGER NOT NULL, guessed_user_id INTEGER NOT NULL, UNIQUE(guesser_id, song_id));
	`)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Create a mocked party
	partyID := "test-party"
	_, err = db.Exec("INSERT INTO parties (id, name) VALUES (?, ?)", partyID, "Test Party")
	if err != nil {
		t.Fatalf("failed to insert party: %v", err)
	}

	service := party.NewService(db)

	t.Run("Join with valid data", func(t *testing.T) {
		userName := "Nikolaj"
		songs := []string{"Song 1", "Song 2", "Song 3"}

		err := service.JoinParty(context.Background(), partyID, userName, songs)
		if err != nil {
			t.Errorf("JoinParty failed: %v", err)
		}

		// Verify user was created
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE party_id = ? AND name = ?", partyID, userName).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query users: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 user, got %d", count)
		}

		// Verify songs were created
		var songCount int
		err = db.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.name = ?", userName).Scan(&songCount)
		if err != nil {
			t.Fatalf("failed to query songs: %v", err)
		}
		if songCount != 3 {
			t.Errorf("expected 3 songs, got %d", songCount)
		}
	})

	t.Run("Join with too many songs should fail", func(t *testing.T) {
		err := service.JoinParty(context.Background(), partyID, "BadUser", []string{"1", "2", "3", "4"})
		if err == nil {
			t.Error("expected error for 4 songs, got nil")
		}
	})

	t.Run("Create Party", func(t *testing.T) {
		id := "new-party"
		name := "New Year 2025"
		err := service.CreateParty(context.Background(), id, name)
		if err != nil {
			t.Errorf("CreateParty failed: %v", err)
		}

		var dbName string
		err = db.QueryRow("SELECT name FROM parties WHERE id = ?", id).Scan(&dbName)
		if err != nil {
			t.Fatalf("failed to query party: %v", err)
		}
		if dbName != name {
			t.Errorf("expected name %s, got %s", name, dbName)
		}
	})
}

func TestStartCompetition(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_, _ = db.Exec(`
		CREATE TABLE parties (id TEXT PRIMARY KEY, name TEXT, started BOOLEAN DEFAULT FALSE, current_round INTEGER DEFAULT 0);
		CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, party_id TEXT, name TEXT);
		CREATE TABLE songs (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, title TEXT, round_number INTEGER DEFAULT 0);
		CREATE TABLE guesses (id INTEGER PRIMARY KEY AUTOINCREMENT, guesser_id INTEGER NOT NULL, song_id INTEGER NOT NULL, guessed_user_id INTEGER NOT NULL, UNIQUE(guesser_id, song_id));
	`)

	partyID := "comp-party"
	_, _ = db.Exec("INSERT INTO parties (id, name) VALUES (?, ?)", partyID, "Comp Party")

	service := party.NewService(db)

	// Add some users and songs
	users := []string{"Alice", "Bob", "Charlie", "Dave"}
	for _, name := range users {
		res, _ := db.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, name)
		userID, _ := res.LastInsertId()
		for j := 1; j <= 3; j++ {
			_, _ = db.Exec("INSERT INTO songs (user_id, title) VALUES (?, ?)", userID, fmt.Sprintf("%s Song %d", name, j))
		}
	}

	t.Run("Start competition shuffles and assigns rounds", func(t *testing.T) {
		err := service.StartCompetition(context.Background(), partyID)
		if err != nil {
			t.Fatalf("StartCompetition failed: %v", err)
		}

		var started bool
		var currentRound int
		err = db.QueryRow("SELECT started, current_round FROM parties WHERE id = ?", partyID).Scan(&started, &currentRound)
		if err != nil {
			t.Fatalf("failed to query party: %v", err)
		}
		if !started {
			t.Error("expected started to be true")
		}
		if currentRound != 1 {
			t.Errorf("expected current_round to be 1, got %d", currentRound)
		}

		// Check if songs have round numbers assigned
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.party_id = ? AND round_number > 0", partyID).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query songs: %v", err)
		}
		if count != 12 { // 4 users * 3 songs
			t.Errorf("expected 12 songs with round numbers, got %d", count)
		}

		// Check round distribution (5 songs per round)
		// Round 1: 5, Round 2: 5, Round 3: 2
		var r1, r2, r3 int
		_ = db.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.party_id = ? AND round_number = 1", partyID).Scan(&r1)
		_ = db.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.party_id = ? AND round_number = 2", partyID).Scan(&r2)
		_ = db.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.party_id = ? AND round_number = 3", partyID).Scan(&r3)

		if r1 != 5 || r2 != 5 || r3 != 2 {
			t.Errorf("unexpected round distribution: r1=%d, r2=%d, r3=%d", r1, r2, r3)
		}
	})
}

func TestGuessingAndLeaderboard(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_, _ = db.Exec(`
		CREATE TABLE parties (id TEXT PRIMARY KEY, name TEXT, started BOOLEAN DEFAULT FALSE, current_round INTEGER DEFAULT 0);
		CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, party_id TEXT, name TEXT);
		CREATE TABLE songs (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, title TEXT, round_number INTEGER DEFAULT 0);
		CREATE TABLE guesses (id INTEGER PRIMARY KEY AUTOINCREMENT, guesser_id INTEGER NOT NULL, song_id INTEGER NOT NULL, guessed_user_id INTEGER NOT NULL, UNIQUE(guesser_id, song_id));
	`)

	partyID := "guess-party"
	_, _ = db.Exec("INSERT INTO parties (id, name, started, current_round) VALUES (?, ?, TRUE, 1)", partyID, "Guess Party")

	service := party.NewService(db)

	// Alice owns Song 1
	res, _ := db.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	aliceID, _ := res.LastInsertId()
	res, _ = db.Exec("INSERT INTO songs (user_id, title, round_number) VALUES (?, ?, 1)", aliceID, "Song 1")
	song1ID, _ := res.LastInsertId()

	// Bob is the guesser
	res, _ = db.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Bob")
	bobID, _ := res.LastInsertId()

	t.Run("Submit correct guess", func(t *testing.T) {
		err := service.SubmitGuess(context.Background(), int(bobID), int(song1ID), int(aliceID))
		if err != nil {
			t.Fatalf("SubmitGuess failed: %v", err)
		}

		// Reveal the round
		err = service.NextRound(context.Background(), partyID)
		if err != nil {
			t.Fatalf("NextRound failed: %v", err)
		}

		leaderboard, err := service.GetLeaderboard(context.Background(), partyID, 0)
		if err != nil {
			t.Fatalf("GetLeaderboard failed: %v", err)
		}

		// Bob should have 1 point
		found := false
		for _, entry := range leaderboard {
			if entry.UserName == "Bob" {
				found = true
				if entry.Score != 1 {
					t.Errorf("expected score 1 for Bob, got %d", entry.Score)
				}
			}
		}
		if !found {
			t.Error("Bob not found in leaderboard")
		}
	})
}
