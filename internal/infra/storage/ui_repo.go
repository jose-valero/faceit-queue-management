package storage

import (
	"context"
	"database/sql"
	"time"
)

type GuildUI struct {
	GuildID        string
	QueueChannelID string
	QueueMessageID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UIRepo struct{ db *sql.DB }

func NewUIRepo(db *sql.DB) *UIRepo { return &UIRepo{db: db} }

func (r *UIRepo) Get(ctx context.Context, guildID string) (GuildUI, error) {
	var u GuildUI
	err := r.db.QueryRowContext(ctx, `
SELECT guild_id, queue_channel_id, queue_message_id, created_at, updated_at
  FROM guild_ui
 WHERE guild_id = $1
`, guildID).Scan(&u.GuildID, &u.QueueChannelID, &u.QueueMessageID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *UIRepo) Upsert(ctx context.Context, guildID, channelID, messageID string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO guild_ui (guild_id, queue_channel_id, queue_message_id)
VALUES ($1,$2,$3)
ON CONFLICT (guild_id) DO UPDATE SET
  queue_channel_id = EXCLUDED.queue_channel_id,
  queue_message_id = EXCLUDED.queue_message_id,
  updated_at       = now()
`, guildID, channelID, messageID)
	return err
}
