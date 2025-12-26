package party

import (
	"context"
	"database/sql"
	"fmt"
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
