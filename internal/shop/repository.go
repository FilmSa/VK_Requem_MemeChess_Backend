package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"meme_chess/internal/inventory"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

var (
	ErrItemNotForSale          = errors.New("item is not for sale")
	ErrItemAlreadyOwned        = errors.New("item already owned")
	ErrInsufficientShopCurrency = errors.New("insufficient shop currency")
)

func (r *Repository) GetCatalog(ctx context.Context, userID string) ([]CatalogItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT
			i.slug, i.type, i.title, i.asset_url, i.meta, i.created_at,
			si.price_shop_currency, si.is_active,
			(ui.user_id IS NOT NULL) AS owned
		FROM shop_items si
		JOIN inventory_items i ON i.slug = si.item_slug
		LEFT JOIN user_inventory_items ui
		       ON ui.user_id = $1
		      AND ui.item_slug = i.slug
		ORDER BY i.type, i.slug
	`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("select shop catalog: %w", err)
	}
	defer rows.Close()

	var out []CatalogItem
	for rows.Next() {
		var it inventory.Item
		var metaBytes []byte
		var ci CatalogItem
		if err := rows.Scan(
			&it.Slug, &it.Type, &it.Title, &it.AssetURL, &metaBytes, &it.CreatedAt,
			&ci.Price, &ci.IsActive,
			&ci.Owned,
		); err != nil {
			return nil, fmt.Errorf("scan shop catalog: %w", err)
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &it.Meta)
		}
		ci.Item = it
		out = append(out, ci)
	}
	return out, rows.Err()
}

func (r *Repository) Buy(ctx context.Context, userID string, itemSlug string) (newShop int64, newGame int64, bought *inventory.Item, err error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var it inventory.Item
	var metaBytes []byte
	var price int64
	var isActive bool
	const qItem = `
		SELECT i.slug, i.type, i.title, i.asset_url, i.meta, i.created_at,
		       si.price_shop_currency, si.is_active
		FROM shop_items si
		JOIN inventory_items i ON i.slug = si.item_slug
		WHERE i.slug = $1
	`
	scanErr := tx.QueryRow(ctx, qItem, itemSlug).Scan(
		&it.Slug, &it.Type, &it.Title, &it.AssetURL, &metaBytes, &it.CreatedAt,
		&price, &isActive,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 0, 0, nil, ErrItemNotForSale
		}
		return 0, 0, nil, fmt.Errorf("select shop item: %w", scanErr)
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &it.Meta)
	}

	if !isActive {
		return 0, 0, nil, ErrItemNotForSale
	}
	switch it.Type {
	case inventory.ItemTypeSticker, inventory.ItemTypeBoardSkin, inventory.ItemTypePieceSkin:
	default:
		return 0, 0, nil, ErrItemNotForSale
	}

	const qOwned = `
		SELECT 1
		FROM user_inventory_items
		WHERE user_id = $1 AND item_slug = $2
	`
	var one int
	ownErr := tx.QueryRow(ctx, qOwned, userID, itemSlug).Scan(&one)
	if ownErr == nil {
		return 0, 0, nil, ErrItemAlreadyOwned
	}
	if ownErr != nil && !errors.Is(ownErr, pgx.ErrNoRows) {
		return 0, 0, nil, fmt.Errorf("check ownership: %w", ownErr)
	}

	const qReserve = `
		UPDATE users
		SET shop_currency = shop_currency - $2
		WHERE id = $1
		  AND shop_currency >= $2
		RETURNING shop_currency, game_currency
	`
	resErr := tx.QueryRow(ctx, qReserve, userID, price).Scan(&newShop, &newGame)
	if resErr != nil {
		if errors.Is(resErr, pgx.ErrNoRows) {
			return 0, 0, nil, ErrInsufficientShopCurrency
		}
		return 0, 0, nil, fmt.Errorf("reserve shop currency: %w", resErr)
	}

	const qGrant = `
		INSERT INTO user_inventory_items (user_id, item_slug)
		VALUES ($1, $2)
	`
	_, err = tx.Exec(ctx, qGrant, userID, itemSlug)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("grant item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, nil, fmt.Errorf("commit tx: %w", err)
	}

	return newShop, newGame, &it, nil
}

