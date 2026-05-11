package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"meme_chess/internal/inventory"

	"github.com/jackc/pgx/v5/pgxpool"
)

type catalogItemSeed struct {
	Slug     string
	Type     inventory.ItemType
	Title    string
	AssetURL string
	Meta     map[string]any
}

type shopItemSeed struct {
	Slug  string
	Price int64
}

const (
	defaultPieceSkinSlug = "piece.classic"
	defaultBoardSkinSlug = "board.classic"
)

var (
	defaultEmoteSlugs = []string{
		"emote.cat",
		"emote.dog",
		"emote.scelet",
	}

	catalogSeed = []catalogItemSeed{
		{Slug: "piece.classic", Type: inventory.ItemTypePieceSkin, Title: "Classic pieces"},
		{Slug: "piece.imperium", Type: inventory.ItemTypePieceSkin, Title: "Imperium"},
		{Slug: "piece.rome", Type: inventory.ItemTypePieceSkin, Title: "ROME"},
		{Slug: "piece.halo", Type: inventory.ItemTypePieceSkin, Title: "Halo"},
		{Slug: "piece.lotr", Type: inventory.ItemTypePieceSkin, Title: "Lotr"},
		{Slug: "board.classic", Type: inventory.ItemTypeBoardSkin, Title: "Classic board"},
		{Slug: "board.burgundy", Type: inventory.ItemTypeBoardSkin, Title: "Burgundy board"},
		{Slug: "board.mono", Type: inventory.ItemTypeBoardSkin, Title: "Mono board"},
		{Slug: "board.rome", Type: inventory.ItemTypeBoardSkin, Title: "Rome board"},
		{Slug: "board.halo", Type: inventory.ItemTypeBoardSkin, Title: "Halo board"},
		{Slug: "emote.cat", Type: inventory.ItemTypeEmote, Title: "Cat", AssetURL: "/emoji/cat.mp4"},
		{Slug: "emote.dog", Type: inventory.ItemTypeEmote, Title: "Dog", AssetURL: "/emoji/dog.mp4"},
		{Slug: "emote.scelet", Type: inventory.ItemTypeEmote, Title: "Scelet", AssetURL: "/emoji/scelet.mp4"},
		{Slug: "emote.axe", Type: inventory.ItemTypeEmote, Title: "Axe", AssetURL: "/emoji/axe.mp4"},
		{Slug: "emote.ishowspeed", Type: inventory.ItemTypeEmote, Title: "IShowSpeed", AssetURL: "/emoji/ishowspeed.mp4"},
		{Slug: "emote.nobrain", Type: inventory.ItemTypeEmote, Title: "NoBrain", AssetURL: "/emoji/nobrain.mp4"},
		{Slug: "emote.brilliant", Type: inventory.ItemTypeEmote, Title: "Brilliant", AssetURL: "/emoji/brilliant.mp4"},
		{Slug: "emote.chinarock", Type: inventory.ItemTypeEmote, Title: "ChinaRock", AssetURL: "/emoji/chinarock.mp4"},
		{Slug: "emote.hello", Type: inventory.ItemTypeEmote, Title: "Hello", AssetURL: "/emoji/hello.mp4"},
		{Slug: "emote.nononono", Type: inventory.ItemTypeEmote, Title: "NoNoNoNo", AssetURL: "/emoji/nononono.mp4"},
		{Slug: "emote.oaoaoao", Type: inventory.ItemTypeEmote, Title: "OAOAOAO", AssetURL: "/emoji/oaoaoao.mp4"},
		{Slug: "emote.ohno", Type: inventory.ItemTypeEmote, Title: "OhNo", AssetURL: "/emoji/ohno.mp4"},
		{Slug: "emote.seletonchik", Type: inventory.ItemTypeEmote, Title: "Seletonchik", AssetURL: "/emoji/seletonchik.mp4"},
		{Slug: "emote.sigma", Type: inventory.ItemTypeEmote, Title: "Sigma", AssetURL: "/emoji/sigma.mp4"},
		{Slug: "emote.toyota", Type: inventory.ItemTypeEmote, Title: "Toyota", AssetURL: "/emoji/toyota.mp4"},
	}

	defaultOwnedSlugs = []string{
		defaultPieceSkinSlug,
		"board.classic",
		"board.burgundy",
		"board.mono",
		"board.rome",
		"board.halo",
		"emote.cat",
		"emote.dog",
		"emote.scelet",
	}

	shopSeed = []shopItemSeed{
		{Slug: "piece.imperium", Price: 200},
		{Slug: "piece.rome", Price: 200},
		{Slug: "piece.halo", Price: 200},
		{Slug: "piece.lotr", Price: 200},
		{Slug: "emote.axe", Price: 200},
		{Slug: "emote.ishowspeed", Price: 200},
		{Slug: "emote.nobrain", Price: 200},
		{Slug: "emote.brilliant", Price: 200},
		{Slug: "emote.chinarock", Price: 200},
		{Slug: "emote.hello", Price: 200},
		{Slug: "emote.nononono", Price: 200},
		{Slug: "emote.oaoaoao", Price: 200},
		{Slug: "emote.ohno", Price: 200},
		{Slug: "emote.seletonchik", Price: 200},
		{Slug: "emote.sigma", Price: 200},
		{Slug: "emote.toyota", Price: 200},
	}
)

func SyncStoreCatalog(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin store catalog sync: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	allowedSlugs := make([]string, 0, len(catalogSeed))
	allowedEmoteSlugs := make([]string, 0, len(catalogSeed))
	for _, item := range catalogSeed {
		allowedSlugs = append(allowedSlugs, item.Slug)
		if item.Type == inventory.ItemTypeEmote {
			allowedEmoteSlugs = append(allowedEmoteSlugs, item.Slug)
		}

		metaJSON, err := json.Marshal(item.Meta)
		if err != nil {
			return fmt.Errorf("marshal item meta for %s: %w", item.Slug, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_items (slug, type, title, asset_url, meta)
			VALUES ($1, $2, $3, NULLIF($4, ''), $5::jsonb)
			ON CONFLICT (slug) DO UPDATE
			SET type = EXCLUDED.type,
			    title = EXCLUDED.title,
			    asset_url = EXCLUDED.asset_url,
			    meta = EXCLUDED.meta
		`, item.Slug, item.Type, item.Title, item.AssetURL, string(metaJSON)); err != nil {
			return fmt.Errorf("upsert inventory item %s: %w", item.Slug, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_inventory_selection
		SET piece_skin_slug = NULL
		WHERE piece_skin_slug IS NOT NULL
		  AND piece_skin_slug <> ALL($1)
	`, allowedSlugs); err != nil {
		return fmt.Errorf("clear obsolete piece selections: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_inventory_selection
		SET board_skin_slug = NULL
		WHERE board_skin_slug IS NOT NULL
		  AND board_skin_slug <> ALL($1)
	`, allowedSlugs); err != nil {
		return fmt.Errorf("clear obsolete board selections: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_inventory_selection
		SET emote_slugs = COALESCE((
				SELECT array_agg(slug)
				FROM unnest(COALESCE(emote_slugs, sticker_slugs, '{}'::text[])) AS slug
				WHERE slug = ANY($1)
			), '{}'::text[]),
		    sticker_slugs = COALESCE((
				SELECT array_agg(slug)
				FROM unnest(COALESCE(emote_slugs, sticker_slugs, '{}'::text[])) AS slug
				WHERE slug = ANY($1)
			), '{}'::text[]),
		    updated_at = now()
	`, allowedEmoteSlugs); err != nil {
		return fmt.Errorf("normalize selected emotes: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM shop_items
		WHERE item_slug <> ALL($1)
	`, allowedSlugs); err != nil {
		return fmt.Errorf("delete obsolete shop items: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM inventory_default_items
		WHERE item_slug <> ALL($1)
	`, defaultOwnedSlugs); err != nil {
		return fmt.Errorf("delete obsolete default items: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM user_inventory_items
		WHERE item_slug <> ALL($1)
	`, allowedSlugs); err != nil {
		return fmt.Errorf("delete obsolete owned items: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM inventory_items
		WHERE slug <> ALL($1)
	`, allowedSlugs); err != nil {
		return fmt.Errorf("delete obsolete catalog items: %w", err)
	}

	for _, slug := range defaultOwnedSlugs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_default_items (item_slug)
			VALUES ($1)
			ON CONFLICT (item_slug) DO NOTHING
		`, slug); err != nil {
			return fmt.Errorf("upsert default item %s: %w", slug, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_inventory_items (user_id, item_slug)
		SELECT u.id, d.item_slug
		FROM users u
		CROSS JOIN inventory_default_items d
		ON CONFLICT (user_id, item_slug) DO NOTHING
	`); err != nil {
		return fmt.Errorf("grant default inventory: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_inventory_selection (
			user_id,
			piece_skin_slug,
			board_skin_slug,
			emote_slugs,
			sticker_slugs
		)
		SELECT u.id, $1, $2, $3::text[], $3::text[]
		FROM users u
		ON CONFLICT (user_id) DO NOTHING
	`, defaultPieceSkinSlug, defaultBoardSkinSlug, defaultEmoteSlugs); err != nil {
		return fmt.Errorf("init inventory selection: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_inventory_selection
		SET piece_skin_slug = COALESCE(piece_skin_slug, $1),
		    board_skin_slug = COALESCE(board_skin_slug, $2),
		    emote_slugs = CASE
		        WHEN coalesce(array_length(emote_slugs, 1), 0) = 0 THEN $3::text[]
		        ELSE emote_slugs
		    END,
		    sticker_slugs = CASE
		        WHEN coalesce(array_length(sticker_slugs, 1), 0) = 0 THEN $3::text[]
		        ELSE sticker_slugs
		    END,
		    updated_at = now()
	`, defaultPieceSkinSlug, defaultBoardSkinSlug, defaultEmoteSlugs); err != nil {
		return fmt.Errorf("finalize inventory selection: %w", err)
	}

	shopSlugs := make([]string, 0, len(shopSeed))
	for _, item := range shopSeed {
		shopSlugs = append(shopSlugs, item.Slug)
		if _, err := tx.Exec(ctx, `
			INSERT INTO shop_items (item_slug, price_shop_currency, is_active)
			VALUES ($1, $2, true)
			ON CONFLICT (item_slug) DO UPDATE
			SET price_shop_currency = EXCLUDED.price_shop_currency,
			    is_active = EXCLUDED.is_active
		`, item.Slug, item.Price); err != nil {
			return fmt.Errorf("upsert shop item %s: %w", item.Slug, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM shop_items
		WHERE item_slug <> ALL($1)
	`, shopSlugs); err != nil {
		return fmt.Errorf("trim shop catalog: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit store catalog sync: %w", err)
	}

	return nil
}
