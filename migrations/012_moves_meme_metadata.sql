ALTER TABLE moves
    ADD COLUMN IF NOT EXISTS meme_id text,
    ADD COLUMN IF NOT EXISTS meme_category text;
