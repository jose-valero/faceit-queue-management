-- +goose Up
CREATE TABLE IF NOT EXISTS match_voice_rooms (
  match_id         text PRIMARY KEY,
  guild_id         text NOT NULL,
  category_id      text NOT NULL,
  team1_channel_id text NOT NULL,
  team2_channel_id text NOT NULL,
  team1_label      text,
  team2_label      text,
  last_status      text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  expires_at       timestamptz
);

-- +goose Down
DROP TABLE IF EXISTS match_voice_rooms;
