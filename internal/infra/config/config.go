package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL     string
	DiscordToken    string
	DiscordGuild    string
	FaceitAPIKey    string
	FaceitHubID     string
	WebhookSecret   string
	HTTPAddr        string
	VoiceCategoryID string
	AFKChannelID    string
	AdminRoleIDs    []string `env:"ADMIN_ROLE_IDS"`
}

func Load() Config {
	get := func(k string, req bool) string {
		v := os.Getenv(k)
		if v == "" && req {
			log.Fatalf("faltante env %s", k)
		}
		return v
	}

	cfg := Config{
		DatabaseURL:   get("DATABASE_URL", true),
		DiscordToken:  get("DISCORD_BOT_TOKEN", true),
		DiscordGuild:  get("DISCORD_GUILD_ID", true),
		FaceitAPIKey:  get("FACEIT_API_KEY", true),
		FaceitHubID:   get("FACEIT_HUB_ID", true),
		WebhookSecret: get("FACEIT_WEBHOOK_SECRET", true),
		HTTPAddr:      get("HTTP_ADDR", false), // puede quedar vacío
		// nuevos
		VoiceCategoryID: get("VOICE_CATEGORY_ID", false),
		AFKChannelID:    get("AFK_CHANNEL_ID", false),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	if s := strings.TrimSpace(os.Getenv("ADMIN_ROLE_IDS")); s != "" {
		parts := strings.Split(s, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		cfg.AdminRoleIDs = parts
	}
	return cfg
}
