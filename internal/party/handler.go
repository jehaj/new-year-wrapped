package party

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	service   *Service
	templates *template.Template
}

func NewHandler(service *Service) *Handler {
	tmpl, _ := template.ParseGlob("templates/*.html")
	return &Handler{
		service:   service,
		templates: tmpl,
	}
}

// UI Handlers

func (h *Handler) IndexPage(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", nil)
}

func (h *Handler) PartyPage(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	partyID := h.getPartyID(r)
	if partyID == "" {
		partyID = r.URL.Query().Get("id")
	}
	if partyID == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	userName := r.URL.Query().Get("user")
	adminToken := r.URL.Query().Get("admin_token")

	started, _, err := h.service.GetPartyState(r.Context(), partyID)
	if err == nil && started {
		http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?user=%s&admin_token=%s", partyID, userName, adminToken), http.StatusSeeOther)
		return
	}

	users, _ := h.service.GetUsers(r.Context(), partyID)
	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)

	data := map[string]interface{}{
		"Party": map[string]string{
			"ID":   partyID,
			"Name": "Party " + partyID,
		},
		"Users":      users,
		"UserJoined": userName != "",
		"UserName":   userName,
		"AdminToken": adminToken,
		"IsAdmin":    isAdmin,
	}

	h.templates.ExecuteTemplate(w, "layout", data)
}

func (h *Handler) GamePage(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	partyID := h.getPartyID(r)
	userName := r.URL.Query().Get("user")
	adminToken := r.URL.Query().Get("admin_token")

	started, currentRound, err := h.service.GetPartyState(r.Context(), partyID)
	if err != nil || !started {
		http.Redirect(w, r, "/parties/"+partyID, http.StatusSeeOther)
		return
	}

	songs, _ := h.service.GetRoundSongs(r.Context(), partyID, currentRound)
	users, _ := h.service.GetUsers(r.Context(), partyID)
	leaderboard, _ := h.service.GetLeaderboard(r.Context(), partyID, 0)
	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)

	var previousResults []SongResult
	if currentRound > 1 {
		previousResults, _ = h.service.GetRoundResults(r.Context(), partyID, currentRound-1)
	}

	data := map[string]interface{}{
		"Party": map[string]string{
			"ID":   partyID,
			"Name": "Party " + partyID,
		},
		"CurrentRound":    currentRound,
		"Songs":           songs,
		"Users":           users,
		"UserName":        userName,
		"AdminToken":      adminToken,
		"Leaderboard":     leaderboard,
		"PreviousResults": previousResults,
		"IsAdmin":         isAdmin,
	}

	h.templates.ExecuteTemplate(w, "layout", data)
}

// UI Action Handlers (Form Submissions)

func (h *Handler) UICreateParty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	id, adminToken, err := h.service.CreateParty(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/parties/%s?admin_token=%s", id, adminToken), http.StatusSeeOther)
}

func (h *Handler) UIJoinParty(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	userName := r.FormValue("user_name")
	adminToken := r.FormValue("admin_token")
	songs := []string{
		r.FormValue("song1"),
		r.FormValue("song2"),
		r.FormValue("song3"),
	}

	if err := h.service.JoinParty(r.Context(), partyID, userName, songs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/parties/%s?user=%s&admin_token=%s", partyID, userName, adminToken), http.StatusSeeOther)
}

func (h *Handler) UIStartCompetition(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	adminToken := r.FormValue("admin_token")

	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)
	if !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.service.StartCompetition(r.Context(), partyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?admin_token=%s", partyID, adminToken), http.StatusSeeOther)
}

func (h *Handler) UINextRound(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	adminToken := r.FormValue("admin_token")

	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)
	if !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.service.NextRound(r.Context(), partyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?admin_token=%s", partyID, adminToken), http.StatusSeeOther)
}

func (h *Handler) UIGuess(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	userName := r.FormValue("user_name")
	adminToken := r.FormValue("admin_token")
	songID, _ := strconv.Atoi(r.FormValue("song_id"))
	ownerName := r.FormValue("owner_name")

	users, _ := h.service.GetUsers(r.Context(), partyID)
	var guesserID, ownerID int
	for _, u := range users {
		if u.Name == userName {
			guesserID = u.ID
		}
		if u.Name == ownerName {
			ownerID = u.ID
		}
	}

	if guesserID != 0 && ownerID != 0 {
		h.service.SubmitGuess(r.Context(), guesserID, songID, ownerID)
	}

	http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?user=%s&admin_token=%s", partyID, userName, adminToken), http.StatusSeeOther)
}

func (h *Handler) CreateParty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, adminToken, err := h.service.CreateParty(r.Context(), req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":          id,
		"admin_token": adminToken,
	})
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
