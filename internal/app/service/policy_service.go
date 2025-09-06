package service

import (
	"context"
	"fmt"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

type PolicyService struct {
	repo PolicyRepo
}

func NewPolicyService(r PolicyRepo) *PolicyService { return &PolicyService{repo: r} }

type PolicyPatch struct {
	RequireMember     *bool
	AFKTimeoutSeconds *int
	DropIfLeftMinutes *int
	VoiceRequired     *bool
}

func (s *PolicyService) Show(ctx context.Context, guildID string) (string, error) {
	p, err := s.repo.Get(ctx, guildID)

	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"**Policies de %s**\n• require_member: **%v**\n• afk_timeout_seconds: **%d**\n• drop_if_left_minutes: **%d**\n• voice_required: **%v**",
		guildID, p.RequireMember, p.AFKTimeoutSeconds, p.DropIfLeftMinutes, p.VoiceRequired,
	), nil
}

func (s *PolicyService) Update(ctx context.Context, guildID string, patch PolicyPatch) (string, error) {
	cur, err := s.repo.Get(ctx, guildID)
	if err != nil {
		return "", err
	}

	if patch.RequireMember != nil {
		cur.RequireMember = *patch.RequireMember
	}

	if patch.AFKTimeoutSeconds != nil {
		cur.AFKTimeoutSeconds = *patch.AFKTimeoutSeconds
	}

	if patch.DropIfLeftMinutes != nil {
		cur.DropIfLeftMinutes = *patch.DropIfLeftMinutes
	}

	if patch.VoiceRequired != nil {
		cur.VoiceRequired = *patch.VoiceRequired
	}

	if err := s.repo.Upsert(ctx, cur); err != nil {
		return "", err
	}
	return s.Show(ctx, guildID)
}

func (s *PolicyService) GetRaw(ctx context.Context, guildID string) (storage.GuildPolicy, error) {
	return s.repo.Get(ctx, guildID)
}
