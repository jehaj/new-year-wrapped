package party_test

import (
	"bytes"
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

	service := party.NewService(database)
	handler := party.NewHandler(service)

	t.Run("Successful creation", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"id":   "party-1",
			"name": "My Party",
		})
		req := httptest.NewRequest("POST", "/parties", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.CreateParty(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rr.Code)
		}
	})
}

func TestHandler_JoinParty(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	defer database.Close()
	_, _ = database.Exec(db.Schema)
	_, _ = database.Exec("INSERT INTO parties (id, name) VALUES (?, ?)", "p1", "Test Party")

	service := party.NewService(database)
	handler := party.NewHandler(service)

	t.Run("Successful join", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":  "Nikolaj",
			"songs": []string{"Song A", "Song B", "Song C"},
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
		body, _ := json.Marshal(map[string]interface{}{
			"name":  "Nikolaj",
			"songs": []string{"Song A", "Song B", "Song C"},
		})
		req := httptest.NewRequest("POST", "/parties/non-existent/join", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.JoinParty(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("Join with wrong number of songs", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":  "Nikolaj",
			"songs": []string{"Song A"},
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
	_, _ = database.Exec("INSERT INTO parties (id, name) VALUES (?, ?)", partyID, "Comp Handler Party")

	// Add a user and songs
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	userID, _ := res.LastInsertId()
	for i := 1; i <= 3; i++ {
		_, _ = database.Exec("INSERT INTO songs (user_id, title) VALUES (?, ?)", userID, fmt.Sprintf("Song %d", i))
	}

	service := party.NewService(database)
	handler := party.NewHandler(service)

	t.Run("Start competition", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/parties/"+partyID+"/start", nil)
		// Mock PathValue if needed, but our handler has a fallback
		rr := httptest.NewRecorder()
		handler.StartCompetition(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("Get current round", func(t *testing.T) {
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
	_, _ = database.Exec("INSERT INTO parties (id, name, started, current_round) VALUES (?, ?, TRUE, 1)", partyID, "Guess Handler Party")

	// Alice owns Song 1
	res, _ := database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Alice")
	aliceID, _ := res.LastInsertId()
	res, _ = database.Exec("INSERT INTO songs (user_id, title, round_number) VALUES (?, ?, 1)", aliceID, "Song 1")
	song1ID, _ := res.LastInsertId()

	// Bob is the guesser
	res, _ = database.Exec("INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, "Bob")
	bobID, _ := res.LastInsertId()

	service := party.NewService(database)
	handler := party.NewHandler(service)

	t.Run("Submit guess", func(t *testing.T) {
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
