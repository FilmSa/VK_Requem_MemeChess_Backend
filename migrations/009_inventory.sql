-- Inventory: catalog, ownership, selection
-- Types:
-- - piece_skin: skin pack for pieces
-- - board_skin: board skin
-- - sticker: sticker asset (typically mp4)

CREATE TABLE IF NOT EXISTS inventory_items (
    slug text PRIMARY KEY,
    type text NOT NULL,
    title text NULL,
    asset_url text NULL,
    meta jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamp NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inventory_items_type ON inventory_items(type);

CREATE TABLE IF NOT EXISTS user_inventory_items (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_slug text NOT NULL REFERENCES inventory_items(slug) ON DELETE RESTRICT,
    acquired_at timestamp NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, item_slug)
);

CREATE INDEX IF NOT EXISTS idx_user_inventory_items_user_id ON user_inventory_items(user_id);

CREATE TABLE IF NOT EXISTS user_inventory_selection (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    piece_skin_slug text NULL REFERENCES inventory_items(slug) ON DELETE SET NULL,
    board_skin_slug text NULL REFERENCES inventory_items(slug) ON DELETE SET NULL,
    sticker_slugs text[] NOT NULL DEFAULT '{}'::text[],
    updated_at timestamp NOT NULL DEFAULT now(),
    CONSTRAINT chk_user_inventory_selection_stickers_len CHECK (coalesce(array_length(sticker_slugs, 1), 0) <= 3)
);

CREATE TABLE IF NOT EXISTS inventory_default_items (
    item_slug text PRIMARY KEY REFERENCES inventory_items(slug) ON DELETE CASCADE
);

INSERT INTO inventory_items (slug, type, title, asset_url)
VALUES
    ('piece.classic', 'piece_skin', 'Classic pieces', NULL),
    ('board.classic', 'board_skin', 'Classic board', NULL),
    ('sticker.laugh', 'sticker', 'Laugh', '/assets/emotes/laugh.mp4'),
    ('sticker.cry', 'sticker', 'Cry', '/assets/emotes/cry.mp4'),
    ('sticker.wow', 'sticker', 'Wow', '/assets/emotes/wow.mp4')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO inventory_default_items (item_slug)
VALUES
    ('piece.classic'),
    ('board.classic'),
    ('sticker.laugh'),
    ('sticker.cry'),
    ('sticker.wow')
ON CONFLICT (item_slug) DO NOTHING;

INSERT INTO user_inventory_items (user_id, item_slug)
SELECT u.id, d.item_slug
FROM users u
JOIN inventory_default_items d ON true
ON CONFLICT (user_id, item_slug) DO NOTHING;

INSERT INTO user_inventory_selection (user_id, piece_skin_slug, board_skin_slug, sticker_slugs)
SELECT u.id, 'piece.classic', 'board.classic', '{}'::text[]
FROM users u
ON CONFLICT (user_id) DO NOTHING;

-- Auto-grant defaults + init selection for newly created users
CREATE OR REPLACE FUNCTION grant_default_inventory_to_user()
RETURNS trigger AS $$
BEGIN
    INSERT INTO user_inventory_items (user_id, item_slug)
    SELECT NEW.id, item_slug FROM inventory_default_items
    ON CONFLICT (user_id, item_slug) DO NOTHING;

    INSERT INTO user_inventory_selection (user_id, piece_skin_slug, board_skin_slug, sticker_slugs)
    VALUES (NEW.id, 'piece.classic', 'board.classic', '{}'::text[])
    ON CONFLICT (user_id) DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_default_inventory ON users;
CREATE TRIGGER trg_users_default_inventory
AFTER INSERT ON users
FOR EACH ROW
EXECUTE FUNCTION grant_default_inventory_to_user();

