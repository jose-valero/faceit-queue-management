-- +goose Up
-- quita la unique global (nombre por defecto de Postgres)
ALTER TABLE user_links DROP CONSTRAINT IF EXISTS user_links_discord_user_id_key;

-- índice único por guild y sólo para filas activas (deleted_at IS NULL)
CREATE UNIQUE INDEX IF NOT EXISTS uniq_user_links_guild_discord_active
ON user_links (guild_id, discord_user_id)
WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS uniq_user_links_guild_discord_active;

ALTER TABLE user_links
  ADD CONSTRAINT user_links_discord_user_id_key UNIQUE (discord_user_id);
