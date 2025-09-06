package storage

import (
	"context"
	"database/sql"
	"fmt"
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
SELECT guild_id, discord_user_id, faceit_user_id, nickname, joined_at, last_seen_at, status
  FROM queue_entries
 WHERE guild_id = $1 AND status = 'waiting'
 ORDER BY joined_at ASC
 LIMIT $2
`, guildID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueueEntry
	for rows.Next() {
		var e QueueEntry
		if err := rows.Scan(&e.GuildID, &e.DiscordUserID, &e.FaceitUserID, &e.Nickname, &e.JoinedAt, &e.LastSeenAt, &e.Status); err != nil {
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
   AND last_seen_at < now() - $2::interval
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
   AND last_seen_at < now() - $2::interval
`, guildID, durToInterval(left))
		if err != nil {
			return nAfk, 0, err
		}
		n, _ := res.RowsAffected()
		nLeft = n
	}

	return nAfk, nLeft, nil
}

// ListWithGrace devuelve:
// - siempre los 'waiting'
// - los 'afk' cuya last_seen esté dentro de graceAFK (si graceAFK > 0) | esto no se si hacerlo
// - los 'left' cuya last_seen esté dentro de graceLeft (si graceLeft > 0) | por ahora mostramos
func (r *QueueRepo) ListWithGrace(ctx context.Context, guildID string, limit int, graceAFK, graceLeft time.Duration) ([]QueueEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT guild_id, discord_user_id, faceit_user_id, nickname, joined_at, last_seen_at, status
  FROM queue_entries
 WHERE guild_id = $1
   AND (
         status = 'waiting'
      OR ( $2::interval > '0 seconds'::interval AND status = 'afk'  AND last_seen_at > now() - $2::interval )
      OR ( $3::interval > '0 seconds'::interval AND status = 'left' AND last_seen_at > now() - $3::interval )
   )
 ORDER BY joined_at ASC
 LIMIT $4
`, guildID, durToInterval(graceAFK), durToInterval(graceLeft), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueueEntry
	for rows.Next() {
		var e QueueEntry
		if err := rows.Scan(&e.GuildID, &e.DiscordUserID, &e.FaceitUserID, &e.Nickname, &e.JoinedAt, &e.LastSeenAt, &e.Status); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func durToInterval(d time.Duration) string {
	secs := int64(d.Seconds())
	if secs <= 0 {
		return "0 seconds"
	}
	return fmt.Sprintf("%d seconds", secs)
}
