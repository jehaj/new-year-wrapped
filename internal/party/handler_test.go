package party_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jehaj/new-year-wrapped/internal/party"
	_ "github.com/mattn/go-sqlite3"
)

func TestHandler_CreateParty(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_, _ = db.Exec("CREATE TABLE parties (id TEXT PRIMARY KEY, name TEXT)")

	service := party.NewService(db)
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
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_, _ = db.Exec(`
		CREATE TABLE parties (id TEXT PRIMARY KEY, name TEXT);
		CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, party_id TEXT, name TEXT);
		CREATE TABLE songs (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, title TEXT);
	`)
	_, _ = db.Exec("INSERT INTO parties (id, name) VALUES (?, ?)", "p1", "Test Party")

	service := party.NewService(db)
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
