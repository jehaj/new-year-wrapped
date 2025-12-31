package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jehaj/new-year-wrapped/internal/db"
	"github.com/jehaj/new-year-wrapped/internal/party"
)

func main() {
	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	// Setup logging to file
	logFile, err := os.OpenFile("data/party.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	partyLogger := log.New(logFile, "PARTY: ", log.LstdFlags)

	database, err := db.Init("data/wrapped.db")
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	partyService := party.NewService(database, partyLogger)
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
	mux.HandleFunc("GET /api/search", partyHandler.SearchSongs)

	// UI Routes
	mux.HandleFunc("GET /", partyHandler.IndexPage)
	mux.HandleFunc("GET /parties", partyHandler.UIPartyRedirect)
	mux.HandleFunc("GET /parties/{id}", partyHandler.PartyPage)
	mux.HandleFunc("GET /parties/{id}/game", partyHandler.GamePage)
	mux.HandleFunc("GET /parties/{id}/song_list", partyHandler.SongListPage)
	mux.HandleFunc("GET /parties/{id}/qrcode", partyHandler.QRCode)

	// UI Action Routes
	mux.HandleFunc("POST /ui/parties/create", partyHandler.UICreateParty)
	mux.HandleFunc("POST /ui/parties/{id}/join", partyHandler.UIJoinParty)
	mux.HandleFunc("POST /ui/parties/{id}/start", partyHandler.UIStartCompetition)
	mux.HandleFunc("POST /ui/parties/{id}/next", partyHandler.UINextRound)
	mux.HandleFunc("POST /ui/parties/{id}/guess", partyHandler.UIGuess)

	// Static Files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port
	log.Println("Server starting on http://" + addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
