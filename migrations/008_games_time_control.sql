ALTER TABLE games
    ADD COLUMN IF NOT EXISTS time_control_id varchar NULL,
    ADD COLUMN IF NOT EXISTS time_control_base_ms bigint NULL,
    ADD COLUMN IF NOT EXISTS time_control_increment_ms bigint NULL,
    ADD COLUMN IF NOT EXISTS player1_remaining_ms bigint NULL,
    ADD COLUMN IF NOT EXISTS player2_remaining_ms bigint NULL,
    ADD COLUMN IF NOT EXISTS current_turn_started_at timestamp NULL;
