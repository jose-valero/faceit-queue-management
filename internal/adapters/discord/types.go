package discord

import (
	"context"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

type Ctx struct {
	Log     *slog.Logger
	Session *discordgo.Session
	Event   *discordgo.InteractionCreate
	GuildID string
	UserID  string
	// Args de slash command (por nombre)
	Args map[string]string
	// Para components: valores parseados de custom_id (si aplican)
	Params map[string]string
}

type CommandHandler func(ctx context.Context, c *Ctx) error

type Command struct {
	Name        string
	Description string
	// Opcional: permisos/middleware
	AdminOnly bool
	Handler   CommandHandler
}

type ComponentHandler func(ctx context.Context, c *Ctx) error

// ComponentKey: usamos prefijos para enrutar (ej: "queue/join", "queue/leave")
type ComponentKey string
