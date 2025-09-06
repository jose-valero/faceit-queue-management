package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
	users := storage.NewUserRepo(db)

	// HTTP webhook (puede quedar prendido; si FACEIT no envía, no pasa nada)
	web := httpfaceit.New(cfg.WebhookSecret, users)
	go web.Start(cfg.HTTPAddr)

	// FACEIT client
	fc := faceit.New(cfg.FaceitAPIKey)

	// Servicio de negocio
	svc := service.NewLinkService(fc, users, cfg.FaceitHubID)

	// Discord
	auth := cfg.DiscordToken
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(auth)), "bot ") {
		auth = "Bot " + strings.TrimSpace(auth)
	}
	s, err := discordgo.New(auth)
	if err != nil {
		log.Fatal(err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds
	if err := s.Open(); err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	log.Printf("✅ Conectado como %s (%s)", s.State.User.Username, s.State.User.ID)

	// Router (métodos del servicio)
	r := discordrouter.NewRouter(
		s,
		cfg.DiscordGuild,
		svc.Link,
		svc.WhoAmI,
		svc.DescribeByNick,
		svc.Unlink,
	)
	if err := r.Register(); err != nil {
		log.Fatalf("registrando comandos: %v", err)
	}
	r.Handlers()
	log.Printf("🔧 comandos registrados en guild %s", cfg.DiscordGuild)

	// Esperar señal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop
}
