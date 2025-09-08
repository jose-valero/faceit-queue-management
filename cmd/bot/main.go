// cmd/xcg-bot/main.go (fragmento completo y ordenado)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	discordrouter "github.com/jose-valero/faceit-queue-bot/internal/adapters/discord"
	"github.com/jose-valero/faceit-queue-bot/internal/adapters/faceit"
	httpfaceit "github.com/jose-valero/faceit-queue-bot/internal/adapters/httpfaceit"
	"github.com/jose-valero/faceit-queue-bot/internal/app/service"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/config"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	_ = godotenv.Load()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg := config.Load()

	// DB
	db, err := storage.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		log.Fatal("migrate:", err)
	}
	log.Println("✅ DB lista y migrada")

	// Repos
	usersRepo := storage.NewUserRepo(db)
	queueRepo := storage.NewQueueRepo(db)
	policyRepo := storage.NewPolicyRepo(db)
	uiRepo := storage.NewUIRepo(db)
	roomsRepo := storage.NewMatchRoomsRepo(db)

	// FACEIT client (antes de services que lo usan)
	fc := faceit.New(cfg.FaceitAPIKey)

	// Discord session (antes del roomsSvc, que la necesita)
	auth := cfg.DiscordToken
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(auth)), "bot ") {
		auth = "Bot " + strings.TrimSpace(auth)
	}
	s, err := discordgo.New(auth)
	if err != nil {
		log.Fatal(err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates
	if err := s.Open(); err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	log.Printf("✅ Conectado como %s (%s)", s.State.User.Username, s.State.User.ID)

	// Services
	linkSvc := service.NewLinkService(fc, usersRepo, cfg.FaceitHubID)
	queueSvc := service.NewQueueService(fc, usersRepo, queueRepo, policyRepo, cfg.FaceitHubID)
	policySvc := service.NewPolicyService(policyRepo)

	// Rooms service (ya tenemos s y fc)
	roomsSvc := service.NewMatchRoomsService(s, fc, usersRepo, roomsRepo, cfg.DiscordGuild, "XCG Faceit Match")

	// Webhook FACEIT (callback opcional)
	web := httpfaceit.New(cfg.WebhookSecret, usersRepo, func(ctx context.Context, matchID, status string) {
		roomsSvc.HandleMatchEvent(ctx, matchID, status)
	})
	go web.Start(cfg.HTTPAddr)

	// Router
	r := discordrouter.NewRouter(
		s,
		cfg.DiscordGuild,
		discordrouter.VoiceCfg{
			AllowedCategoryID: cfg.VoiceCategoryID,
			AFKChannelID:      cfg.AFKChannelID,
		},
		linkSvc,
		queueSvc,
		policySvc,
		uiRepo,
		cfg.AdminRoleIDs,
		roomsSvc,
	)
	if err := r.Register(); err != nil {
		log.Fatalf("registrando comandos: %v", err)
	}
	r.Handlers()
	log.Printf("✅ comandos registrados en guild %s", cfg.DiscordGuild)

	// Pruner (gracias AFK/LEFT) — ya usás segundos
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			pol, err := policyRepo.Get(ctx, cfg.DiscordGuild)
			cancel()
			if err != nil {
				continue
			}

			afk := time.Duration(pol.AFKTimeoutSeconds) * time.Second
			left := time.Duration(pol.DropIfLeftSeconds) * time.Second
			if afk <= 0 && left <= 0 {
				continue
			}

			_, _, _ = queueSvc.Prune(context.Background(), cfg.DiscordGuild, afk, left)
			// opcional: r.RefreshIfVisible(cfg.DiscordGuild)
		}
	}()

	// Esperar señal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop
}
