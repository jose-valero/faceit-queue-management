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
	RequireMember            *bool
	AFKTimeoutSeconds        *int
	DropIfLeftSeconds        *int
	VoiceRequired            *bool
	CooldownAfterLossSeconds *int
}

func (s *PolicyService) GetPolicy(ctx context.Context, guildID string) (storage.GuildPolicy, error) {
	return s.repo.Get(ctx, guildID)
}

func (s *PolicyService) Show(ctx context.Context, guildID string) (string, error) {
	p, err := s.repo.Get(ctx, guildID)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"**Policies de %s**\n• require_member: **%v**\n• voice_required: **%v**\n• afk_timeout_seconds: **%d**\n• drop_if_left_minutes: **%d**\n• cooldown_after_loss_seconds: **%d**",
		guildID, p.RequireMember, p.VoiceRequired, p.AFKTimeoutSeconds, p.DropIfLeftSeconds, p.CooldownAfterLossSeconds,
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
	if patch.DropIfLeftSeconds != nil {
		cur.DropIfLeftSeconds = *patch.DropIfLeftSeconds
	}
	if patch.VoiceRequired != nil {
		cur.VoiceRequired = *patch.VoiceRequired
	}
	if patch.CooldownAfterLossSeconds != nil {
		cur.CooldownAfterLossSeconds = *patch.CooldownAfterLossSeconds
	}

	if err := s.repo.Upsert(ctx, cur); err != nil {
		return "", err
	}
	return s.Show(ctx, guildID)
}
