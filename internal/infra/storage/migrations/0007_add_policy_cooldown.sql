-- +goose Up
ALTER TABLE guild_policies
  ADD COLUMN IF NOT EXISTS cooldown_after_loss_seconds integer NOT NULL DEFAULT 120;

-- +goose Down
ALTER TABLE guild_policies
  DROP COLUMN IF EXISTS cooldown_after_loss_seconds;
