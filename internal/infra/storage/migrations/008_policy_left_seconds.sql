-- +goose Up
ALTER TABLE guild_policies ADD COLUMN drop_if_left_seconds integer NOT NULL DEFAULT 300;
UPDATE guild_policies SET drop_if_left_seconds = COALESCE(drop_if_left_minutes, 5) * 60;
ALTER TABLE guild_policies DROP COLUMN drop_if_left_minutes;

-- +goose Down
ALTER TABLE guild_policies ADD COLUMN drop_if_left_minutes integer NOT NULL DEFAULT 5;
UPDATE guild_policies SET drop_if_left_minutes = GREATEST(1, COALESCE(drop_if_left_seconds, 300) / 60);
ALTER TABLE guild_policies DROP COLUMN drop_if_left_seconds;
