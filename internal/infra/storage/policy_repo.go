package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type PolicyRepo struct{ db *sql.DB }

func NewPolicyRepo(db *sql.DB) *PolicyRepo { return &PolicyRepo{db: db} }

func (r *PolicyRepo) Get(ctx context.Context, guildID string) (GuildPolicy, error) {
	var p GuildPolicy
	err := r.db.QueryRowContext(ctx, `
SELECT guild_id, require_member, afk_timeout_seconds, drop_if_left_minutes, voice_required, created_at, updated_at
  FROM guild_policies
 WHERE guild_id = $1
`, guildID).Scan(
		&p.GuildID, &p.RequireMember, &p.AFKTimeoutSeconds, &p.DropIfLeftMinutes, &p.VoiceRequired, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// crea default
		_, err := r.db.ExecContext(ctx, `
INSERT INTO guild_policies (guild_id) VALUES ($1)
`, guildID)
		if err != nil {
			return GuildPolicy{}, err
		}
		return r.Get(ctx, guildID)
	}
	return p, err
}

func (r *PolicyRepo) Update(ctx context.Context, guildID string, u GuildPolicyUpdate) (GuildPolicy, error) {
	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)
	i := 1

	if u.RequireMember != nil {
		sets = append(sets, fmt.Sprintf("require_member = $%d", i))
		args = append(args, *u.RequireMember)
		i++
	}
	if u.VoiceRequired != nil {
		sets = append(sets, fmt.Sprintf("voice_required = $%d", i))
		args = append(args, *u.VoiceRequired)
		i++
	}
	if u.AFKTimeoutSeconds != nil {
		sets = append(sets, fmt.Sprintf("afk_timeout_seconds = $%d", i))
		args = append(args, *u.AFKTimeoutSeconds)
		i++
	}
	if u.DropIfLeftMinutes != nil {
		sets = append(sets, fmt.Sprintf("drop_if_left_minutes = $%d", i))
		args = append(args, *u.DropIfLeftMinutes)
		i++
	}
	if len(sets) == 0 {
		// nada que cambiar
		return r.Get(ctx, guildID)
	}
	sets = append(sets, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	i++

	args = append(args, guildID)

	_, err := r.db.ExecContext(ctx, `
UPDATE guild_policies
   SET `+strings.Join(sets, ", ")+`
 WHERE guild_id = $`+fmt.Sprint(i), args...)
	if err != nil {
		return GuildPolicy{}, err
	}
	return r.Get(ctx, guildID)
}

func (r *PolicyRepo) Upsert(ctx context.Context, p GuildPolicy) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO guild_policies
  (guild_id, require_member, afk_timeout_seconds, drop_if_left_minutes, voice_required, created_at, updated_at)
VALUES
  ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (guild_id) DO UPDATE SET
  require_member        = EXCLUDED.require_member,
  afk_timeout_seconds   = EXCLUDED.afk_timeout_seconds,
  drop_if_left_minutes  = EXCLUDED.drop_if_left_minutes,
  voice_required        = EXCLUDED.voice_required,
  updated_at            = NOW()
`, p.GuildID, p.RequireMember, p.AFKTimeoutSeconds, p.DropIfLeftMinutes, p.VoiceRequired)
	return err
}
