package party

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateParty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.CreateParty(r.Context(), req.ID, req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) JoinParty(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name  string   `json:"name"`
		Songs []string `json:"songs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.JoinParty(r.Context(), partyID, req.Name, req.Songs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) StartCompetition(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	if err := h.service.StartCompetition(r.Context(), partyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetCurrentRound(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	started, currentRound, err := h.service.GetPartyState(r.Context(), partyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !started {
		http.Error(w, "competition not started", http.StatusBadRequest)
		return
	}

	songs, err := h.service.GetRoundSongs(r.Context(), partyID, currentRound)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"round": currentRound,
		"songs": songs,
	})
}

func (h *Handler) SubmitGuess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuesserID     int `json:"guesser_id"`
		SongID        int `json:"song_id"`
		GuessedUserID int `json:"guessed_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.SubmitGuess(r.Context(), req.GuesserID, req.SongID, req.GuessedUserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	roundStr := r.URL.Query().Get("round")
	round := 0
	if roundStr != "" {
		var err error
		round, err = strconv.Atoi(roundStr)
		if err != nil {
			http.Error(w, "invalid round", http.StatusBadRequest)
			return
		}
	}

	leaderboard, err := h.service.GetLeaderboard(r.Context(), partyID, round)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(leaderboard)
}

func (h *Handler) NextRound(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	if err := h.service.NextRound(r.Context(), partyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	users, err := h.service.GetUsers(r.Context(), partyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(users)
}

func (h *Handler) GetRoundResults(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	roundStr := r.URL.Query().Get("round")
	if roundStr == "" {
		http.Error(w, "missing round", http.StatusBadRequest)
		return
	}

	round, err := strconv.Atoi(roundStr)
	if err != nil {
		http.Error(w, "invalid round", http.StatusBadRequest)
		return
	}

	results, err := h.service.GetRoundResults(r.Context(), partyID, round)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(results)
}

func (h *Handler) getPartyID(r *http.Request) string {
	id := r.PathValue("id")
	if id != "" {
		return id
	}
	// Fallback for tests
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
