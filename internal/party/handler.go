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
	partyID := r.PathValue("id")
	if partyID == "" {
		// Fallback for tests or older mux if needed, but PathValue is preferred
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			partyID = parts[2]
		}
	}

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
