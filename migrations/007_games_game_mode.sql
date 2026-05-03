ALTER TABLE games
    ADD COLUMN IF NOT EXISTS game_mode varchar NOT NULL DEFAULT 'classic';

UPDATE games
SET game_mode = CASE
    WHEN meme_mode THEN 'meme'
    ELSE 'classic'
END
WHERE game_mode = 'classic';
