package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// Al tope del archivo (importa "time" ya lo tenés)
type QueueItemRich struct {
	DiscordUserID string
	FaceitUserID  string
	Nickname      string
	Status        string
	SkillLevel    *int // snapshot; puede ser nil
	JoinedAt      time.Time
	LastSeenAt    time.Time
}

func (s *QueueService) ListRich(ctx context.Context, guildID string, limit int) ([]QueueItemRich, error) {
	base, err := s.queue.List(ctx, guildID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]QueueItemRich, 0, len(base))
	for _, it := range base {
		qi := QueueItemRich{
			DiscordUserID: it.DiscordUserID,
			FaceitUserID:  it.FaceitUserID,
			Nickname:      it.Nickname,
			Status:        it.Status,
			JoinedAt:      it.JoinedAt,
			LastSeenAt:    it.LastSeenAt,
		}
		if ul, err := s.users.GetByDiscordID(ctx, it.DiscordUserID); err == nil {
			qi.SkillLevel = ul.SkillLevelSnapshot
			if ul.Nickname != "" {
				qi.Nickname = ul.Nickname
			}
		}
		out = append(out, qi)
	}
	// por si acaso
	sort.SliceStable(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}

type QueueService struct {
	users  UserRepo
	queue  QueueRepo
	policy PolicyRepo
	fc     FaceitAPI
	hubID  string
}

func NewQueueService(fc FaceitAPI, users UserRepo, queue QueueRepo, policy PolicyRepo, hubID string) *QueueService {
	return &QueueService{fc: fc, users: users, queue: queue, policy: policy, hubID: hubID}
}

func (s *QueueService) Join(ctx context.Context, guildID, discordID string) (string, error) {
	// valida vinculacion
	ul, err := s.users.GetByDiscordID(ctx, discordID)
	if err != nil {
		return "❌ No estás vinculado. Usa `/link nick:<tu_nick_FACEIT>`", nil
	}

	if ok, err := s.fc.PlayerInOngoingHub(ctx, ul.FaceitUserID, s.hubID); err == nil && ok {
		return "⛔ No puedes unirte: estás en una **partida activa del hub**.", nil
	}

	// 2) Cooldown de 2 minutos si su último match fue derrota
	if lost, endedAt, err := s.fc.LastMatchLossWithin(ctx, ul.FaceitUserID, "cs2", 2*time.Minute); err == nil && lost {
		wait := time.Until(endedAt.Add(2 * time.Minute))
		if wait > 0 {
			return fmt.Sprintf("⌛ Acabas de **perder** una partida. Debes esperar **%d segundos** para unirte.", int(wait.Seconds())), nil
		}
	}

	// valida policy
	pol, _ := s.policy.Get(ctx, guildID)

	// revalidacion on-demand si se exige membresía
	if pol.RequireMember {
		stale := ul.MemberCheckedAt == nil || time.Since(*ul.MemberCheckedAt) > 10*time.Minute
		if stale {
			if ok, err := s.fc.IsMemberOfHub(ctx, ul.FaceitUserID, s.hubID); err == nil {
				now := time.Now()

				// refresco de snapshots si estan nulos o viejos
				var eloPtr, skillPtr *int
				snapStale := ul.EloSnapshot == nil || ul.SkillLevelSnapshot == nil ||
					(ul.MemberCheckedAt != nil && time.Since(*ul.MemberCheckedAt) > 24*time.Hour)

				if snapStale {
					if p, e2 := s.fc.GetPlayerByNickname(ctx, ul.Nickname, "cs2"); e2 == nil {
						elo, skill := p.Elo, p.Skill
						eloPtr, skillPtr = &elo, &skill
					}
				}

				_ = s.users.UpsertLink(ctx, storage.UserLink{
					FaceitUserID:       ul.FaceitUserID,
					DiscordUserID:      ul.DiscordUserID,
					Nickname:           ul.Nickname,
					IsMember:           ok,
					MemberCheckedAt:    &now,
					GuildID:            guildID,
					EloSnapshot:        eloPtr,
					SkillLevelSnapshot: skillPtr,
				})
				ul.IsMember = ok
				ul.MemberCheckedAt = &now
				if eloPtr != nil {
					ul.EloSnapshot = eloPtr
				}
				if skillPtr != nil {
					ul.SkillLevelSnapshot = skillPtr
				}
			}
		}
		if !ul.IsMember {
			return "❌ Debes ser **miembro del Club** en FACEIT para unirte a la cola.", nil
		}
	}

	if ok, err := s.fc.PlayerInOngoingHub(ctx, ul.FaceitUserID, s.hubID); err == nil && ok {
		return "⛔ No puedes unirte: estás en una **partida activa del hub**.", nil
	}

	// 2) Cooldown configurable tras derrota
	cd := time.Duration(pol.CooldownAfterLossSeconds) * time.Second
	if cd <= 0 {
		cd = 2 * time.Minute // fallback sano
	}
	if lost, endedAt, err := s.fc.LastMatchLossWithin(ctx, ul.FaceitUserID, "cs2", cd); err == nil && lost {
		wait := time.Until(endedAt.Add(cd))
		if wait > 0 {
			return fmt.Sprintf("⌛ Acabas de **perder** una partida. Debes esperar **%d s** para unirte.", int(wait.Seconds())), nil
		}
	}

	already, _ := s.queue.Exists(ctx, guildID, discordID)
	// unirse / refrescar last_seen
	if err := s.queue.Join(ctx, storage.QueueEntry{
		GuildID:       guildID,
		DiscordUserID: discordID,
		FaceitUserID:  ul.FaceitUserID,
		Nickname:      ul.Nickname,
		Status:        "waiting",
	}); err != nil {
		return "", err
	}

	if already {
		return fmt.Sprintf("🟡 Ya estabas en la cola, actualicé tu estado: **%s**.", ul.Nickname), nil
	}

	return fmt.Sprintf("✅ %s te uniste a la cola.", ul.Nickname), nil
}

func (s *QueueService) Leave(ctx context.Context, guildID, discordID string) (string, error) {
	ok, err := s.queue.Leave(ctx, guildID, discordID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "ℹ️ No estabas en la cola.", nil
	}
	return "✅ Saliste de la cola.", nil
}

func (s *QueueService) Status(ctx context.Context, guildID string) (string, error) {
	// lee policy para calcular ventanas de gracia mostradas
	pol, _ := s.policy.Get(ctx, guildID)
	afkGrace := time.Duration(pol.AFKTimeoutSeconds) * time.Second
	leftGrace := time.Duration(pol.DropIfLeftSeconds) * time.Minute

	items, err := s.queue.ListWithGrace(ctx, guildID, 50, afkGrace, leftGrace)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "ℹ️ La cola está vacía.", nil
	}

	out := "📋 **Cola actual**\n"
	for i, it := range items {
		suf := ""
		switch it.Status {
		case "left":
			// esta dentro del tiempo de gracia de 'left'
			suf = " ·🚶"
		case "afk":
			// nose si mostrar los afk o no(afkGrace > 0)
			suf = " · 😴 *(afk)*"
		}
		out += fmt.Sprintf("%d) <@%s> — **%s** (%s)%s\n", i+1, it.DiscordUserID, it.Nickname, it.Status, suf)
	}
	return out, nil
}

func (s *QueueService) TouchValid(ctx context.Context, guildID, discordID string) error {
	return s.queue.TouchValid(ctx, guildID, discordID)
}

func (s *QueueService) MarkLeft(ctx context.Context, guildID, discordID string) error {
	return s.queue.MarkLeft(ctx, guildID, discordID)
}

func (s *QueueService) MarkAFK(ctx context.Context, guildID, discordID string) error {
	return s.queue.MarkAFK(ctx, guildID, discordID)
}

func (s *QueueService) Prune(ctx context.Context, guildID string, afk, left time.Duration) (int64, int64, error) {
	return s.queue.Prune(ctx, guildID, afk, left)
}

func (s *QueueService) List(ctx context.Context, guildID string, limit int) ([]storage.QueueEntry, error) {
	return s.queue.List(ctx, guildID, limit)
}

func (s *QueueService) ListRichWithGrace(ctx context.Context, guildID string, limit int, graceAFK, graceLeft time.Duration) ([]QueueItemRich, error) {
	base, err := s.queue.ListWithGrace(ctx, guildID, limit, graceAFK, graceLeft)
	if err != nil {
		return nil, err
	}
	out := make([]QueueItemRich, 0, len(base))
	for _, it := range base {
		qi := QueueItemRich{
			DiscordUserID: it.DiscordUserID,
			FaceitUserID:  it.FaceitUserID,
			Nickname:      it.Nickname,
			Status:        it.Status,
			JoinedAt:      it.JoinedAt,
			LastSeenAt:    it.LastSeenAt,
		}
		if ul, err := s.users.GetByDiscordID(ctx, it.DiscordUserID); err == nil {
			qi.SkillLevel = ul.SkillLevelSnapshot
			if ul.Nickname != "" {
				qi.Nickname = ul.Nickname
			}
		}
		out = append(out, qi)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}
