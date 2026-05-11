CREATE TABLE IF NOT EXISTS shop_items (
    item_slug text PRIMARY KEY REFERENCES inventory_items(slug) ON DELETE CASCADE,
    price_shop_currency bigint NOT NULL CHECK (price_shop_currency >= 0),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_shop_items_active ON shop_items(is_active);

INSERT INTO shop_items (item_slug, price_shop_currency, is_active)
VALUES
    ('sticker.laugh', 100, true),
    ('sticker.cry', 100, true),
    ('sticker.wow', 100, true),
    ('board.classic', 0, false),
    ('piece.classic', 0, false)
ON CONFLICT (item_slug) DO NOTHING;

