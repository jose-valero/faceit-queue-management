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

type Notifier interface {
	// Opcional: si lo seteas, podés mandar un DM o mensaje en canal
	Notify(guildID, discordUserID, msg string)
}

type QueueService struct {
	users    UserRepo
	queue    QueueRepo
	policy   PolicyRepo
	fc       FaceitAPI
	hubID    string
	notifier Notifier
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

func NewQueueService(fc FaceitAPI, users UserRepo, queue QueueRepo, policy PolicyRepo, hubID string) *QueueService {
	return &QueueService{fc: fc, users: users, queue: queue, policy: policy, hubID: hubID}
}

func (s *QueueService) Join(ctx context.Context, guildID, discordID string) (string, error) {
	// 1) Link debe existir (DB local, rápido)
	ul, err := s.users.GetByDiscordID(ctx, discordID)
	if err != nil {
		return "❌ No estás vinculado. Usa `/link nick:<tu_nick_FACEIT>`", nil
	}

	// 2) Escribir en cola YA (no bloqueamos por redes externas)
	already, _ := s.queue.Exists(ctx, guildID, discordID)
	if err := s.queue.Join(ctx, storage.QueueEntry{
		GuildID:       guildID,
		DiscordUserID: discordID,
		FaceitUserID:  ul.FaceitUserID,
		Nickname:      ul.Nickname,
		Status:        "waiting",
	}); err != nil {
		return "", err
	}

	// 3) Disparar validación en background (no bloquea UX)
	go s.validateJoinAsync(guildID, ul)

	// 4) Responder rápido
	if already {
		return fmt.Sprintf("🟡 Ya estabas en la cola, actualicé tu estado: **%s**.", ul.Nickname), nil
	}
	return fmt.Sprintf("✅ %s te uniste a la cola. (validando requisitos…)", ul.Nickname), nil
}

// --- validación asíncrona post-join ---
func (s *QueueService) validateJoinAsync(guildID string, ul storage.UserLink) {
	// límites agresivos: no queremos bloquear nada largo en background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// policy (si falla, usa defaults)
	pol, _ := s.policy.Get(ctx, guildID)
	cd := time.Duration(pol.CooldownAfterLossSeconds) * time.Second
	if cd <= 0 {
		cd = 2 * time.Minute
	}

	// 1) match en curso en el hub → fuera
	if ok, err := s.fc.PlayerInOngoingHub(ctx, ul.FaceitUserID, s.hubID); err == nil && ok {
		// _ = s.queue.Leave(context.Background(), guildID, ul.DiscordUserID)
		s.notify(guildID, ul.DiscordUserID, "⛔ No puedes unirte: estás en una **partida activa del hub**.")
		return
	}

	// 2) cooldown por última derrota → fuera si no cumplió
	if lost, endedAt, err := s.fc.LastMatchLossWithin(ctx, ul.FaceitUserID, "cs2", cd); err == nil && lost {
		wait := time.Until(endedAt.Add(cd))
		if wait > 0 {
			// _ = s.queue.Leave(context.Background(), guildID, ul.DiscordUserID)
			s.notify(guildID, ul.DiscordUserID,
				fmt.Sprintf("⌛ Acabas de **perder** una partida. Debes esperar **%d s** para unirte.", int(wait.Seconds())))
			return
		}
	}

	// 3) membresía si la policy lo exige (refresca snapshots si está “stale”)
	if pol.RequireMember {
		stale := ul.MemberCheckedAt == nil || time.Since(*ul.MemberCheckedAt) > 10*time.Minute
		if stale {
			if ok, err := s.fc.IsMemberOfHub(ctx, ul.FaceitUserID, s.hubID); err == nil {
				now := time.Now()
				var eloPtr, skillPtr *int
				// snapshots si están nulos o vencidos (>24h)
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
			// _ = s.queue.Leave(context.Background(), guildID, ul.DiscordUserID)
			s.notify(guildID, ul.DiscordUserID, "❌ Debes ser **miembro del Club** en FACEIT para unirte a la cola.")
			return
		}
	}

	// si llegó hasta acá, mantiene su lugar en la cola
}

func (s *QueueService) notify(guildID, userID, msg string) {
	if s.notifier != nil {
		s.notifier.Notify(guildID, userID, msg)
	}
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
