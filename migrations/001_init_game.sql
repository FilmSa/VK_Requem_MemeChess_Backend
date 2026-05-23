CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY,
    email varchar UNIQUE,
    password_hash varchar,
    username varchar UNIQUE NOT NULL,
    avatar_url varchar,
    created_at timestamp NOT NULL DEFAULT now()
    );

CREATE TABLE IF NOT EXISTS games (
    id uuid PRIMARY KEY,
    player1_id uuid NOT NULL REFERENCES users(id),
    player2_id uuid NOT NULL REFERENCES users(id),
    winner_id uuid NULL REFERENCES users(id),

    status varchar NOT NULL,
    bet_amount bigint NOT NULL DEFAULT 0,
    currency varchar NOT NULL DEFAULT 'cash',
    meme_mode boolean NOT NULL DEFAULT false,

    fen text NOT NULL,
    current_turn_user_id uuid NOT NULL REFERENCES users(id),

    started_at timestamp NULL,
    finished_at timestamp NULL,
    created_at timestamp NOT NULL DEFAULT now()
    );

CREATE TABLE IF NOT EXISTS moves (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    player_id uuid NOT NULL REFERENCES users(id),
    move_number integer NOT NULL,
    move varchar NOT NULL,
    fen text NOT NULL,
    is_capture boolean NOT NULL DEFAULT false,
    is_check boolean NOT NULL DEFAULT false,
    is_checkmate boolean NOT NULL DEFAULT false,
    meme_id text NULL,
    meme_category text NULL,
    created_at timestamp NOT NULL DEFAULT now()
    );

CREATE INDEX IF NOT EXISTS idx_moves_game_id ON moves(game_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_moves_game_id_move_number ON moves(game_id, move_number);
