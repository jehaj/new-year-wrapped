package party

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"
)

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

func (s *Service) JoinParty(ctx context.Context, partyID string, userName string, songs []string) error {
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
		_, err = tx.ExecContext(ctx, "INSERT INTO songs (user_id, title) VALUES (?, ?)", userID, song)
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
	ID    int    `json:"id"`
	Title string `json:"title"`
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
		SELECT songs.id, songs.title 
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
		if err := rows.Scan(&s.ID, &s.Title); err != nil {
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

func (s *Service) GetPartyState(ctx context.Context, partyID string) (started bool, currentRound int, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT started, current_round FROM parties WHERE id = ?", partyID).Scan(&started, &currentRound)
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
	query := `
		SELECT u.name, COUNT(CASE WHEN g.guessed_user_id = s.user_id THEN 1 END) as score
		FROM users u
		JOIN parties p ON u.party_id = p.id
		LEFT JOIN guesses g ON u.id = g.guesser_id
		LEFT JOIN songs s ON g.song_id = s.id
		WHERE u.party_id = ? AND s.shuffle_index < (p.current_round - 1) * p.songs_per_round`

	args := []interface{}{partyID}
	if round > 0 {
		query += " AND s.shuffle_index BETWEEN (? - 1) * p.songs_per_round AND ? * p.songs_per_round - 1"
		args = append(args, round, round)
	}

	query += `
		GROUP BY u.id
		ORDER BY score DESC`

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
	s.log(partyID, "Moving to next round")
	_, err := s.db.ExecContext(ctx, "UPDATE parties SET current_round = current_round + 1 WHERE id = ?", partyID)
	return err
}

// User represents a participant in a party.
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SongResult represents a song along with its actual owner, used for reveals.
type SongResult struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	OwnerName string `json:"owner_name"`
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
	err := s.db.QueryRowContext(ctx, "SELECT current_round, songs_per_round FROM parties WHERE id = ?", partyID).Scan(&currentRound, &songsPerRound)
	if err != nil {
		return nil, err
	}

	if currentRound <= round {
		return nil, fmt.Errorf("round %d has not been revealed yet", round)
	}

	startIndex := (round - 1) * songsPerRound
	endIndex := round*songsPerRound - 1

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.title, u.name
		FROM songs s
		JOIN users u ON s.user_id = u.id
		WHERE u.party_id = ? AND s.shuffle_index BETWEEN ? AND ?
		ORDER BY s.shuffle_index ASC`, partyID, startIndex, endIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SongResult
	for rows.Next() {
		var r SongResult
		if err := rows.Scan(&r.ID, &r.Title, &r.OwnerName); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
