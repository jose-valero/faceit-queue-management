package storage

import (
	"context"
	"database/sql"
	"time"
)

type MatchVoiceRoom struct {
	MatchID        string
	GuildID        string
	CategoryID     string
	Team1ChannelID string
	Team2ChannelID string
	Team1Label     *string
	Team2Label     *string
	LastStatus     *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      *time.Time
}

type MatchRoomsRepo struct{ db *sql.DB }

func NewMatchRoomsRepo(db *sql.DB) *MatchRoomsRepo { return &MatchRoomsRepo{db: db} }

func (r *MatchRoomsRepo) Get(ctx context.Context, matchID string) (MatchVoiceRoom, error) {
	var m MatchVoiceRoom
	err := r.db.QueryRowContext(ctx, `
SELECT match_id, guild_id, category_id, team1_channel_id, team2_channel_id,
       team1_label, team2_label, last_status, created_at, updated_at, expires_at
  FROM match_voice_rooms
 WHERE match_id = $1
`, matchID).Scan(
		&m.MatchID, &m.GuildID, &m.CategoryID, &m.Team1ChannelID, &m.Team2ChannelID,
		&m.Team1Label, &m.Team2Label, &m.LastStatus, &m.CreatedAt, &m.UpdatedAt, &m.ExpiresAt,
	)
	return m, err
}

func (r *MatchRoomsRepo) Upsert(ctx context.Context, m MatchVoiceRoom) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO match_voice_rooms
  (match_id, guild_id, category_id, team1_channel_id, team2_channel_id, team1_label, team2_label, last_status, updated_at, expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),$9)
ON CONFLICT (match_id) DO UPDATE SET
  guild_id=$2, category_id=$3, team1_channel_id=$4, team2_channel_id=$5,
  team1_label=$6, team2_label=$7, last_status=$8, updated_at=now(), expires_at=$9
`,
		m.MatchID, m.GuildID, m.CategoryID, m.Team1ChannelID, m.Team2ChannelID,
		m.Team1Label, m.Team2Label, m.LastStatus, m.ExpiresAt,
	)
	return err
}

func (r *MatchRoomsRepo) UpdateStatus(ctx context.Context, matchID string, status string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE match_voice_rooms SET last_status=$2, updated_at=now() WHERE match_id=$1
`, matchID, status)
	return err
}

func (r *MatchRoomsRepo) Delete(ctx context.Context, matchID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM match_voice_rooms WHERE match_id=$1`, matchID)
	return err
}
