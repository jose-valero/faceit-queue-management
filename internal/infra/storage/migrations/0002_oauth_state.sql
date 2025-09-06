-- +goose Up
CREATE TABLE IF NOT EXISTS oauth_states (
  state             TEXT PRIMARY KEY,
  discord_user_id   TEXT NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at        TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS oauth_states;
