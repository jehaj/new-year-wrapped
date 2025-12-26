package main

import (
	"log"
	"net/http"

	"github.com/jehaj/new-year-wrapped/internal/db"
	"github.com/jehaj/new-year-wrapped/internal/party"
)

func main() {
	database, err := db.Init("wrapped.db")
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	partyService := party.NewService(database)
	partyHandler := party.NewHandler(partyService)

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("POST /parties", partyHandler.CreateParty)
	mux.HandleFunc("POST /parties/{id}/join", partyHandler.JoinParty)
	mux.HandleFunc("GET /parties/{id}/users", partyHandler.GetUsers)
	mux.HandleFunc("POST /parties/{id}/start", partyHandler.StartCompetition)
	mux.HandleFunc("POST /parties/{id}/next", partyHandler.NextRound)
	mux.HandleFunc("GET /parties/{id}/round", partyHandler.GetCurrentRound)
	mux.HandleFunc("GET /parties/{id}/results", partyHandler.GetRoundResults)
	mux.HandleFunc("POST /parties/{id}/guess", partyHandler.SubmitGuess)
	mux.HandleFunc("GET /parties/{id}/leaderboard", partyHandler.GetLeaderboard)

	// UI Routes
	mux.HandleFunc("GET /", partyHandler.IndexPage)
	mux.HandleFunc("GET /parties/{id}", partyHandler.PartyPage)
	mux.HandleFunc("GET /parties/{id}/game", partyHandler.GamePage)

	// UI Action Routes
	mux.HandleFunc("POST /ui/parties/create", partyHandler.UICreateParty)
	mux.HandleFunc("POST /ui/parties/{id}/join", partyHandler.UIJoinParty)
	mux.HandleFunc("POST /ui/parties/{id}/start", partyHandler.UIStartCompetition)
	mux.HandleFunc("POST /ui/parties/{id}/next", partyHandler.UINextRound)
	mux.HandleFunc("POST /ui/parties/{id}/guess", partyHandler.UIGuess)

	// Static Files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
