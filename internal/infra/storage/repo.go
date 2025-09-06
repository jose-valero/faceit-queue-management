package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type UserLink struct {
	FaceitUserID       string
	DiscordUserID      string
	Nickname           string
	LinkedAt           time.Time
	IsMember           bool
	MemberCheckedAt    *time.Time
	EloSnapshot        *int
	SkillLevelSnapshot *int
	GuildID            string
}

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

var ErrNotFound = errors.New("not found")

// Upsert por faceit_user_id; mantiene discord_id único.
func (r *UserRepo) UpsertLink(ctx context.Context, ul UserLink) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO user_links
  (faceit_user_id, discord_user_id, nickname, is_member, member_checked_at, elo_snapshot, skill_level_snapshot, guild_id, deleted_at)
VALUES
  ($1,$2,$3,$4,$5,$6,$7,$8,NULL)
ON CONFLICT (faceit_user_id) DO UPDATE SET
  discord_user_id = EXCLUDED.discord_user_id,
  nickname        = EXCLUDED.nickname,
  is_member       = EXCLUDED.is_member,
  member_checked_at = EXCLUDED.member_checked_at,
  elo_snapshot    = EXCLUDED.elo_snapshot,
  skill_level_snapshot = EXCLUDED.skill_level_snapshot,
  guild_id        = EXCLUDED.guild_id,
  deleted_at      = NULL
`, ul.FaceitUserID, ul.DiscordUserID, ul.Nickname, ul.IsMember, ul.MemberCheckedAt, ul.EloSnapshot, ul.SkillLevelSnapshot, ul.GuildID)
	return err
}

func (r *UserRepo) GetByDiscordID(ctx context.Context, discordID string) (UserLink, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT faceit_user_id, discord_user_id, nickname, linked_at, is_member, member_checked_at,
       elo_snapshot, skill_level_snapshot, guild_id
FROM user_links
WHERE discord_user_id = $1 AND deleted_at IS NULL
`, discordID)
	var ul UserLink
	err := row.Scan(&ul.FaceitUserID, &ul.DiscordUserID, &ul.Nickname, &ul.LinkedAt, &ul.IsMember, &ul.MemberCheckedAt,
		&ul.EloSnapshot, &ul.SkillLevelSnapshot, &ul.GuildID)
	if err == sql.ErrNoRows {
		return UserRepo{}.zero(), ErrNotFound
	}
	return ul, err
}

func (r *UserRepo) SoftDeleteByDiscordID(ctx context.Context, discordID, guildID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
UPDATE user_links
   SET deleted_at = NOW()
 WHERE discord_user_id = $1
   AND guild_id       = $2
   AND deleted_at IS NULL
`, discordID, guildID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (UserRepo) zero() UserLink { return UserLink{} }

// internal/infra/storage/repo.go

func (r *UserRepo) UpdateMembershipByFaceitID(ctx context.Context, faceitUserID string, isMember bool) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE user_links
   SET is_member = $1,
       member_checked_at = NOW()
 WHERE faceit_user_id = $2
   AND deleted_at IS NULL
`, isMember, faceitUserID)
	if err != nil {
		return err
	}

	// Opcional: si querés saber si no había fila (p.ej. el user aún no hizo /link)
	if n, _ := res.RowsAffected(); n == 0 {
		// no hacemos nada; puede ser un webhook de alguien que todavía no se linkeó
		// log.Printf("webhook: no row to update for player=%s (not linked yet)", faceitUserID)
	}
	return nil
}
