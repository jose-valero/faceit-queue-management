package discord

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Router struct {
	s       *discordgo.Session
	guildID string

	link     func(ctx context.Context, nick, discordID, guildID string) (string, error)
	whoami   func(ctx context.Context, discordID string) (string, error)
	describe func(ctx context.Context, nick string) (string, error)
	unlink   func(ctx context.Context, discordID, guildID string) (string, error)
}

func NewRouter(
	s *discordgo.Session,
	guildID string,
	linkFn func(context.Context, string, string, string) (string, error),
	whoFn func(context.Context, string) (string, error),
	describeFn func(context.Context, string) (string, error),
	unlinkFn func(context.Context, string, string) (string, error),
) *Router {
	return &Router{
		s:        s,
		guildID:  guildID,
		link:     linkFn,
		whoami:   whoFn,
		describe: describeFn,
		unlink:   unlinkFn,
	}
}

func (r *Router) Register() error {
	appID := r.s.State.User.ID
	for _, cmd := range Commands {
		if _, err := r.s.ApplicationCommandCreate(appID, r.guildID, cmd); err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) Handlers() {
	r.s.AddHandler(func(s *discordgo.Session, ic *discordgo.InteractionCreate) {
		if ic.Type != discordgo.InteractionApplicationCommand {
			return
		}
		data := ic.ApplicationCommandData()
		log.Printf("slash: /%s by=%s guild=%s", data.Name, ic.Member.User.ID, ic.GuildID)

		// anti-panic → siempre respondemos
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic in slash /%s: %v", data.Name, rec)
				ReplyEphemeral(s, ic, "⚠️ Ocurrió un error inesperado.")
			}
		}()

		_ = DeferEphemeral(s, ic)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		switch data.Name {
		case "ping":
			ReplyEphemeral(s, ic, "🏓 pong")
			return

		case "fcplayer":
			nick := data.Options[0].StringValue()
			msg, err := r.describe(ctx, nick)
			if err != nil {
				msg = "⚠️ No pude obtener el jugador: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)
			return

		case "link":
			nick := data.Options[0].StringValue()
			msg, err := r.link(ctx, nick, ic.Member.User.ID, ic.GuildID)
			if err != nil {
				msg = "⚠️ No se pudo vincular: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)
			return

		case "whoami":
			msg, err := r.whoami(ctx, ic.Member.User.ID)
			if err != nil {
				msg = "No estás linkeado. Usa `/link nick:<tu_nick_FACEIT>`"
			}
			ReplyEphemeral(s, ic, msg)
			return
		case "unlink":
			msg, err := r.unlink(ctx, ic.Member.User.ID, ic.GuildID)
			if err != nil {
				msg = "⚠️ No se pudo desvincular: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)
			return
		}
	})
}
