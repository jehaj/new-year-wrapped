package party_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/jehaj/new-year-wrapped/internal/db"
	"github.com/jehaj/new-year-wrapped/internal/party"
	_ "github.com/mattn/go-sqlite3"
)

func TestJoinParty(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Setup schema
	_, err = database.Exec(db.Schema)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Create a mocked party
	partyID := "test-party"
	_, err = database.Exec("INSERT INTO parties (id, name, admin_token) VALUES (?, ?, ?)", partyID, "Test Party", "test-token")
	if err != nil {
		t.Fatalf("failed to insert party: %v", err)
	}

	service := party.NewService(database, nil)

	t.Run("Join with valid data", func(t *testing.T) {
		// Given: A party exists
		// When: A user joins with 3 songs
		// Then: The user and songs are created in the database
		userName := "Nikolaj"
		songs := []party.SongInput{
			{Title: "Song 1", YouTubeID: "id1", ThumbnailURL: "thumb1"},
			{Title: "Song 2", YouTubeID: "id2", ThumbnailURL: "thumb2"},
			{Title: "Song 3", YouTubeID: "id3", ThumbnailURL: "thumb3"},
		}

		err := service.JoinParty(context.Background(), partyID, userName, songs)
		if err != nil {
			t.Errorf("JoinParty failed: %v", err)
		}

		// Verify user was created
		var count int
		err = database.QueryRow("SELECT COUNT(*) FROM users WHERE party_id = ? AND name = ?", partyID, userName).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query users: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 user, got %d", count)
		}

		// Verify songs were created
		var songCount int
		err = database.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.name = ?", userName).Scan(&songCount)
		if err != nil {
			t.Fatalf("failed to query songs: %v", err)
		}
		if songCount != 3 {
			t.Errorf("expected 3 songs, got %d", songCount)
		}
	})

	t.Run("Join with too many songs should fail", func(t *testing.T) {
		// Given: A party exists
		// When: A user tries to join with 4 songs
		// Then: An error is returned
		err := service.JoinParty(context.Background(), partyID, "BadUser", []party.SongInput{
			{Title: "1"}, {Title: "2"}, {Title: "3"}, {Title: "4"},
		})
		if err == nil {
			t.Error("expected error for 4 songs, got nil")
		}
	})

	t.Run("Create Party", func(t *testing.T) {
		// Given: A database connection
		// When: A new party is created
		// Then: The party exists in the database with the correct name and a token is returned
		name := "New Year 2025"
		id, token, err := service.CreateParty(context.Background(), name)
		if err != nil {
			t.Errorf("CreateParty failed: %v", err)
		}

		if id == "" || token == "" {
			t.Errorf("expected non-empty id and token, got id=%s, token=%s", id, token)
		}

		var dbName string
		err = database.QueryRow("SELECT name FROM parties WHERE id = ?", id).Scan(&dbName)
		if err != nil {
			t.Fatalf("failed to query party: %v", err)
		}
		if dbName != name {
			t.Errorf("expected name %s, got %s", name, dbName)
		}

		// Verify admin token
		isAdmin, err := service.VerifyAdmin(context.Background(), id, token)
		if err != nil {
			t.Fatalf("VerifyAdmin failed: %v", err)
		}
		if !isAdmin {
			t.Error("expected token to be valid admin token")
		}
	})
}

func TestStartCompetition(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)

	partyID := "comp-party"
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token) VALUES (?, ?, ?)", partyID, "Comp Party", "token")

	service := party.NewService(database, nil)

	// Add some users and songs
	users := []string{"Alice", "Bob", "Charlie", "Dave"}
	for _, name := range users {
		res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, name)
		userID, _ := res.LastInsertId()
		for j := 1; j <= 3; j++ {
			_, _ = database.Exec("INSERT INTO songs (user_id, title) VALUES (?, ?)", userID, fmt.Sprintf("%s Song %d", name, j))
		}
	}

	t.Run("Start competition shuffles and assigns rounds", func(t *testing.T) {
		// Given: A party with multiple users and songs
		// When: The competition is started
		// Then: The party is marked as started, current round is 1, and all songs have shuffle indices
		err := service.StartCompetition(context.Background(), partyID)
		if err != nil {
			t.Fatalf("StartCompetition failed: %v", err)
		}

		var started bool
		var currentRound int
		err = database.QueryRow("SELECT started, current_round FROM parties WHERE id = ?", partyID).Scan(&started, &currentRound)
		if err != nil {
			t.Fatalf("failed to query party: %v", err)
		}
		if !started {
			t.Error("expected started to be true")
		}
		if currentRound != 1 {
			t.Errorf("expected current_round to be 1, got %d", currentRound)
		}

		// Check if songs have shuffle indices assigned
		var count int
		err = database.QueryRow("SELECT COUNT(*) FROM songs JOIN users ON songs.user_id = users.id WHERE users.party_id = ? AND shuffle_index >= 0", partyID).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query songs: %v", err)
		}
		if count != 12 { // 4 users * 3 songs
			t.Errorf("expected 12 songs with shuffle indices, got %d", count)
		}
	})
}

func TestGuessingAndLeaderboard(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)

	partyID := "guess-party"
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token, started, current_round) VALUES (?, ?, ?, TRUE, 1)", partyID, "Guess Party", "token")

	service := party.NewService(database, nil)

	// Alice owns Song 1
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	aliceID, _ := res.LastInsertId()
	res, _ = database.Exec("INSERT INTO songs (user_id, title, youtube_id, shuffle_index) VALUES (?, ?, ?, 0)", aliceID, "Song 1", "yt1")
	song1ID, _ := res.LastInsertId()

	// Bob is the guesser
	res, _ = database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Bob")
	bobID, _ := res.LastInsertId()

	t.Run("Submit correct guess", func(t *testing.T) {
		// Given: A started competition with a song owned by Alice and a guesser Bob
		// When: Bob submits a correct guess for Alice's song and the round is revealed
		// Then: Bob has 1 point on the leaderboard
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

func TestDuplicateSongGuessing(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)

	partyID := "dup-party"
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token, started, current_round, show_results, songs_per_round) VALUES (?, ?, ?, TRUE, 1, TRUE, 5)", partyID, "Dup Party", "token")

	service := party.NewService(database, nil)

	// Alice owns "Song X"
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	aliceID, _ := res.LastInsertId()
	_, _ = database.Exec("INSERT INTO songs (user_id, title, youtube_id, thumbnail_url, shuffle_index) VALUES (?, ?, ?, ?, 0)", aliceID, "Song X", "yt1", "thumb1")

	// Bob also owns "Song X"
	res, _ = database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Bob")
	bobID, _ := res.LastInsertId()
	res, _ = database.Exec("INSERT INTO songs (user_id, title, youtube_id, thumbnail_url, shuffle_index) VALUES (?, ?, ?, ?, 1)", bobID, "Song X", "yt1", "thumb2")
	song2ID, _ := res.LastInsertId()

	// Charlie is the guesser
	res, _ = database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Charlie")
	charlieID, _ := res.LastInsertId()

	t.Run("Guessing Alice for Bob's song should be correct", func(t *testing.T) {
		// Charlie guesses Alice for Bob's instance of the song (song2ID)
		err := service.SubmitGuess(context.Background(), int(charlieID), int(song2ID), int(aliceID))
		if err != nil {
			t.Fatalf("SubmitGuess failed: %v", err)
		}

		leaderboard, err := service.GetLeaderboard(context.Background(), partyID, 0)
		if err != nil {
			t.Fatalf("GetLeaderboard failed: %v", err)
		}

		var charlieScore int
		for _, entry := range leaderboard {
			if entry.UserName == "Charlie" {
				charlieScore = entry.Score
			}
		}

		if charlieScore != 1 {
			t.Errorf("expected Charlie to have 1 point, got %d", charlieScore)
		}
	})

	t.Run("GetRoundResults shows both owners", func(t *testing.T) {
		results, err := service.GetRoundResults(context.Background(), partyID, 1)
		if err != nil {
			t.Fatalf("GetRoundResults failed: %v", err)
		}

		found := false
		for _, r := range results {
			if r.YouTubeID == "yt1" {
				if r.OwnerName == "Alice, Bob" || r.OwnerName == "Bob, Alice" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("expected 'Alice, Bob' as owners for song yt1, got results: %+v", results)
		}
	})
}

func TestGetUsers(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()

	if _, err := dbConn.Exec(db.Schema); err != nil {
		t.Fatal(err)
	}

	svc := party.NewService(dbConn, nil)
	ctx := context.Background()

	partyID, _, _ := svc.CreateParty(ctx, "Test Party")
	svc.JoinParty(ctx, partyID, "Alice", []party.SongInput{{Title: "S1"}, {Title: "S2"}, {Title: "S3"}})
	svc.JoinParty(ctx, partyID, "Bob", []party.SongInput{{Title: "S4"}, {Title: "S5"}, {Title: "S6"}})

	// Given: A party with two users
	// When: GetUsers is called
	// Then: Both users are returned
	users, err := svc.GetUsers(ctx, partyID)
	if err != nil {
		t.Fatalf("failed to get users: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestGetRoundResults(t *testing.T) {
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()

	if _, err := dbConn.Exec(db.Schema); err != nil {
		t.Fatal(err)
	}

	svc := party.NewService(dbConn, nil)
	ctx := context.Background()

	partyID, _, _ := svc.CreateParty(ctx, "Test Party")
	svc.JoinParty(ctx, partyID, "Alice", []party.SongInput{{Title: "S1"}, {Title: "S2"}, {Title: "S3"}})
	svc.JoinParty(ctx, partyID, "Bob", []party.SongInput{{Title: "S4"}, {Title: "S5"}, {Title: "S6"}})

	// Start competition (shuffles songs)
	if err := svc.StartCompetition(ctx, partyID); err != nil {
		t.Fatal(err)
	}

	// Move to next round to reveal 1
	if err := svc.NextRound(ctx, partyID); err != nil {
		t.Fatal(err)
	}

	// Given: A started competition with songs and round 1 revealed
	// When: GetRoundResults is called for round 1
	// Then: The songs for round 1 are returned with their owner names
	results, err := svc.GetRoundResults(ctx, partyID, 1)
	if err != nil {
		t.Fatalf("failed to get round results: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("expected 5 songs in round 1, got %d", len(results))
	}

	for _, res := range results {
		if res.OwnerName == "" {
			t.Error("expected owner name to be populated")
		}
	}
}
