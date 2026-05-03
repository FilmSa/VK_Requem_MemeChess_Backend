package game

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrOpponentSeatTaken = errors.New("opponent seat already taken")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type CreateGameParams struct {
	GameID            string
	Player1ID         string
	Player2ID         *string // nil = waiting for opponent (invite link)
	Status            string
	BetAmount         int64
	MemeMode          bool
	GameMode          string
	FEN               string
	CurrentTurnUserID string
}

func (r *Repository) CreateGame(ctx context.Context, p CreateGameParams) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO games (
			id, player1_id, player2_id, status, bet_amount, meme_mode, game_mode, fen, current_turn_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	var player2 any
	if p.Player2ID != nil {
		player2 = *p.Player2ID
	}

	_, err := r.pool.Exec(ctx, query,
		p.GameID,
		p.Player1ID,
		player2,
		p.Status,
		p.BetAmount,
		p.MemeMode,
		p.GameMode,
		p.FEN,
		p.CurrentTurnUserID,
	)
	if err != nil {
		return fmt.Errorf("insert game: %w", err)
	}

	return nil
}

func (r *Repository) SetPlayer2(ctx context.Context, gameID, player2ID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE games
		SET player2_id = $2
		WHERE id = $1 AND player2_id IS NULL
	`

	tag, err := r.pool.Exec(ctx, q, gameID, player2ID)
	if err != nil {
		return fmt.Errorf("set player2: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOpponentSeatTaken
	}
	return nil
}

type SaveMoveParams struct {
	GameID      string
	PlayerID    string
	MoveNumber  int
	Move        string
	FEN         string
	IsCapture   bool
	IsCheck     bool
	IsCheckmate bool
}

func (r *Repository) SaveMove(ctx context.Context, p SaveMoveParams) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO moves (
			game_id, player_id, move_number, move, fen, is_capture, is_check, is_checkmate
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.Exec(ctx, query,
		p.GameID,
		p.PlayerID,
		p.MoveNumber,
		p.Move,
		p.FEN,
		p.IsCapture,
		p.IsCheck,
		p.IsCheckmate,
	)
	if err != nil {
		return fmt.Errorf("insert move: %w", err)
	}

	return nil
}

type UpdateGameStateParams struct {
	GameID            string
	Status            string
	FEN               string
	CurrentTurnUserID string
	WinnerID          *string
	FinishedAt        *time.Time
	FinishedReason    *string
}

func (r *Repository) UpdateGameState(ctx context.Context, p UpdateGameStateParams) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		UPDATE games
		SET
			status = $2,
			fen = $3,
			current_turn_user_id = $4,
			winner_id = $5,
			finished_at = $6,
			finished_reason = $7
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		p.GameID,
		p.Status,
		p.FEN,
		p.CurrentTurnUserID,
		p.WinnerID,
		p.FinishedAt,
		p.FinishedReason,
	)
	if err != nil {
		return fmt.Errorf("update game state: %w", err)
	}

	return nil
}

func (r *Repository) TryMarkPaidOut(ctx context.Context, gameID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE games
		SET paid_out = true
		WHERE id = $1 AND paid_out = false
	`

	tag, err := r.pool.Exec(ctx, q, gameID)
	if err != nil {
		return false, fmt.Errorf("mark paid_out: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
