package service

import (
	"context"
	"time"

	"github.com/jose-valero/faceit-queue-bot/internal/domain"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// Implementado por internal/adapters/faceit.Client
type FaceitAPI interface {
	GetPlayerByNickname(ctx context.Context, nick, game string) (*domain.Player, error)
	IsMemberOfHub(ctx context.Context, playerID, hubID string) (bool, error)

	PlayerInOngoingHub(ctx context.Context, playerID, hubID string) (bool, error)
	LastMatchLossWithin(ctx context.Context, playerID, game string, within time.Duration) (bool, time.Time, error)
}

// Implementado por internal/infra/storage.UserRepo
type UserRepo interface {
	GetByDiscordID(ctx context.Context, discordID string) (storage.UserLink, error)
	UpsertLink(ctx context.Context, ul storage.UserLink) error
	SoftDeleteByDiscordID(ctx context.Context, discordID, guildID string) (bool, error)
	FindDiscordByFaceitIDs(ctx context.Context, ids []string) (map[string]string, error)
}

// Implementado por internal/infra/storage.QueueRepo
type QueueRepo interface {
	Join(ctx context.Context, e storage.QueueEntry) error
	Leave(ctx context.Context, guildID, discordID string) (bool, error)
	List(ctx context.Context, guildID string, limit int) ([]storage.QueueEntry, error)

	TouchValid(ctx context.Context, guildID, discordID string) error
	MarkLeft(ctx context.Context, guildID, discordID string) error
	MarkAFK(ctx context.Context, guildID, discordID string) error

	Exists(ctx context.Context, guildID, discordID string) (bool, error)

	// Prune con “tiempos de gracia” para AFK/LEFT
	Prune(ctx context.Context, guildID string, afkTimeout, leftTimeout time.Duration) (int64, int64, error)
	// LEFT/AFK con tiempos de gracia
	ListWithGrace(ctx context.Context, guildID string, limit int, graceAFK, graceLeft time.Duration) ([]storage.QueueEntry, error)
}

// Implementado por internal/infra/storage.PolicyRepo
type PolicyRepo interface {
	Get(ctx context.Context, guildID string) (storage.GuildPolicy, error)
	Upsert(ctx context.Context, p storage.GuildPolicy) error
}
