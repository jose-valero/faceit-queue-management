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
INSERT INTO user_links (
  guild_id, discord_user_id, faceit_user_id, nickname,
  elo_snapshot, skill_level_snapshot, is_member, member_checked_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
ON CONFLICT (guild_id, discord_user_id) DO UPDATE SET
  deleted_at           = NULL,  -- 🔴 re-activa si estaba soft-deleted
  nickname             = COALESCE(NULLIF(EXCLUDED.nickname, ''), user_links.nickname),
  elo_snapshot         = COALESCE(EXCLUDED.elo_snapshot,         user_links.elo_snapshot),
  skill_level_snapshot = COALESCE(
                           EXCLUDED.skill_level_snapshot,
                           CASE
                             WHEN EXCLUDED.elo_snapshot IS NOT NULL THEN
                               CASE
                                 WHEN EXCLUDED.elo_snapshot >= 2001 THEN 10
                                 WHEN EXCLUDED.elo_snapshot >= 1751 THEN 9
                                 WHEN EXCLUDED.elo_snapshot >= 1531 THEN 8
                                 WHEN EXCLUDED.elo_snapshot >= 1351 THEN 7
                                 WHEN EXCLUDED.elo_snapshot >= 1201 THEN 6
                                 WHEN EXCLUDED.elo_snapshot >= 1051 THEN 5
                                 WHEN EXCLUDED.elo_snapshot >= 901  THEN 4
                                 WHEN EXCLUDED.elo_snapshot >= 751  THEN 3
                                 WHEN EXCLUDED.elo_snapshot >= 501  THEN 2
                                 WHEN EXCLUDED.elo_snapshot >= 100  THEN 1
                                 ELSE NULL
                               END
                             ELSE user_links.skill_level_snapshot
                           END
                         ),
  is_member            = COALESCE(EXCLUDED.is_member,            user_links.is_member),
  member_checked_at    = GREATEST(user_links.member_checked_at,  EXCLUDED.member_checked_at),
  linked_at            = COALESCE(user_links.linked_at, now()),
  updated_at           = now()
`,
		ul.GuildID,
		ul.DiscordUserID,
		ul.FaceitUserID,
		ul.Nickname,
		ul.EloSnapshot,
		ul.SkillLevelSnapshot,
		ul.IsMember,
		ul.MemberCheckedAt,
	)
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
