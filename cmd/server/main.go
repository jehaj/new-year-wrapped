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

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
