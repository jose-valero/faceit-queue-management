package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL  string
	DiscordToken string
	DiscordGuild string
	FaceitAPIKey string
	FaceitHubID  string

	WebhookSecret string
	HTTPAddr      string
}

func get(k string, required bool) string {
	v := os.Getenv(k)
	if required && v == "" {
		log.Fatalf("faltante env %s", k)
	}
	return v
}

func Load() Config {
	addr := get("HTTP_ADDR", false)
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		DatabaseURL:   get("DATABASE_URL", true),
		DiscordToken:  get("DISCORD_BOT_TOKEN", true),
		DiscordGuild:  get("DISCORD_GUILD_ID", true),
		FaceitAPIKey:  get("FACEIT_API_KEY", true),
		FaceitHubID:   get("FACEIT_HUB_ID", true),
		WebhookSecret: get("FACEIT_WEBHOOK_SECRET", true),
		HTTPAddr:      addr,
	}
}
