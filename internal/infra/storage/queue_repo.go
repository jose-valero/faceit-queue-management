package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type QueueRepo struct{ db *sql.DB }

func NewQueueRepo(db *sql.DB) *QueueRepo { return &QueueRepo{db: db} }

// Join: inserta o refresca (upsert). Siempre deja status=waiting y last_seen=now().
func (r *QueueRepo) Join(ctx context.Context, e QueueEntry) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO queue_entries (guild_id, discord_user_id, faceit_user_id, nickname, status)
VALUES ($1,$2,$3,$4,'waiting')
ON CONFLICT (guild_id, discord_user_id) DO UPDATE SET
  faceit_user_id = EXCLUDED.faceit_user_id,
  nickname       = EXCLUDED.nickname,
  status         = 'waiting',
  last_seen_at   = now()
`,
		e.GuildID, e.DiscordUserID, e.FaceitUserID, e.Nickname,
	)
	return err
}

func (r *QueueRepo) Leave(ctx context.Context, guildID, discordID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
DELETE FROM queue_entries
 WHERE guild_id = $1 AND discord_user_id = $2
`, guildID, discordID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *QueueRepo) List(ctx context.Context, guildID string, limit int) ([]QueueEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  qe.guild_id,
  qe.discord_user_id,
  qe.faceit_user_id,
  COALESCE(qe.nickname, ul.nickname) AS nickname,
  qe.joined_at,
  qe.last_seen_at,
  qe.status,
  COALESCE(
    ul.skill_level_snapshot,
    CASE
      WHEN ul.elo_snapshot >= 2001 THEN 10
      WHEN ul.elo_snapshot >= 1751 THEN 9
      WHEN ul.elo_snapshot >= 1531 THEN 8
      WHEN ul.elo_snapshot >= 1351 THEN 7
      WHEN ul.elo_snapshot >= 1201 THEN 6
      WHEN ul.elo_snapshot >= 1051 THEN 5
      WHEN ul.elo_snapshot >= 901  THEN 4
      WHEN ul.elo_snapshot >= 751  THEN 3
      WHEN ul.elo_snapshot >= 501  THEN 2
      WHEN ul.elo_snapshot >= 100  THEN 1
      ELSE NULL
    END
  ) AS skill_level_snapshot
FROM queue_entries qe
LEFT JOIN user_links ul
  ON ul.guild_id = qe.guild_id
 AND ul.discord_user_id = qe.discord_user_id
WHERE qe.guild_id = $1
  AND qe.status   = 'waiting'
ORDER BY qe.joined_at ASC
LIMIT $2
`, guildID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueueEntry
	for rows.Next() {
		var e QueueEntry
		if err := rows.Scan(
			&e.GuildID, &e.DiscordUserID, &e.FaceitUserID, &e.Nickname,
			&e.JoinedAt, &e.LastSeenAt, &e.Status,
			&e.SkillLevelSnapshot, // <- NUEVO: escaneamos snapshot
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *QueueRepo) TouchValid(ctx context.Context, guildID, discordID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE queue_entries
   SET last_seen_at = now(), status = 'waiting'
 WHERE guild_id = $1 AND discord_user_id = $2
`, guildID, discordID)
	return err
}

func (r *QueueRepo) MarkLeft(ctx context.Context, guildID, discordID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE queue_entries
   SET last_seen_at = now(), status = 'left'
 WHERE guild_id = $1 AND discord_user_id = $2
`, guildID, discordID)
	return err
}

func (r *QueueRepo) MarkAFK(ctx context.Context, guildID, discordID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE queue_entries
   SET last_seen_at = now(), status = 'afk'
 WHERE guild_id = $1 AND discord_user_id = $2
`, guildID, discordID)
	return err
}

// Prune: elimina definitvamente segun ventanas de gracia para AFK/LEFT.
func (r *QueueRepo) Prune(ctx context.Context, guildID string, afk, left time.Duration) (int64, int64, error) {
	var nAfk, nLeft int64

	if afk > 0 {
		res, err := r.db.ExecContext(ctx, `
DELETE FROM queue_entries
 WHERE guild_id = $1
   AND status   = 'afk'
   AND last_seen_at <= now() - $2::interval
`, guildID, durToInterval(afk))
		if err != nil {
			return 0, 0, err
		}
		n, _ := res.RowsAffected()
		nAfk = n
	}

	if left > 0 {
		res, err := r.db.ExecContext(ctx, `
DELETE FROM queue_entries
 WHERE guild_id = $1
   AND status   = 'left'
   AND last_seen_at <= now() - $2::interval
`, guildID, durToInterval(left))
		if err != nil {
			return nAfk, 0, err
		}
		n, _ := res.RowsAffected()
		nLeft = n
	}

	return nAfk, nLeft, nil
}

// ListWithGrace devuelve waiting + (afk dentro de graceAFK) + (left dentro de graceLeft)
func (r *QueueRepo) ListWithGrace(ctx context.Context, guildID string, limit int, graceAFK, graceLeft time.Duration) ([]QueueEntry, error) {
	conds := []string{"qe.status = 'waiting'"}

	args := []any{guildID}
	i := 2

	if graceAFK > 0 {
		conds = append(conds, fmt.Sprintf("(qe.status = 'afk'  AND qe.last_seen_at > now() - $%d::interval)", i))
		args = append(args, durToInterval(graceAFK))
		i++
	}
	if graceLeft > 0 {
		conds = append(conds, fmt.Sprintf("(qe.status = 'left' AND qe.last_seen_at > now() - $%d::interval)", i))
		args = append(args, durToInterval(graceLeft))
		i++
	}
	where := " AND (" + strings.Join(conds, " OR ") + ")"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, `
SELECT
  qe.guild_id,
  qe.discord_user_id,
  qe.faceit_user_id,
  COALESCE(qe.nickname, ul.nickname) AS nickname,
  qe.joined_at,
  qe.last_seen_at,
  qe.status,
  COALESCE(
    ul.skill_level_snapshot,
    CASE
      WHEN ul.elo_snapshot >= 2001 THEN 10
      WHEN ul.elo_snapshot >= 1751 THEN 9
      WHEN ul.elo_snapshot >= 1531 THEN 8
      WHEN ul.elo_snapshot >= 1351 THEN 7
      WHEN ul.elo_snapshot >= 1201 THEN 6
      WHEN ul.elo_snapshot >= 1051 THEN 5
      WHEN ul.elo_snapshot >= 901  THEN 4
      WHEN ul.elo_snapshot >= 751  THEN 3
      WHEN ul.elo_snapshot >= 501  THEN 2
      WHEN ul.elo_snapshot >= 100  THEN 1
      ELSE NULL
    END
  ) AS skill_level_snapshot
FROM queue_entries qe
LEFT JOIN user_links ul
  ON ul.guild_id = qe.guild_id
 AND ul.discord_user_id = qe.discord_user_id
WHERE qe.guild_id = $1
`+where+`
ORDER BY qe.joined_at ASC
LIMIT $`+fmt.Sprint(i), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueueEntry
	for rows.Next() {
		var e QueueEntry
		if err := rows.Scan(
			&e.GuildID, &e.DiscordUserID, &e.FaceitUserID, &e.Nickname,
			&e.JoinedAt, &e.LastSeenAt, &e.Status,
			&e.SkillLevelSnapshot,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *QueueRepo) Exists(ctx context.Context, guildID, discordID string) (bool, error) {
	var x int
	err := r.db.QueryRowContext(ctx, `
SELECT 1
  FROM queue_entries
 WHERE guild_id = $1 AND discord_user_id = $2
`, guildID, discordID).Scan(&x)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func durToInterval(d time.Duration) string {
	secs := int64(d.Seconds())
	if secs <= 0 {
		return "0 seconds"
	}
	return fmt.Sprintf("%d seconds", secs)
}
