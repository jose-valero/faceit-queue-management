-- +goose Up
CREATE TABLE IF NOT EXISTS guild_ui (
  guild_id         text PRIMARY KEY,
  queue_channel_id text NOT NULL,
  queue_message_id text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS guild_ui;
