ALTER TABLE users
    ALTER COLUMN shop_currency SET DEFAULT 1000;

UPDATE users
SET shop_currency = 1000
WHERE shop_currency = 0;

ALTER TABLE user_inventory_selection
    ADD COLUMN IF NOT EXISTS emote_slugs text[] NOT NULL DEFAULT '{}'::text[];

UPDATE user_inventory_selection
SET emote_slugs = sticker_slugs
WHERE coalesce(array_length(emote_slugs, 1), 0) = 0
  AND coalesce(array_length(sticker_slugs, 1), 0) > 0;

ALTER TABLE user_inventory_selection
    DROP CONSTRAINT IF EXISTS chk_user_inventory_selection_stickers_len;

ALTER TABLE user_inventory_selection
    DROP CONSTRAINT IF EXISTS chk_user_inventory_selection_emotes_len;

ALTER TABLE user_inventory_selection
    ADD CONSTRAINT chk_user_inventory_selection_emotes_len
    CHECK (coalesce(array_length(emote_slugs, 1), 0) <= 3);
