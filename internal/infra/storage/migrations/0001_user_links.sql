-- +goose Up
CREATE TABLE IF NOT EXISTS user_links (
  faceit_user_id           TEXT PRIMARY KEY,
  discord_user_id          TEXT NOT NULL UNIQUE,
  nickname                 TEXT NOT NULL,
  linked_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  is_member                BOOLEAN NOT NULL DEFAULT FALSE,
  member_checked_at        TIMESTAMPTZ,
  elo_snapshot             INTEGER,
  skill_level_snapshot     INTEGER,
  guild_id                 TEXT NOT NULL,
  deleted_at               TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_user_links_guild ON user_links (guild_id);

-- +goose Down
DROP TABLE IF EXISTS user_links;
