package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

var ErrNotFound = errors.New("not found")

func (r *Repository) GetCatalog(ctx context.Context) ([]Item, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT slug,
		       CASE WHEN type = 'sticker' THEN 'emote' ELSE type END AS type,
		       title, asset_url, meta, created_at
		FROM inventory_items
		ORDER BY type, slug
	`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("select catalog: %w", err)
	}
	defer rows.Close()

	var out []Item
	for rows.Next() {
		var it Item
		var metaBytes []byte
		if err := rows.Scan(&it.Slug, &it.Type, &it.Title, &it.AssetURL, &metaBytes, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan catalog: %w", err)
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &it.Meta)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repository) GetOwned(ctx context.Context, userID string) ([]Item, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT i.slug,
		       CASE WHEN i.type = 'sticker' THEN 'emote' ELSE i.type END AS type,
		       i.title, i.asset_url, i.meta, i.created_at
		FROM user_inventory_items ui
		JOIN inventory_items i ON i.slug = ui.item_slug
		WHERE ui.user_id = $1
		ORDER BY i.type, i.slug
	`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("select owned: %w", err)
	}
	defer rows.Close()

	var out []Item
	for rows.Next() {
		var it Item
		var metaBytes []byte
		if err := rows.Scan(&it.Slug, &it.Type, &it.Title, &it.AssetURL, &metaBytes, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan owned: %w", err)
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &it.Meta)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repository) EnsureSelectionRow(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		INSERT INTO user_inventory_selection (
			user_id,
			piece_skin_slug,
			board_skin_slug,
			emote_slugs,
			sticker_slugs
		)
		VALUES ($1, NULL, NULL, '{}'::text[], '{}'::text[])
		ON CONFLICT (user_id) DO NOTHING
	`
	_, err := r.pool.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("ensure selection: %w", err)
	}
	return nil
}

func (r *Repository) GetSelection(ctx context.Context, userID string) (Selection, error) {
	if err := r.EnsureSelectionRow(ctx, userID); err != nil {
		return Selection{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT piece_skin_slug,
		       board_skin_slug,
		       COALESCE(emote_slugs, sticker_slugs, '{}'::text[])
		FROM user_inventory_selection
		WHERE user_id = $1
	`

	var sel Selection
	err := r.pool.QueryRow(ctx, q, userID).Scan(&sel.PieceSkinSlug, &sel.BoardSkinSlug, &sel.EmoteSlugs)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Selection{}, ErrNotFound
		}
		return Selection{}, fmt.Errorf("select selection: %w", err)
	}
	if sel.EmoteSlugs == nil {
		sel.EmoteSlugs = []string{}
	}
	return sel, nil
}

func (r *Repository) SetSelection(ctx context.Context, userID string, sel Selection) (Selection, error) {
	if err := r.EnsureSelectionRow(ctx, userID); err != nil {
		return Selection{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE user_inventory_selection
		SET piece_skin_slug = $2,
		    board_skin_slug = $3,
		    emote_slugs = $4,
		    sticker_slugs = $4,
		    updated_at = now()
		WHERE user_id = $1
		RETURNING piece_skin_slug, board_skin_slug, COALESCE(emote_slugs, sticker_slugs, '{}'::text[])
	`

	var out Selection
	err := r.pool.QueryRow(ctx, q, userID, sel.PieceSkinSlug, sel.BoardSkinSlug, sel.EmoteSlugs).
		Scan(&out.PieceSkinSlug, &out.BoardSkinSlug, &out.EmoteSlugs)
	if err != nil {
		return Selection{}, fmt.Errorf("update selection: %w", err)
	}
	if out.EmoteSlugs == nil {
		out.EmoteSlugs = []string{}
	}
	return out, nil
}

func (r *Repository) GetItem(ctx context.Context, slug string) (*Item, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT slug,
		       CASE WHEN type = 'sticker' THEN 'emote' ELSE type END AS type,
		       title, asset_url, meta, created_at
		FROM inventory_items
		WHERE slug = $1
	`

	var it Item
	var metaBytes []byte
	err := r.pool.QueryRow(ctx, q, slug).Scan(&it.Slug, &it.Type, &it.Title, &it.AssetURL, &metaBytes, &it.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select item: %w", err)
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &it.Meta)
	}
	return &it, nil
}

func (r *Repository) UserOwns(ctx context.Context, userID string, slug string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT 1
		FROM user_inventory_items
		WHERE user_id = $1 AND item_slug = $2
	`
	var one int
	err := r.pool.QueryRow(ctx, q, userID, slug).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("select ownership: %w", err)
	}
	return true, nil
}

func (r *Repository) HasSelectedEmoteAssetURL(ctx context.Context, userID string, assetURL string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT 1
		FROM user_inventory_selection sel
		JOIN unnest(COALESCE(sel.emote_slugs, sel.sticker_slugs, '{}'::text[])) es(slug) ON true
		JOIN inventory_items i ON i.slug = es.slug AND (i.type = 'emote' OR i.type = 'sticker')
		JOIN user_inventory_items ui ON ui.user_id = sel.user_id AND ui.item_slug = i.slug
		WHERE sel.user_id = $1
		  AND i.asset_url = $2
		LIMIT 1
	`

	var one int
	err := r.pool.QueryRow(ctx, q, userID, assetURL).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check selected emote asset_url: %w", err)
	}
	return true, nil
}
