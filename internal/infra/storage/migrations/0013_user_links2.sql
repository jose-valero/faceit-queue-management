-- +goose Up
ALTER TABLE user_links
  ADD COLUMN IF NOT EXISTS elo_snapshot INT;
ALTER TABLE user_links
  ADD COLUMN IF NOT EXISTS skill_level_snapshot INT;
ALTER TABLE user_links
  ADD COLUMN IF NOT EXISTS nickname TEXT;

CREATE INDEX IF NOT EXISTS idx_user_links_guild_discord
  ON user_links(guild_id, discord_user_id);

UPDATE user_links
SET skill_level_snapshot = CASE
  WHEN elo_snapshot >= 2001 THEN 10
  WHEN elo_snapshot >= 1751 THEN 9
  WHEN elo_snapshot >= 1531 THEN 8
  WHEN elo_snapshot >= 1351 THEN 7
  WHEN elo_snapshot >= 1201 THEN 6
  WHEN elo_snapshot >= 1051 THEN 5
  WHEN elo_snapshot >= 901  THEN 4
  WHEN elo_snapshot >= 751  THEN 3
  WHEN elo_snapshot >= 501  THEN 2
  WHEN elo_snapshot >= 100  THEN 1
  ELSE NULL
END
WHERE skill_level_snapshot IS NULL AND elo_snapshot IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_user_links_guild_discord;

ALTER TABLE user_links
  DROP COLUMN IF EXISTS nickname;

ALTER TABLE user_links
  DROP COLUMN IF EXISTS skill_level_snapshot;

ALTER TABLE user_links
  DROP COLUMN IF EXISTS elo_snapshot;
