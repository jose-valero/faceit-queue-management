package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// DTO que consume la UI. Fuente única: repos de queue (JOIN a user_links).
type QueueItemRich struct {
	DiscordUserID      string
	FaceitUserID       string
	Nickname           string
	Status             string
	JoinedAt           time.Time
	LastSeenAt         time.Time
	SkillLevelSnapshot *int // nivel 1..10 derivado del ELO o guardado; puede ser nil
}

type Notifier interface {
	Notify(guildID, discordUserID, msg string)
}

type queueUpserter interface {
	UpsertJoin(ctx context.Context, e storage.QueueEntry) (already bool, err error)
}

type QueueService struct {
	users    UserRepo
	queue    QueueRepo
	policy   PolicyRepo
	fc       FaceitAPI
	hubID    string
	notifier Notifier
}

func NewQueueService(fc FaceitAPI, users UserRepo, queue QueueRepo, policy PolicyRepo, hubID string) *QueueService {
	return &QueueService{fc: fc, users: users, queue: queue, policy: policy, hubID: hubID}
}

// ------------------ LISTADOS (sin lookups extra) ------------------

func (s *QueueService) ListRich(ctx context.Context, guildID string, limit int) ([]QueueItemRich, error) {
	base, err := s.queue.List(ctx, guildID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]QueueItemRich, 0, len(base))
	for _, it := range base {
		out = append(out, QueueItemRich{
			DiscordUserID:      it.DiscordUserID,
			FaceitUserID:       it.FaceitUserID,
			Nickname:           it.Nickname,
			Status:             it.Status,
			JoinedAt:           it.JoinedAt,
			LastSeenAt:         it.LastSeenAt,
			SkillLevelSnapshot: it.SkillLevelSnapshot, // <- viene del JOIN en el repo
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}

func (s *QueueService) ListRichWithGrace(ctx context.Context, guildID string, limit int, graceAFK, graceLeft time.Duration) ([]QueueItemRich, error) {
	base, err := s.queue.ListWithGrace(ctx, guildID, limit, graceAFK, graceLeft)
	if err != nil {
		return nil, err
	}
	out := make([]QueueItemRich, 0, len(base))
	for _, it := range base {
		out = append(out, QueueItemRich{
			DiscordUserID:      it.DiscordUserID,
			FaceitUserID:       it.FaceitUserID,
			Nickname:           it.Nickname,
			Status:             it.Status,
			JoinedAt:           it.JoinedAt,
			LastSeenAt:         it.LastSeenAt,
			SkillLevelSnapshot: it.SkillLevelSnapshot, // <- viene del JOIN en el repo
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}

// ------------------ JOIN / LEAVE ------------------

func (s *QueueService) Join(ctx context.Context, guildID, discordID string) (string, error) {
	// 1) Link debe existir (DB local)
	ul, err := s.users.GetByDiscordID(ctx, discordID)
	if err != nil {
		return "❌ No estás vinculado. Usa `/link nick:<tu_nick_FACEIT>`", nil
	}

	entry := storage.QueueEntry{
		GuildID:            guildID,
		DiscordUserID:      discordID,
		FaceitUserID:       ul.FaceitUserID,
		Nickname:           chooseNonEmpty(ul.Nickname, "unknown"),
		Status:             "waiting",
		SkillLevelSnapshot: ul.SkillLevelSnapshot, // opcional; el repo igual lo calcula en List*
	}

	var already bool
	if uq, ok := s.queue.(queueUpserter); ok {
		already, err = uq.UpsertJoin(ctx, entry)
		if err != nil {
			return "", err
		}
	} else {
		// Fallback idempotente
		if a, _ := s.queue.Exists(ctx, guildID, discordID); !a {
			if err := s.queue.Join(ctx, entry); err != nil {
				return "", err
			}
			already = false
		} else {
			if err := s.queue.Join(ctx, entry); err != nil {
				return "", err
			}
			already = true
		}
	}

	// 3) Validaciones asíncronas (no bloquean UX)
	go s.validateJoinAsync(guildID, ul)

	// 4) Respuesta rápida
	if already {
		return fmt.Sprintf("🟡 Ya estabas en la cola, actualicé tu estado: **%s**.", ul.Nickname), nil
	}
	return fmt.Sprintf("✅ %s te uniste a la cola. (validando requisitos…)", ul.Nickname), nil
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

// ------------------ STATUS / HELPERS ------------------

func (s *QueueService) Status(ctx context.Context, guildID string) (string, error) {
	pol, _ := s.policy.Get(ctx, guildID)
	afkGrace := time.Duration(pol.AFKTimeoutSeconds) * time.Second
	leftGrace := time.Duration(pol.DropIfLeftSeconds) * time.Second // <- estaba en *time.Minute (bug)

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
			suf = " · 🚶"
		case "afk":
			suf = " · 😴 *(afk)*"
		}
		out += fmt.Sprintf("%d) <@%s> — **%s** (%s)%s\n", i+1, it.DiscordUserID, it.Nickname, it.Status, suf)
	}
	return out, nil
}

func chooseNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// --- validación asíncrona post-join ---
func (s *QueueService) validateJoinAsync(guildID string, ul storage.UserLink) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// policy
	pol, _ := s.policy.Get(ctx, guildID)
	cd := time.Duration(pol.CooldownAfterLossSeconds) * time.Second
	if cd <= 0 {
		cd = 2 * time.Minute
	}

	// 1) partide en curso en el hub
	if ok, err := s.fc.PlayerInOngoingHub(ctx, ul.FaceitUserID, s.hubID); err == nil && ok {
		_, leaveErr := s.queue.Leave(context.Background(), guildID, ul.DiscordUserID)
		if leaveErr != nil {
			fmt.Printf("⚠️ [validateJoinAsync] no pude sacar a %s de la cola: %v\n", ul.DiscordUserID, leaveErr)
		}
		s.notify(guildID, ul.DiscordUserID, "⛔ No puedes unirte: estás en una **partida activa del hub**.")
		return
	}

	// 2) cooldown por última derrota
	if lost, endedAt, err := s.fc.LastMatchLossWithin(ctx, ul.FaceitUserID, "cs2", cd); err == nil && lost {
		wait := time.Until(endedAt.Add(cd))
		if wait > 0 {
			s.notify(guildID, ul.DiscordUserID,
				fmt.Sprintf("⌛ Acabas de **perder** una partida. Debes esperar **%d s** para unirte.", int(wait.Seconds())))
			return
		}
	}

	// 3) membresía si la policy lo exige (y refresco de snapshots si están stale)
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
			s.notify(guildID, ul.DiscordUserID, "❌ Debes ser **miembro del Club** en FACEIT para unirte a la cola.")
			return
		}
	}
	// si llegó hasta acá, mantiene su lugar
}

func (s *QueueService) notify(guildID, userID, msg string) {
	if s.notifier != nil {
		s.notifier.Notify(guildID, userID, msg)
	}
}

// ------------------ Passthroughs ------------------

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
