package party_test

import (
	"context"
	"database/sql"
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
		CREATE TABLE parties (id TEXT PRIMARY KEY, name TEXT);
		CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, party_id TEXT, name TEXT);
		CREATE TABLE songs (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, title TEXT);
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
