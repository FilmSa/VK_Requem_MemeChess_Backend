package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meme_chess/internal/analyzer/pattern"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is nil")
	}

	store := &PostgresStore{pool: pool}
	if err := store.init(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS analysis_positions (
			position_hash TEXT PRIMARY KEY,
			depth INTEGER NOT NULL,
			tree_depth INTEGER NOT NULL DEFAULT 0,
			frontier_depth INTEGER NOT NULL DEFAULT 0,
			best_score_cp INTEGER NOT NULL,
			ready BOOLEAN NOT NULL DEFAULT FALSE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_moves (
			position_hash TEXT NOT NULL REFERENCES analysis_positions(position_hash) ON DELETE CASCADE,
			move_key TEXT NOT NULL,
			score_cp INTEGER NOT NULL,
			delta_cp INTEGER NOT NULL,
			quality TEXT NOT NULL,
			tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			depth INTEGER NOT NULL,
			next_position_hash TEXT NOT NULL DEFAULT '',
			ready BOOLEAN NOT NULL DEFAULT FALSE,
			PRIMARY KEY (position_hash, move_key)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("init analysis postgres store: %w", err)
		}
	}

	return nil
}

func (s *PostgresStore) GetPosition(hash string) (*PositionAnalysis, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := s.pool.QueryRow(ctx, `
		SELECT depth, tree_depth, frontier_depth, best_score_cp, ready
		FROM analysis_positions
		WHERE position_hash = $1
	`, hash)

	p := &PositionAnalysis{
		PositionHash: hash,
		Moves:        make(map[string]*MoveAnalysis),
	}
	if err := row.Scan(&p.Depth, &p.TreeDepth, &p.FrontierDepth, &p.BestScoreCP, &p.Ready); err != nil {
		return nil, false
	}

	rows, err := s.pool.Query(ctx, `
		SELECT move_key, score_cp, delta_cp, quality, tags_json, depth, next_position_hash, ready
		FROM analysis_moves
		WHERE position_hash = $1
	`, hash)
	if err != nil {
		return p, true
	}
	defer rows.Close()

	for rows.Next() {
		var (
			moveKey  string
			tagsJSON []byte
			move     MoveAnalysis
		)

		if err := rows.Scan(&moveKey, &move.ScoreCP, &move.DeltaCP, &move.Quality, &tagsJSON, &move.Depth, &move.NextPositionHash, &move.Ready); err != nil {
			continue
		}
		if err := json.Unmarshal(tagsJSON, &move.Tags); err != nil {
			move.Tags = nil
		}
		p.Moves[moveKey] = cloneMoveAnalysis(&move)
	}

	return p, true
}

func (s *PostgresStore) PutPosition(p *PositionAnalysis) {
	if p == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = s.pool.Exec(ctx, `
		INSERT INTO analysis_positions(position_hash, depth, tree_depth, frontier_depth, best_score_cp, ready, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (position_hash) DO UPDATE SET
			depth = GREATEST(analysis_positions.depth, EXCLUDED.depth),
			tree_depth = GREATEST(analysis_positions.tree_depth, EXCLUDED.tree_depth),
			frontier_depth = GREATEST(analysis_positions.frontier_depth, EXCLUDED.frontier_depth),
			best_score_cp = EXCLUDED.best_score_cp,
			ready = EXCLUDED.ready,
			updated_at = now()
	`, p.PositionHash, p.Depth, p.TreeDepth, p.FrontierDepth, p.BestScoreCP, p.Ready)
}

func (s *PostgresStore) SetTreeDepth(hash string, treeDepth int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = s.pool.Exec(ctx, `
		UPDATE analysis_positions
		SET tree_depth = GREATEST(tree_depth, $2), updated_at = now()
		WHERE position_hash = $1
	`, hash, treeDepth)
}

func (s *PostgresStore) SetFrontierDepth(hash string, frontierDepth int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = s.pool.Exec(ctx, `
		UPDATE analysis_positions
		SET frontier_depth = GREATEST(frontier_depth, $2), updated_at = now()
		WHERE position_hash = $1
	`, hash, frontierDepth)
}

func (s *PostgresStore) GetMove(hash, moveKey string) (*MoveAnalysis, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		move     MoveAnalysis
		tagsJSON []byte
	)
	row := s.pool.QueryRow(ctx, `
		SELECT score_cp, delta_cp, quality, tags_json, depth, next_position_hash, ready
		FROM analysis_moves
		WHERE position_hash = $1 AND move_key = $2
	`, hash, moveKey)
	if err := row.Scan(&move.ScoreCP, &move.DeltaCP, &move.Quality, &tagsJSON, &move.Depth, &move.NextPositionHash, &move.Ready); err != nil {
		return nil, false
	}
	if err := json.Unmarshal(tagsJSON, &move.Tags); err != nil {
		return nil, false
	}
	return &move, true
}

func (s *PostgresStore) PutMove(hash string, depth int, bestScore int, moveKey string, move *MoveAnalysis) {
	if move == nil {
		return
	}

	s.PutPosition(&PositionAnalysis{
		PositionHash:  hash,
		Depth:         depth,
		TreeDepth:     0,
		FrontierDepth: 0,
		BestScoreCP:   bestScore,
		Ready:         true,
	})

	tagsJSON, err := json.Marshal(move.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = s.pool.Exec(ctx, `
		INSERT INTO analysis_moves(position_hash, move_key, score_cp, delta_cp, quality, tags_json, depth, next_position_hash, ready)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)
		ON CONFLICT (position_hash, move_key) DO UPDATE SET
			score_cp = EXCLUDED.score_cp,
			delta_cp = EXCLUDED.delta_cp,
			quality = EXCLUDED.quality,
			tags_json = EXCLUDED.tags_json,
			depth = GREATEST(analysis_moves.depth, EXCLUDED.depth),
			next_position_hash = EXCLUDED.next_position_hash,
			ready = EXCLUDED.ready
	`, hash, moveKey, move.ScoreCP, move.DeltaCP, move.Quality, string(tagsJSON), move.Depth, move.NextPositionHash, move.Ready)
}

func (s *PostgresStore) Stats() (positions int, moves int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM analysis_positions`).Scan(&positions)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM analysis_moves`).Scan(&moves)
	return positions, moves
}

func (s *PostgresStore) Dump() map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := make(map[string]any)
	rows, err := s.pool.Query(ctx, `
		SELECT
			p.position_hash,
			p.depth,
			p.tree_depth,
			p.frontier_depth,
			p.best_score_cp,
			p.ready,
			m.move_key,
			m.score_cp,
			m.delta_cp,
			m.quality,
			m.tags_json,
			m.depth,
			m.next_position_hash,
			m.ready
		FROM analysis_positions p
		LEFT JOIN analysis_moves m ON m.position_hash = p.position_hash
		ORDER BY p.position_hash, m.move_key
	`)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var (
			hash          string
			posDepth      int
			treeDepth     int
			frontierDepth int
			bestScore     int
			posReady      bool
			moveKey       pgtype.Text
			moveScore     pgtype.Int4
			moveDelta     pgtype.Int4
			moveQuality   pgtype.Text
			tagsJSON      []byte
			moveDepth     pgtype.Int4
			nextHash      pgtype.Text
			moveReady     pgtype.Bool
		)

		if err := rows.Scan(&hash, &posDepth, &treeDepth, &frontierDepth, &bestScore, &posReady, &moveKey, &moveScore, &moveDelta, &moveQuality, &tagsJSON, &moveDepth, &nextHash, &moveReady); err != nil {
			continue
		}

		entry, ok := out[hash]
		if !ok {
			entry = map[string]any{
				"depth":          posDepth,
				"tree_depth":     treeDepth,
				"frontier_depth": frontierDepth,
				"best_score":     bestScore,
				"ready":          posReady,
				"moves":          map[string]any{},
			}
			out[hash] = entry
		}

		if !moveKey.Valid {
			continue
		}

		var tags []pattern.Tag
		if err := json.Unmarshal(tagsJSON, &tags); err != nil {
			tags = nil
		}

		entry.(map[string]any)["moves"].(map[string]any)[moveKey.String] = map[string]any{
			"score_cp":           int32ToInt(moveScore),
			"delta_cp":           int32ToInt(moveDelta),
			"quality":            textToString(moveQuality),
			"tags":               tags,
			"depth":              int32ToInt(moveDepth),
			"next_position_hash": textToString(nextHash),
			"ready":              boolValue(moveReady),
		}
	}

	return out
}

func int32ToInt(v pgtype.Int4) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int32)
}

func textToString(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func boolValue(v pgtype.Bool) bool {
	return v.Valid && v.Bool
}

var _ Store = (*PostgresStore)(nil)
var _ Store = (*MemoryStore)(nil)
var _ = pgx.ErrNoRows
