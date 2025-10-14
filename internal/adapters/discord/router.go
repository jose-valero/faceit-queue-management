package discord

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/jose-valero/faceit-queue-bot/internal/app/service"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// VoiceCfg delimita dónde es “válido” estar en voz y cuál es AFK.
type VoiceCfg struct {
	AllowedCategoryID string
	AFKChannelID      string
}

type Router struct {
	s       *discordgo.Session
	guildID string

	voice        VoiceCfg
	link         *service.LinkService
	queue        *service.QueueService
	policy       *service.PolicyService
	uiStorage    *storage.UIRepo
	refreshMu    sync.Mutex
	refreshTimer *time.Timer
	adminRoleIDs []string
	rooms        *service.MatchRoomsService
	levelEmojis  map[int]string
	clickLimiter *userLimiter
}

func NewRouter(
	s *discordgo.Session,
	guildID string,
	voice VoiceCfg,
	link *service.LinkService,
	queue *service.QueueService,
	policy *service.PolicyService,
	ui *storage.UIRepo,
	adminRoleIDs []string,
	rooms *service.MatchRoomsService,
) *Router {
	return &Router{
		s:            s,
		guildID:      guildID,
		voice:        voice,
		link:         link,
		queue:        queue,
		policy:       policy,
		uiStorage:    ui,
		adminRoleIDs: adminRoleIDs,
		rooms:        rooms,
		clickLimiter: newUserLimiter(900 * time.Millisecond),
	}
}

func (r *Router) Register() error {
	appID := r.s.State.User.ID
	t0 := time.Now()
	_, err := r.s.ApplicationCommandBulkOverwrite(appID, r.guildID, Commands)
	if err != nil {
		return err
	}
	log.Printf("✅ comandos sincronizados (%d) in %s", len(Commands), time.Since(t0))
	r.initLevelBadges()
	return nil
}

func (r *Router) Handlers() {
	// Interactions
	r.s.AddHandler(func(s *discordgo.Session, ic *discordgo.InteractionCreate) {
		switch ic.Type {
		case discordgo.InteractionApplicationCommand:
			r.handleSlashCommand(s, ic) // ↙️ en commands_dispatch.go
		case discordgo.InteractionMessageComponent:
			r.handleMessageComponent(s, ic) // ↙️ en components_dispatch.go
		default:
			// ignoramos otros tipos por ahora
		}
	})

	// Voice events
	r.s.AddHandler(r.onVoiceStateUpdate) // ↙️ helper en voice_helpers.go

	// nuevo: se fue del servidor → marcar LEFT (no borrar)
	r.s.AddHandler(func(_ *discordgo.Session, e *discordgo.GuildMemberRemove) {
		if e.GuildID != r.guildID {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = r.queue.MarkLeft(ctx, e.GuildID, e.User.ID)
		go r.refreshQueueUI(e.GuildID)
	})

	// Refresher de cuenta regresiva (usa debounce interno)
	go r.runCountdownRefresher() // ↙️ en queue_ui.go
}
