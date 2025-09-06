-- +goose Up
CREATE TABLE IF NOT EXISTS guild_policies (
  guild_id              text PRIMARY KEY,
  require_member        boolean NOT NULL DEFAULT true,
  afk_timeout_seconds   integer NOT NULL DEFAULT 300,
  drop_if_left_minutes  integer NOT NULL DEFAULT 5,
  voice_required        boolean NOT NULL DEFAULT false,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS queue_entries (
  guild_id        text NOT NULL,
  discord_user_id text NOT NULL,
  faceit_user_id  text NOT NULL,
  nickname        text NOT NULL,
  joined_at       timestamptz NOT NULL DEFAULT now(),
  last_seen_at    timestamptz NOT NULL DEFAULT now(),
  status          text NOT NULL DEFAULT 'waiting',
  PRIMARY KEY (guild_id, discord_user_id)
);

CREATE INDEX IF NOT EXISTS idx_queue_entries_guild_joined
  ON queue_entries (guild_id, joined_at);

-- +goose Down
DROP TABLE IF EXISTS queue_entries;
DROP TABLE IF EXISTS guild_policies;
