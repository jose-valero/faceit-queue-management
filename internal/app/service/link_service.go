package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

type LinkService struct {
	fc    FaceitAPI
	users UserRepo
	hubID string
}

func NewLinkService(fc FaceitAPI, users UserRepo, hubID string) *LinkService {
	return &LinkService{fc: fc, users: users, hubID: hubID}
}

func (s *LinkService) DescribeByNick(ctx context.Context, nick string) (string, error) {
	p, err := s.fc.GetPlayerByNickname(ctx, nick, "cs2")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("**%s** — Lvl %d | Elo %d", p.Nickname, p.Skill, p.Elo), nil
}

func (s *LinkService) Link(ctx context.Context, nick, discordID, guildID string) (string, error) {
	p, err := s.fc.GetPlayerByNickname(ctx, nick, "cs2")
	if err != nil {
		return "", err
	}

	// ¿ya está vinculado este discord en este guild?
	existing, err := s.users.GetByDiscordID(ctx, discordID)
	if err == nil && existing.GuildID == guildID {
		if existing.FaceitUserID == p.ID {
			// revalida membresía + refresca snapshot
			isMember, err := s.fc.IsMemberOfHub(ctx, p.ID, s.hubID)
			if err != nil {
				return "", err
			}
			now := time.Now()
			_ = s.users.UpsertLink(ctx, storage.UserLink{
				FaceitUserID:    p.ID,
				DiscordUserID:   discordID,
				Nickname:        p.Nickname,
				IsMember:        isMember,
				MemberCheckedAt: &now,
				GuildID:         guildID,
			})
			if isMember {
				return "✅ Ya estabas vinculado como **" + p.Nickname + "** y eres **miembro del Club**. ¡Todo listo!", nil
			}
			return "✅ Ya estabas vinculado como **" + p.Nickname + "**, pero **no apareces** como miembro del Club aún. Pide acceso y vuelve a probar.", nil
		}
		// Vinculado a otra cuenta
		return "⚠️ Ya estás vinculado a **" + existing.Nickname + "**. Usa `/unlink` y luego `/link` con la nueva.", nil
	}
	// si err != nil y no es ErrNotFound → propaga
	if err != nil && err != storage.ErrNotFound {
		return "", err
	}

	isMember, err := s.fc.IsMemberOfHub(ctx, p.ID, s.hubID)
	if err != nil {
		return "", err
	}

	now := time.Now()
	if err := s.users.UpsertLink(ctx, storage.UserLink{
		FaceitUserID:    p.ID,
		DiscordUserID:   discordID,
		Nickname:        p.Nickname,
		IsMember:        isMember,
		MemberCheckedAt: &now,
		GuildID:         guildID,
	}); err != nil {
		return "", err
	}

	if isMember {
		return "✅ Vinculado: **" + p.Nickname + "**.\nEres **miembro del Club**. Ya podés unirte a la cola.", nil
	}
	return "✅ Vinculado: **" + p.Nickname + "**.\nNo apareces como **miembro del Club** aún. Pide acceso en FACEIT y vuelve con `/link`.", nil
}

func (s *LinkService) Unlink(ctx context.Context, discordID, guildID string) (string, error) {
	ok, err := s.users.SoftDeleteByDiscordID(ctx, discordID, guildID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "ℹ️ No tenías un link activo en este servidor.", nil
	}
	return "✅ Listo, desvinculado. Usa `/link` cuando quieras volver a vincular.", nil
}

func (s *LinkService) WhoAmI(ctx context.Context, discordID string) (string, error) {
	ul, err := s.users.GetByDiscordID(ctx, discordID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"**Discord:** <@%s>\n**FACEIT:** `%s` (%s)\n**Miembro del Club:** %v\n**Vinculado:** <t:%d:R>",
		ul.DiscordUserID, ul.FaceitUserID, ul.Nickname, ul.IsMember, ul.LinkedAt.Unix(),
	), nil
}
