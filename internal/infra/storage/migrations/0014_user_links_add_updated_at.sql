-- +goose Up
ALTER TABLE user_links
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;
UPDATE user_links SET updated_at = now() WHERE updated_at IS NULL;

-- +goose Down
ALTER TABLE user_links
  DROP COLUMN IF EXISTS updated_at;
