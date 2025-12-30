package party

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
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

	started, _, _, err := h.service.GetPartyState(r.Context(), partyID)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if started {
		http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?user=%s&admin_token=%s", partyID, userName, adminToken), http.StatusSeeOther)
		return
	}

	partyName, _ := h.service.GetPartyName(r.Context(), partyID)
	users, _ := h.service.GetUsers(r.Context(), partyID)
	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)

	data := map[string]interface{}{
		"Party": map[string]string{
			"ID":   partyID,
			"Name": partyName,
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

	started, currentRound, showResults, err := h.service.GetPartyState(r.Context(), partyID)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !started {
		http.Redirect(w, r, "/parties/"+partyID, http.StatusSeeOther)
		return
	}

	partyName, _ := h.service.GetPartyName(r.Context(), partyID)
	songs, _ := h.service.GetRoundSongs(r.Context(), partyID, currentRound)
	totalSongs, _ := h.service.GetTotalSongs(r.Context(), partyID)
	users, _ := h.service.GetUsers(r.Context(), partyID)

	var leaderboard []LeaderboardEntry
	var previousResults []SongResult

	if showResults {
		leaderboard, _ = h.service.GetLeaderboard(r.Context(), partyID, currentRound)
		previousResults, _ = h.service.GetRoundResults(r.Context(), partyID, currentRound)
	} else {
		leaderboard, _ = h.service.GetLeaderboard(r.Context(), partyID, currentRound-1)
		if currentRound > 1 {
			previousResults, _ = h.service.GetRoundResults(r.Context(), partyID, currentRound-1)
		}
	}

	globalLeaderboard, _ := h.service.GetLeaderboard(r.Context(), partyID, 0)
	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)
	userGuesses, _ := h.service.GetUserGuesses(r.Context(), partyID, userName)

	// Check if game is over
	gameOver := false
	if !showResults && len(songs) == 0 {
		gameOver = true
		// Fetch all songs for the final reveal
		previousResults, _ = h.service.GetPartySongs(r.Context(), partyID)
	}

	data := map[string]interface{}{
		"Party": map[string]string{
			"ID":   partyID,
			"Name": partyName,
		},
		"Started":           true,
		"CurrentRound":      currentRound,
		"ShowResults":       showResults,
		"GameOver":          gameOver,
		"Songs":             songs,
		"TotalSongs":        totalSongs,
		"Users":             users,
		"UserName":          userName,
		"AdminToken":        adminToken,
		"Leaderboard":       leaderboard,
		"GlobalLeaderboard": globalLeaderboard,
		"PreviousResults":   previousResults,
		"IsAdmin":           isAdmin,
		"UserGuesses":       userGuesses,
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

func (h *Handler) UIPartyRedirect(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/parties/"+id, http.StatusSeeOther)
}

func (h *Handler) UIJoinParty(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	userName := r.FormValue("user_name")
	adminToken := r.FormValue("admin_token")
	songs := []SongInput{
		{Title: r.FormValue("song1"), YouTubeID: r.FormValue("song1_id"), ThumbnailURL: r.FormValue("song1_thumb")},
		{Title: r.FormValue("song2"), YouTubeID: r.FormValue("song2_id"), ThumbnailURL: r.FormValue("song2_thumb")},
		{Title: r.FormValue("song3"), YouTubeID: r.FormValue("song3_id"), ThumbnailURL: r.FormValue("song3_thumb")},
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
	userName := r.FormValue("user_name")

	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)
	if !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.service.StartCompetition(r.Context(), partyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?user=%s&admin_token=%s", partyID, userName, adminToken), http.StatusSeeOther)
}

func (h *Handler) UINextRound(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	adminToken := r.FormValue("admin_token")
	userName := r.FormValue("user_name")

	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)
	if !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.service.NextRound(r.Context(), partyID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/parties/%s/game?user=%s&admin_token=%s", partyID, userName, adminToken), http.StatusSeeOther)
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
		Name  string      `json:"name"`
		Songs []SongInput `json:"songs"`
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

func (h *Handler) SearchSongs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode([]SongInput{})
		return
	}

	songs, err := h.service.SearchYouTubeMusic(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(songs)
}

func (h *Handler) QRCode(w http.ResponseWriter, r *http.Request) {
	partyID := h.getPartyID(r)
	if partyID == "" {
		http.Error(w, "Missing party ID", http.StatusBadRequest)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	joinURL := fmt.Sprintf("%s://%s/parties/%s", scheme, r.Host, partyID)

	qrc, err := qrcode.New(joinURL)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	wr := standard.NewWithWriter(nopCloser{w})

	if err := qrc.Save(wr); err != nil {
		return
	}
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

func (h *Handler) SongListPage(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	partyID := h.getPartyID(r)
	adminToken := r.URL.Query().Get("admin_token")
	userName := r.URL.Query().Get("user")

	isAdmin, _ := h.service.VerifyAdmin(r.Context(), partyID, adminToken)
	if !isAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	partyName, _ := h.service.GetPartyName(r.Context(), partyID)
	songs, _ := h.service.GetPartySongs(r.Context(), partyID)

	data := map[string]interface{}{
		"Party": map[string]string{
			"ID":   partyID,
			"Name": partyName,
		},
		"Songs":      songs,
		"AdminToken": adminToken,
		"UserName":   userName,
		"IsSongList": true,
	}

	h.templates.ExecuteTemplate(w, "layout", data)
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

	started, currentRound, _, err := h.service.GetPartyState(r.Context(), partyID)
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
