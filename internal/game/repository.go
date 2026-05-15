package game

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	GameID                 string
	Player1ID              string
	Player2ID              *string // nil = waiting for opponent (invite link)
	Status                 string
	BetAmount              int64
	MemeMode               bool
	GameMode               string
	TimeControlID          *string
	TimeControlBaseMs      *int64
	TimeControlIncrementMs *int64
	Player1RemainingMs     *int64
	Player2RemainingMs     *int64
	CurrentTurnStartedAt   *time.Time
	FEN                    string
	CurrentTurnUserID      string
}

func (r *Repository) CreateGame(ctx context.Context, p CreateGameParams) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	queryWithMode := `
		INSERT INTO games (
			id, player1_id, player2_id, status, bet_amount, meme_mode, game_mode,
			time_control_id, time_control_base_ms, time_control_increment_ms,
			player1_remaining_ms, player2_remaining_ms, current_turn_started_at,
			fen, current_turn_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	var player2 any
	if p.Player2ID != nil {
		player2 = *p.Player2ID
	}

	_, err := r.pool.Exec(ctx, queryWithMode,
		p.GameID,
		p.Player1ID,
		player2,
		p.Status,
		p.BetAmount,
		p.MemeMode,
		p.GameMode,
		p.TimeControlID,
		p.TimeControlBaseMs,
		p.TimeControlIncrementMs,
		p.Player1RemainingMs,
		p.Player2RemainingMs,
		p.CurrentTurnStartedAt,
		p.FEN,
		p.CurrentTurnUserID,
	)
	if err != nil {
		if !isMissingGameMetadataColumns(err) {
			return fmt.Errorf("insert game: %w", err)
		}

		legacyQuery := `
			INSERT INTO games (
				id, player1_id, player2_id, status, bet_amount, meme_mode, fen, current_turn_user_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`

		_, legacyErr := r.pool.Exec(ctx, legacyQuery,
			p.GameID,
			p.Player1ID,
			player2,
			p.Status,
			p.BetAmount,
			p.MemeMode,
			p.FEN,
			p.CurrentTurnUserID,
		)
		if legacyErr != nil {
			return fmt.Errorf("insert game: %w", legacyErr)
		}

		return nil
	}

	return nil
}

func isMissingGameModeColumn(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "game_mode") &&
		(strings.Contains(msg, "does not exist") || strings.Contains(msg, "unknown column"))
}

func isMissingGameMetadataColumns(err error) bool {
	if err == nil {
		return false
	}

	if isMissingGameModeColumn(err) {
		return true
	}

	msg := strings.ToLower(err.Error())
	for _, column := range []string{
		"time_control_id",
		"time_control_base_ms",
		"time_control_increment_ms",
		"player1_remaining_ms",
		"player2_remaining_ms",
		"current_turn_started_at",
	} {
		if strings.Contains(msg, column) &&
			(strings.Contains(msg, "does not exist") || strings.Contains(msg, "unknown column")) {
			return true
		}
	}

	return false
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
	GameID                 string
	Status                 string
	FEN                    string
	CurrentTurnUserID      string
	TimeControlID          *string
	TimeControlBaseMs      *int64
	TimeControlIncrementMs *int64
	Player1RemainingMs     *int64
	Player2RemainingMs     *int64
	CurrentTurnStartedAt   *time.Time
	WinnerID               *string
	FinishedAt             *time.Time
	FinishedReason         *string
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
			time_control_id = $5,
			time_control_base_ms = $6,
			time_control_increment_ms = $7,
			player1_remaining_ms = $8,
			player2_remaining_ms = $9,
			current_turn_started_at = $10,
			winner_id = $11,
			finished_at = $12,
			finished_reason = $13
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		p.GameID,
		p.Status,
		p.FEN,
		p.CurrentTurnUserID,
		p.TimeControlID,
		p.TimeControlBaseMs,
		p.TimeControlIncrementMs,
		p.Player1RemainingMs,
		p.Player2RemainingMs,
		p.CurrentTurnStartedAt,
		p.WinnerID,
		p.FinishedAt,
		p.FinishedReason,
	)
	if err != nil {
		if !isMissingGameMetadataColumns(err) {
			return fmt.Errorf("update game state: %w", err)
		}

		legacyQuery := `
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

		_, legacyErr := r.pool.Exec(ctx, legacyQuery,
			p.GameID,
			p.Status,
			p.FEN,
			p.CurrentTurnUserID,
			p.WinnerID,
			p.FinishedAt,
			p.FinishedReason,
		)
		if legacyErr != nil {
			return fmt.Errorf("update game state: %w", legacyErr)
		}

		return nil
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

type UserGameListRow struct {
	GameID         string
	Status         string
	GameMode       string
	Player1ID      string
	Player2ID      *string
	WinnerID       *string
	FEN            string
	BetAmount      int64
	Currency       string
	FinishedAt     *time.Time
	FinishedReason *string
	CreatedAt      time.Time
	StartedAt      *time.Time
	TimeControlID  *string

	Player1Username  string
	Player1AvatarURL *string
	Player2Username  *string
	Player2AvatarURL *string

	LastMoveSAN    *string
	LastMoveNumber *int32
}

type GameParticipantsRow struct {
	GameID           string
	Player1ID        string
	Player1Username  string
	Player1AvatarURL *string
	Player2ID        *string
	Player2Username  *string
	Player2AvatarURL *string
}

func (r *Repository) ListUserGames(ctx context.Context, userID string, limit, offset int) ([]UserGameListRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	const q = `
		SELECT
			g.id::text,
			g.status,
			g.game_mode,
			g.player1_id::text,
			g.player2_id::text,
			g.winner_id::text,
			g.fen,
			g.bet_amount,
			g.currency,
			g.finished_at,
			g.finished_reason,
			g.created_at,
			g.started_at,
			g.time_control_id,
			p1.username,
			p1.avatar_url,
			p2.username,
			p2.avatar_url,
			lm.move,
			lm.move_number
		FROM games g
		INNER JOIN users p1 ON p1.id = g.player1_id
		LEFT JOIN users p2 ON p2.id = g.player2_id
		LEFT JOIN LATERAL (
			SELECT m.move, m.move_number
			FROM moves m
			WHERE m.game_id = g.id
			ORDER BY m.move_number DESC
			LIMIT 1
		) lm ON true
		WHERE g.player1_id = $1::uuid OR g.player2_id = $1::uuid
		ORDER BY COALESCE(g.finished_at, g.started_at, g.created_at) DESC NULLS LAST,
			g.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user games: %w", err)
	}
	defer rows.Close()

	var out []UserGameListRow
	for rows.Next() {
		var row UserGameListRow
		if err := rows.Scan(
			&row.GameID,
			&row.Status,
			&row.GameMode,
			&row.Player1ID,
			&row.Player2ID,
			&row.WinnerID,
			&row.FEN,
			&row.BetAmount,
			&row.Currency,
			&row.FinishedAt,
			&row.FinishedReason,
			&row.CreatedAt,
			&row.StartedAt,
			&row.TimeControlID,
			&row.Player1Username,
			&row.Player1AvatarURL,
			&row.Player2Username,
			&row.Player2AvatarURL,
			&row.LastMoveSAN,
			&row.LastMoveNumber,
		); err != nil {
			return nil, fmt.Errorf("scan user game row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list user games rows: %w", err)
	}
	return out, nil
}

func (r *Repository) GetGameParticipants(ctx context.Context, gameID string) (*GameParticipantsRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT
			g.id::text,
			g.player1_id::text,
			p1.username,
			p1.avatar_url,
			g.player2_id::text,
			p2.username,
			p2.avatar_url
		FROM games g
		INNER JOIN users p1 ON p1.id = g.player1_id
		LEFT JOIN users p2 ON p2.id = g.player2_id
		WHERE g.id = $1
	`

	var row GameParticipantsRow
	err := r.pool.QueryRow(ctx, q, gameID).Scan(
		&row.GameID,
		&row.Player1ID,
		&row.Player1Username,
		&row.Player1AvatarURL,
		&row.Player2ID,
		&row.Player2Username,
		&row.Player2AvatarURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get game participants: %w", err)
	}

	return &row, nil
}
