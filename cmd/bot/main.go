// cmd/xcg-bot/main.go (fragmento completo y ordenado)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
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

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func startWebhookListener(ctx context.Context, dsn string, onEvent func(id int64, typ string, payload string)) error {
	go func() {
		for {
			conn, err := pgx.Connect(ctx, dsn)
			if err != nil {
				log.Printf("listen connect: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}
			defer conn.Close(ctx)

			if _, err := conn.Exec(ctx, "LISTEN faceit_webhook"); err != nil {
				log.Printf("listen exec: %v", err)
				_ = conn.Close(ctx)
				time.Sleep(2 * time.Second)
				continue
			}
			log.Println("👂 listening on channel faceit_webhook")

			for {
				n, err := conn.WaitForNotification(ctx)
				if err != nil {
					log.Printf("listen wait: %v", err)
					_ = conn.Close(ctx)
					break // reconectar
				}
				id, _ := strconv.ParseInt(n.Payload, 10, 64)
				var typ, payload string
				// leemos el evento
				if err := conn.QueryRow(ctx,
					"SELECT type, payload::text FROM webhook_events WHERE id=$1", id,
				).Scan(&typ, &payload); err != nil {
					log.Printf("listen fetch %d: %v", id, err)
					continue
				}
				onEvent(id, typ, payload)
			}
		}
	}()
	return nil
}

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

	// justo después de "✅ DB lista y migrada"
	_ = startWebhookListener(context.Background(), cfg.DatabaseURL, func(id int64, typ, payload string) {
		log.Printf("[WEBHOOK] id=%d type=%s payload=%s", id, typ, payload)

		// (opcional) si querés disparar lógica:
		switch typ {
		case "match_object_created", "match_status_configuring", "match_status_ready", "match_demo_ready", "match_status_finished", "match_status_cancelled", "match_status_aborted":
			// ejemplo muy simple: podrías parsear payload y llamar roomsSvc.HandleMatchEvent
			// aquí lo dejamos en log para validar visualmente
		case "hub_user_role_added", "hub_user_role_removed":
			// por ahora solo log
		}
	})

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
