package party

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type SongInput struct {
	Title        string `json:"title"`
	YouTubeID    string `json:"youtube_id"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type Service struct {
	db     *sql.DB
	logger *log.Logger
}

func NewService(db *sql.DB, logger *log.Logger) *Service {
	return &Service{db: db, logger: logger}
}

func (s *Service) log(partyID string, format string, v ...interface{}) {
	if s.logger != nil {
		msg := fmt.Sprintf(format, v...)
		s.logger.Printf("[%s] %s", partyID, msg)
	}
}

func (s *Service) JoinParty(ctx context.Context, partyID string, userName string, songs []SongInput) error {
	s.log(partyID, "User %s joining with %d songs", userName, len(songs))
	if len(songs) != 3 {
		return fmt.Errorf("exactly 3 songs are required, got %d", len(songs))
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if party exists
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM parties WHERE id = ?)", partyID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("party %s does not exist", partyID)
	}

	// Create user
	res, err := tx.ExecContext(ctx, "INSERT INTO users (party_id, name) VALUES (?, ?)", partyID, userName)
	if err != nil {
		return err
	}

	userID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	// Create songs
	for _, song := range songs {
		_, err = tx.ExecContext(ctx, "INSERT INTO songs (user_id, title, youtube_id, thumbnail_url) VALUES (?, ?, ?, ?)", userID, song.Title, song.YouTubeID, song.ThumbnailURL)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) generateRandomString(n int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (s *Service) CreateParty(ctx context.Context, name string) (id string, adminToken string, err error) {
	id = s.generateRandomString(6)
	adminToken = s.generateRandomString(12)

	s.log(id, "Creating party: %s", name)
	_, err = s.db.ExecContext(ctx, "INSERT INTO parties (id, name, admin_token) VALUES (?, ?, ?)", id, name, adminToken)
	return id, adminToken, err
}

func (s *Service) VerifyAdmin(ctx context.Context, partyID, token string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM parties WHERE id = ? AND admin_token = ?)", partyID, token).Scan(&exists)
	return exists, err
}

func (s *Service) StartCompetition(ctx context.Context, partyID string) error {
	s.log(partyID, "Starting competition")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get all songs for the party
	rows, err := tx.QueryContext(ctx, `
		SELECT songs.id 
		FROM songs 
		JOIN users ON songs.user_id = users.id 
		WHERE users.party_id = ?`, partyID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var songIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		songIDs = append(songIDs, id)
	}

	if len(songIDs) == 0 {
		return fmt.Errorf("no songs found for party %s", partyID)
	}

	// Shuffle songs
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(songIDs), func(i, j int) {
		songIDs[i], songIDs[j] = songIDs[j], songIDs[i]
	})

	// Assign shuffle index
	for i, id := range songIDs {
		_, err = tx.ExecContext(ctx, "UPDATE songs SET shuffle_index = ? WHERE id = ?", i, id)
		if err != nil {
			return err
		}
	}

	// Update party state
	_, err = tx.ExecContext(ctx, "UPDATE parties SET started = TRUE, current_round = 1 WHERE id = ?", partyID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

type Song struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	YouTubeID    string `json:"youtube_id"`
	ThumbnailURL string `json:"thumbnail_url"`
}

func (s *Service) GetRoundSongs(ctx context.Context, partyID string, round int) ([]Song, error) {
	var songsPerRound int
	err := s.db.QueryRowContext(ctx, "SELECT songs_per_round FROM parties WHERE id = ?", partyID).Scan(&songsPerRound)
	if err != nil {
		return nil, err
	}

	startIndex := (round - 1) * songsPerRound
	endIndex := round*songsPerRound - 1

	rows, err := s.db.QueryContext(ctx, `
		SELECT songs.id, songs.title, songs.youtube_id, songs.thumbnail_url 
		FROM songs 
		JOIN users ON songs.user_id = users.id 
		WHERE users.party_id = ? AND songs.shuffle_index BETWEEN ? AND ?
		ORDER BY songs.shuffle_index ASC`, partyID, startIndex, endIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var songs []Song
	for rows.Next() {
		var s Song
		if err := rows.Scan(&s.ID, &s.Title, &s.YouTubeID, &s.ThumbnailURL); err != nil {
			return nil, err
		}
		songs = append(songs, s)
	}
	return songs, nil
}

func (s *Service) GetPartyName(ctx context.Context, partyID string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, "SELECT name FROM parties WHERE id = ?", partyID).Scan(&name)
	return name, err
}

func (s *Service) GetUserGuesses(ctx context.Context, partyID string, userName string) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.song_id, u_guessed.name
		FROM guesses g
		JOIN users u_guesser ON g.guesser_id = u_guesser.id
		JOIN users u_guessed ON g.guessed_user_id = u_guessed.id
		WHERE u_guesser.party_id = ? AND u_guesser.name = ?`, partyID, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	guesses := make(map[int]string)
	for rows.Next() {
		var songID int
		var guessedName string
		if err := rows.Scan(&songID, &guessedName); err != nil {
			return nil, err
		}
		guesses[songID] = guessedName
	}
	return guesses, nil
}

func (s *Service) GetPartyState(ctx context.Context, partyID string) (started bool, currentRound int, showResults bool, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT started, current_round, show_results FROM parties WHERE id = ?", partyID).Scan(&started, &currentRound, &showResults)
	return
}

type LeaderboardEntry struct {
	UserName string `json:"user_name"`
	Score    int    `json:"score"`
}

func (s *Service) SubmitGuess(ctx context.Context, guesserID, songID, guessedUserID int) error {
	if s.logger != nil {
		s.logger.Printf("Guess submitted: Guesser %d, Song %d, Guessed Owner %d", guesserID, songID, guessedUserID)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO guesses (guesser_id, song_id, guessed_user_id) 
		VALUES (?, ?, ?)
		ON CONFLICT(guesser_id, song_id) DO UPDATE SET guessed_user_id = excluded.guessed_user_id`,
		guesserID, songID, guessedUserID)
	return err
}

func (s *Service) GetLeaderboard(ctx context.Context, partyID string, round int) ([]LeaderboardEntry, error) {
	var query string
	var args []interface{}

	if round > 0 {
		query = `
			SELECT u.name, COUNT(CASE WHEN (
				(s.youtube_id != '' AND EXISTS (SELECT 1 FROM songs s2 WHERE s2.youtube_id = s.youtube_id AND s2.user_id = g.guessed_user_id)) OR
				(s.youtube_id = '' AND g.guessed_user_id = s.user_id)
			) AND s.shuffle_index BETWEEN (? - 1) * p.songs_per_round AND ? * p.songs_per_round - 1 THEN 1 END) as score
			FROM users u
			JOIN parties p ON u.party_id = p.id
			LEFT JOIN guesses g ON u.id = g.guesser_id
			LEFT JOIN songs s ON g.song_id = s.id
			WHERE u.party_id = ?
			GROUP BY u.id
			ORDER BY score DESC`
		args = []interface{}{round, round, partyID}
	} else {
		query = `
			SELECT u.name, COUNT(CASE WHEN (
				(s.youtube_id != '' AND EXISTS (SELECT 1 FROM songs s2 WHERE s2.youtube_id = s.youtube_id AND s2.user_id = g.guessed_user_id)) OR
				(s.youtube_id = '' AND g.guessed_user_id = s.user_id)
			) AND s.shuffle_index < CASE WHEN p.show_results THEN p.current_round * p.songs_per_round ELSE (p.current_round - 1) * p.songs_per_round END THEN 1 END) as score
			FROM users u
			JOIN parties p ON u.party_id = p.id
			LEFT JOIN guesses g ON u.id = g.guesser_id
			LEFT JOIN songs s ON g.song_id = s.id
			WHERE u.party_id = ?
			GROUP BY u.id
			ORDER BY score DESC`
		args = []interface{}{partyID}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.UserName, &entry.Score); err != nil {
			return nil, err
		}
		leaderboard = append(leaderboard, entry)
	}
	return leaderboard, nil
}

func (s *Service) NextRound(ctx context.Context, partyID string) error {
	var showResults bool
	err := s.db.QueryRowContext(ctx, "SELECT show_results FROM parties WHERE id = ?", partyID).Scan(&showResults)
	if err != nil {
		return err
	}

	if showResults {
		s.log(partyID, "Moving to next round")
		_, err = s.db.ExecContext(ctx, "UPDATE parties SET current_round = current_round + 1, show_results = FALSE WHERE id = ?", partyID)
	} else {
		s.log(partyID, "Revealing round results")
		_, err = s.db.ExecContext(ctx, "UPDATE parties SET show_results = TRUE WHERE id = ?", partyID)
	}
	return err
}

// User represents a participant in a party.
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SongResult represents a song along with its actual owner, used for reveals.
type SongResult struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	YouTubeID    string `json:"youtube_id"`
	ThumbnailURL string `json:"thumbnail_url"`
	OwnerName    string `json:"owner_name"`
}

func (s *Service) SearchYouTubeMusic(ctx context.Context, query string) ([]SongInput, error) {
	url := "https://music.youtube.com/youtubei/v1/music/get_search_suggestions?prettyPrint=false"

	body := map[string]interface{}{
		"input": query,
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":       "WEB_REMIX",
				"clientVersion":    "1.20251215.03.00",
				"hl":               "en",
				"gl":               "US",
				"utcOffsetMinutes": 0,
			},
			"user": map[string]interface{}{
				"lockedSafetyMode": false,
			},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:146.0) Gecko/20100101 Firefox/146.0")
	req.Header.Set("Origin", "https://music.youtube.com")
	req.Header.Set("Referer", "https://music.youtube.com/")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Contents []struct {
			SearchSuggestionsSectionRenderer struct {
				Contents []struct {
					MusicResponsiveListItemRenderer struct {
						Thumbnail struct {
							MusicThumbnailRenderer struct {
								Thumbnail struct {
									Thumbnails []struct {
										URL string `json:"url"`
									} `json:"thumbnails"`
								} `json:"thumbnail"`
							} `json:"musicThumbnailRenderer"`
						} `json:"thumbnail"`
						FlexColumns []struct {
							MusicResponsiveListItemFlexColumnRenderer struct {
								Text struct {
									Runs []struct {
										Text string `json:"text"`
									} `json:"runs"`
								} `json:"text"`
							} `json:"musicResponsiveListItemFlexColumnRenderer"`
						} `json:"flexColumns"`
						NavigationEndpoint struct {
							WatchEndpoint struct {
								VideoID string `json:"videoId"`
							} `json:"watchEndpoint"`
						} `json:"navigationEndpoint"`
						PlaylistItemData struct {
							VideoID string `json:"videoId"`
						} `json:"playlistItemData"`
					} `json:"musicResponsiveListItemRenderer"`
				} `json:"contents"`
			} `json:"searchSuggestionsSectionRenderer"`
		} `json:"contents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var songs []SongInput
	for _, section := range result.Contents {
		for _, item := range section.SearchSuggestionsSectionRenderer.Contents {
			renderer := item.MusicResponsiveListItemRenderer

			// Prefer videoId from playlistItemData or navigationEndpoint
			videoID := renderer.PlaylistItemData.VideoID
			if videoID == "" {
				videoID = renderer.NavigationEndpoint.WatchEndpoint.VideoID
			}

			if videoID == "" {
				continue
			}

			title := ""
			artist := ""
			if len(renderer.FlexColumns) > 0 && len(renderer.FlexColumns[0].MusicResponsiveListItemFlexColumnRenderer.Text.Runs) > 0 {
				title = renderer.FlexColumns[0].MusicResponsiveListItemFlexColumnRenderer.Text.Runs[0].Text
			}
			if len(renderer.FlexColumns) > 1 && len(renderer.FlexColumns[1].MusicResponsiveListItemFlexColumnRenderer.Text.Runs) > 0 {
				// The second column usually contains "Song • Artist • Views"
				artistParts := []string{}
				for _, run := range renderer.FlexColumns[1].MusicResponsiveListItemFlexColumnRenderer.Text.Runs {
					if run.Text != " • " && run.Text != "Sang" && run.Text != "Song" {
						artistParts = append(artistParts, run.Text)
					}
				}
				if len(artistParts) > 0 {
					artist = artistParts[0]
				}
			}

			fullTitle := title
			if artist != "" {
				fullTitle = fmt.Sprintf("%s - %s", title, artist)
			}

			thumbnailURL := ""
			if len(renderer.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails) > 0 {
				// Get the highest resolution thumbnail available in the list
				thumbnailURL = renderer.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails[len(renderer.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails)-1].URL
			}

			songs = append(songs, SongInput{
				Title:        fullTitle,
				YouTubeID:    videoID,
				ThumbnailURL: thumbnailURL,
			})
		}
	}

	return songs, nil
}

func (s *Service) GetPartySongs(ctx context.Context, partyID string) ([]SongResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.title, s.youtube_id, s.thumbnail_url, (
			SELECT GROUP_CONCAT(u2.name, ', ')
			FROM songs s2
			JOIN users u2 ON s2.user_id = u2.id
			WHERE u2.party_id = ? AND (
				(s.youtube_id != '' AND s2.youtube_id = s.youtube_id) OR
				(s.youtube_id = '' AND s2.id = s.id)
			)
		) as owner_names
		FROM songs s
		JOIN users u ON s.user_id = u.id
		WHERE u.party_id = ?
		ORDER BY s.shuffle_index ASC, u.name ASC, s.id ASC`, partyID, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var songs []SongResult
	for rows.Next() {
		var s SongResult
		if err := rows.Scan(&s.ID, &s.Title, &s.YouTubeID, &s.ThumbnailURL, &s.OwnerName); err != nil {
			return nil, err
		}
		songs = append(songs, s)
	}
	return songs, nil
}

// GetUsers returns all participants in a party.
func (s *Service) GetUsers(ctx context.Context, partyID string) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM users WHERE party_id = ? ORDER BY name ASC", partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// GetRoundResults returns the songs and their owners for a specific round,
// but only if the round has been revealed (i.e., current_round > round).
func (s *Service) GetRoundResults(ctx context.Context, partyID string, round int) ([]SongResult, error) {
	s.log(partyID, "Fetching results for round %d", round)
	var currentRound int
	var songsPerRound int
	var showResults bool
	err := s.db.QueryRowContext(ctx, "SELECT current_round, songs_per_round, show_results FROM parties WHERE id = ?", partyID).Scan(&currentRound, &songsPerRound, &showResults)
	if err != nil {
		return nil, err
	}

	if currentRound < round || (currentRound == round && !showResults) {
		return nil, fmt.Errorf("round %d has not been revealed yet", round)
	}

	startIndex := (round - 1) * songsPerRound
	endIndex := round*songsPerRound - 1

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.title, s.youtube_id, s.thumbnail_url, (
			SELECT GROUP_CONCAT(u2.name, ', ')
			FROM songs s2
			JOIN users u2 ON s2.user_id = u2.id
			WHERE u2.party_id = ? AND (
				(s.youtube_id != '' AND s2.youtube_id = s.youtube_id) OR
				(s.youtube_id = '' AND s2.id = s.id)
			)
		) as owner_names
		FROM songs s
		JOIN users u ON s.user_id = u.id
		WHERE u.party_id = ? AND s.shuffle_index BETWEEN ? AND ?
		ORDER BY s.shuffle_index ASC`, partyID, partyID, startIndex, endIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SongResult
	for rows.Next() {
		var r SongResult
		if err := rows.Scan(&r.ID, &r.Title, &r.YouTubeID, &r.ThumbnailURL, &r.OwnerName); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *Service) GetTotalSongs(ctx context.Context, partyID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM songs 
		JOIN users ON songs.user_id = users.id 
		WHERE users.party_id = ?`, partyID).Scan(&count)
	return count, err
}
