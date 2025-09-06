package service

import (
	"context"

	"github.com/jose-valero/faceit-queue-bot/internal/domain"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// Lo implementa internal/adapters/faceit.Client
type FaceitAPI interface {
	GetPlayerByNickname(ctx context.Context, nick, game string) (*domain.Player, error)
	IsMemberOfHub(ctx context.Context, playerID, hubID string) (bool, error)
}

// Lo implementa internal/infra/storage.UserRepo
type UserRepo interface {
	GetByDiscordID(ctx context.Context, discordID string) (storage.UserLink, error)
	UpsertLink(ctx context.Context, ul storage.UserLink) error
	SoftDeleteByDiscordID(ctx context.Context, discordID, guildID string) (bool, error)
}
