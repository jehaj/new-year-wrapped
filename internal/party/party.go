package party

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) JoinParty(ctx context.Context, partyID string, userName string, songs []string) error {
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

func (s *Service) CreateParty(ctx context.Context, id string, name string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO parties (id, name) VALUES (?, ?)", id, name)
	return err
}

func (s *Service) StartCompetition(ctx context.Context, partyID string) error {
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

	// Assign rounds (5 songs per round)
	for i, id := range songIDs {
		roundNumber := (i / 5) + 1
		_, err = tx.ExecContext(ctx, "UPDATE songs SET round_number = ? WHERE id = ?", roundNumber, id)
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT songs.id, songs.title 
		FROM songs 
		JOIN users ON songs.user_id = users.id 
		WHERE users.party_id = ? AND songs.round_number = ?`, partyID, round)
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

func (s *Service) GetPartyState(ctx context.Context, partyID string) (started bool, currentRound int, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT started, current_round FROM parties WHERE id = ?", partyID).Scan(&started, &currentRound)
	return
}
