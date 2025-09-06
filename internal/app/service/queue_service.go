package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

type QueueService struct {
	users  UserRepo
	queue  QueueRepo
	policy PolicyRepo
}

func NewQueueService(users UserRepo, queue QueueRepo, policy PolicyRepo) *QueueService {
	return &QueueService{users: users, queue: queue, policy: policy}
}

func (s *QueueService) Join(ctx context.Context, guildID, discordID string) (string, error) {
	// valida vinculacion
	ul, err := s.users.GetByDiscordID(ctx, discordID)
	if err != nil {
		return "❌ No estás vinculado. Usa `/link nick:<tu_nick_FACEIT>`", nil
	}

	// valida policy
	pol, _ := s.policy.Get(ctx, guildID)
	if pol.RequireMember && !ul.IsMember {
		return "❌ Debes ser **miembro del Club** en FACEIT para unirte a la cola.", nil
	}

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
	leftGrace := time.Duration(pol.DropIfLeftMinutes) * time.Minute

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
