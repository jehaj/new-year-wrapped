package party

import (
	"encoding/json"
	"net/http"
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
