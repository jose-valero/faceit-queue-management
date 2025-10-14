-- +goose Up
-- 1) Deduplicar (si existieran entradas duplicadas por guild+discord, conservar la más reciente)
WITH ranked AS (
  SELECT
    ctid,
    guild_id,
    discord_user_id,
    ROW_NUMBER() OVER (
      PARTITION BY guild_id, discord_user_id
      ORDER BY COALESCE(updated_at, linked_at, now()) DESC
    ) AS rn
  FROM user_links
  WHERE deleted_at IS NULL OR deleted_at IS NOT NULL
)
DELETE FROM user_links u
USING ranked r
WHERE u.ctid = r.ctid
  AND r.rn > 1;

-- 2) Agregar la restricción única para ON CONFLICT
ALTER TABLE user_links
  ADD CONSTRAINT user_links_guild_discord_uniq
  UNIQUE (guild_id, discord_user_id);

-- 3) (Opcional) Índice auxiliar para GetByDiscordID si consultas mucho por discord solo
CREATE INDEX IF NOT EXISTS idx_user_links_discord
  ON user_links(discord_user_id);

-- +goose Down
-- Quitar índice e unique (nota: el dedupe no se revierte)
DROP INDEX IF EXISTS idx_user_links_discord;
ALTER TABLE user_links
  DROP CONSTRAINT IF EXISTS user_links_guild_discord_uniq;
