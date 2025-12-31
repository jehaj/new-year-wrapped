package party_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jehaj/new-year-wrapped/internal/db"
	"github.com/jehaj/new-year-wrapped/internal/party"
	_ "github.com/mattn/go-sqlite3"
)

func TestHandler_CreateParty(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)

	service := party.NewService(database, nil)
	handler := party.NewHandler(service)

	t.Run("Successful creation", func(t *testing.T) {
		// Given: A valid party name
		// When: A POST request is made to /parties
		// Then: The party is created and a 201 status is returned with id and admin_token
		body, _ := json.Marshal(map[string]string{
			"name": "My Party",
		})
		req := httptest.NewRequest("POST", "/parties", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.CreateParty(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rr.Code)
		}

		var resp map[string]string
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["id"] == "" || resp["admin_token"] == "" {
			t.Errorf("expected id and admin_token in response, got %v", resp)
		}
	})
}

func TestHandler_JoinParty(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token) VALUES (?, ?, ?)", "p1", "Test Party", "token")

	service := party.NewService(database, nil)
	handler := party.NewHandler(service)

	t.Run("Successful join", func(t *testing.T) {
		// Given: An existing party
		// When: A POST request is made to /parties/{id}/join with valid user and songs
		// Then: The user joins the party and a 200 status is returned
		body, _ := json.Marshal(map[string]interface{}{
			"name": "Nikolaj",
			"songs": []party.SongInput{
				{Title: "Song A"}, {Title: "Song B"}, {Title: "Song C"},
			},
		})
		// We'll use a simple way to pass the ID since we aren't using a router yet in the test
		req := httptest.NewRequest("POST", "/parties/p1/join", bytes.NewBuffer(body))
		// In a real app, the router would extract "p1". For the test, we might need to mock the ID extraction or use a router.
		// Let's use a simple router in the handler or just pass it in the test if we use a specific pattern.

		rr := httptest.NewRecorder()

		// We'll need to handle the ID extraction. Let's assume the handler expects it in the URL path.
		handler.JoinParty(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Join non-existent party", func(t *testing.T) {
		// Given: A non-existent party ID
		// When: A POST request is made to /parties/{id}/join
		// Then: A 500 status is returned
		body, _ := json.Marshal(map[string]interface{}{
			"name": "Nikolaj",
			"songs": []party.SongInput{
				{Title: "Song A"}, {Title: "Song B"}, {Title: "Song C"},
			},
		})
		req := httptest.NewRequest("POST", "/parties/non-existent/join", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.JoinParty(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("Join with wrong number of songs", func(t *testing.T) {
		// Given: An existing party
		// When: A POST request is made to /parties/{id}/join with only 1 song
		// Then: A 500 status is returned
		body, _ := json.Marshal(map[string]interface{}{
			"name":  "Nikolaj",
			"songs": []party.SongInput{{Title: "Song A"}},
		})
		req := httptest.NewRequest("POST", "/parties/p1/join", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.JoinParty(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})
}

func TestHandler_Competition(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)
	partyID := "comp-h"
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token) VALUES (?, ?, ?)", partyID, "Comp Handler Party", "token")

	// Add a user and songs
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	userID, _ := res.LastInsertId()
	for i := 1; i <= 3; i++ {
		_, _ = database.Exec("INSERT INTO songs (user_id, title, youtube_id, thumbnail_url) VALUES (?, ?, '', '')", userID, fmt.Sprintf("Song %d", i))
	}

	service := party.NewService(database, nil)
	handler := party.NewHandler(service)

	t.Run("Start competition", func(t *testing.T) {
		// Given: A party with users and songs
		// When: A POST request is made to /parties/{id}/start
		// Then: The competition starts and a 200 status is returned
		req := httptest.NewRequest("POST", "/parties/"+partyID+"/start", nil)
		// Mock PathValue if needed, but our handler has a fallback
		rr := httptest.NewRecorder()
		handler.StartCompetition(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("Get current round", func(t *testing.T) {
		// Given: A started competition
		// When: A GET request is made to /parties/{id}/round
		// Then: The current round and songs are returned with a 200 status
		req := httptest.NewRequest("GET", "/parties/"+partyID+"/round", nil)
		rr := httptest.NewRecorder()
		handler.GetCurrentRound(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var resp struct {
			Round int          `json:"round"`
			Songs []party.Song `json:"songs"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Round != 1 {
			t.Errorf("expected round 1, got %d", resp.Round)
		}
		if len(resp.Songs) != 3 {
			t.Errorf("expected 3 songs, got %d", len(resp.Songs))
		}
	})
}

func TestHandler_Guessing(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)
	partyID := "guess-h"
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token, started, current_round) VALUES (?, ?, ?, TRUE, 1)", partyID, "Guess Handler Party", "token")

	// Alice owns Song 1
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	aliceID, _ := res.LastInsertId()
	res, _ = database.Exec("INSERT INTO songs (user_id, title, youtube_id, thumbnail_url, shuffle_index) VALUES (?, ?, 'yt1', '', 0)", aliceID, "Song 1")
	song1ID, _ := res.LastInsertId()

	// Bob is the guesser
	res, _ = database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Bob")
	bobID, _ := res.LastInsertId()

	service := party.NewService(database, nil)
	handler := party.NewHandler(service)

	t.Run("Submit guess", func(t *testing.T) {
		// Given: A started competition
		// When: A POST request is made to /parties/{id}/guess with a valid guess
		// Then: The guess is recorded and a 200 status is returned
		body, _ := json.Marshal(map[string]int{
			"guesser_id":      int(bobID),
			"song_id":         int(song1ID),
			"guessed_user_id": int(aliceID),
		})
		req := httptest.NewRequest("POST", "/parties/"+partyID+"/guess", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		handler.SubmitGuess(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		// Reveal the round
		handler.NextRound(httptest.NewRecorder(), httptest.NewRequest("POST", "/parties/"+partyID+"/next", nil))
	})

	t.Run("Get leaderboard", func(t *testing.T) {
		// Given: A competition with recorded guesses
		// When: A GET request is made to /parties/{id}/leaderboard after revealing results
		handler.NextRound(httptest.NewRecorder(), httptest.NewRequest("POST", "/parties/"+partyID+"/next", nil))

		req := httptest.NewRequest("GET", "/parties/"+partyID+"/leaderboard", nil)
		rr := httptest.NewRecorder()
		handler.GetLeaderboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var leaderboard []party.LeaderboardEntry
		json.NewDecoder(rr.Body).Decode(&leaderboard)

		found := false
		for _, entry := range leaderboard {
			if entry.UserName == "Bob" && entry.Score == 1 {
				found = true
			}
		}
		if !found {
			t.Error("Bob with score 1 not found in leaderboard")
		}
	})
}

func TestGetUsersHandler(t *testing.T) {
	dbConn, _ := sql.Open("sqlite3", ":memory:")
	defer dbConn.Close()
	dbConn.Exec(db.Schema)

	svc := party.NewService(dbConn, nil)
	h := party.NewHandler(svc)

	partyID, _, _ := svc.CreateParty(context.Background(), "Test Party")
	svc.JoinParty(context.Background(), partyID, "Alice", []party.SongInput{{Title: "S1"}, {Title: "S2"}, {Title: "S3"}})

	req := httptest.NewRequest("GET", "/parties/"+partyID+"/users", nil)
	// Mock path value for Go 1.22+
	req.SetPathValue("id", partyID)
	w := httptest.NewRecorder()

	// Given: A party with a user
	// When: A GET request is made to /parties/{id}/users
	// Then: The list of users is returned with a 200 status
	h.GetUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var users []party.User
	json.NewDecoder(w.Body).Decode(&users)
	if len(users) != 1 || users[0].Name != "Alice" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestHandler_GetLeaderboard(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)
	partyID := "leaderboard-h"
	_, _ = database.Exec("INSERT INTO parties (id, name, admin_token, started, current_round) VALUES (?, ?, ?, TRUE, 1)", partyID, "Leaderboard Handler Party", "token")

	// Alice owns Song 1
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	aliceID, _ := res.LastInsertId()
	res, _ = database.Exec("INSERT INTO songs (user_id, title, youtube_id, thumbnail_url, shuffle_index) VALUES (?, ?, 'yt1', '', 0)", aliceID, "Song 1")
	song1ID, _ := res.LastInsertId()

	// Bob is the guesser
	res, _ = database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Bob")
	bobID, _ := res.LastInsertId()

	// Record a guess
	_, _ = database.Exec("INSERT INTO guesses (guesser_id, song_id, guessed_user_id) VALUES (?, ?, ?)", bobID, song1ID, aliceID)

	service := party.NewService(database, nil)
	handler := party.NewHandler(service)

	t.Run("Get leaderboard", func(t *testing.T) {
		// Given: A competition with recorded guesses
		// When: A GET request is made to /parties/{id}/leaderboard after revealing results
		handler.NextRound(httptest.NewRecorder(), httptest.NewRequest("POST", "/parties/"+partyID+"/next", nil))

		req := httptest.NewRequest("GET", "/parties/"+partyID+"/leaderboard", nil)
		rr := httptest.NewRecorder()
		handler.GetLeaderboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var leaderboard []party.LeaderboardEntry
		json.NewDecoder(rr.Body).Decode(&leaderboard)

		found := false
		for _, entry := range leaderboard {
			if entry.UserName == "Bob" && entry.Score == 1 {
				found = true
			}
		}
		if !found {
			t.Error("Bob with score 1 not found in leaderboard")
		}
	})
}

func TestHandler_GetRoundResults(t *testing.T) {
	dbConn, _ := sql.Open("sqlite3", ":memory:")
	defer dbConn.Close()
	dbConn.Exec(db.Schema)

	svc := party.NewService(dbConn, nil)
	h := party.NewHandler(svc)

	partyID, _, _ := svc.CreateParty(context.Background(), "Test Party")
	svc.JoinParty(context.Background(), partyID, "Alice", []party.SongInput{{Title: "S1"}, {Title: "S2"}, {Title: "S3"}})
	svc.StartCompetition(context.Background(), partyID)
	svc.NextRound(context.Background(), partyID)

	req := httptest.NewRequest("GET", "/parties/"+partyID+"/results?round=1", nil)
	req.SetPathValue("id", partyID)
	w := httptest.NewRecorder()

	// Given: A started competition with a revealed round
	// When: A GET request is made to /parties/{id}/results for that round
	// Then: The round results are returned with a 200 status
	h.GetRoundResults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var results []party.SongResult
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 3 { // Alice has 3 songs, default songs_per_round is 5, so all 3 should be in round 1
		t.Errorf("expected 3 results, got %d", len(results))
	}
}
