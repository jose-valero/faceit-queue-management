package discord

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jose-valero/faceit-queue-bot/internal/app/service"
)

// VoiceCfg delimita dónde es “válido” estar en voz y cuál es AFK.
type VoiceCfg struct {
	AllowedCategoryID string
	AFKChannelID      string
}

type Router struct {
	s       *discordgo.Session
	guildID string

	voice  VoiceCfg
	link   *service.LinkService
	queue  *service.QueueService
	policy *service.PolicyService
}

func NewRouter(
	s *discordgo.Session,
	guildID string,
	voice VoiceCfg,
	link *service.LinkService,
	queue *service.QueueService,
	policy *service.PolicyService,
) *Router {
	return &Router{
		s:       s,
		guildID: guildID,
		voice:   voice,
		link:    link,
		queue:   queue,
		policy:  policy,
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
	// Slash commands
	r.s.AddHandler(func(s *discordgo.Session, ic *discordgo.InteractionCreate) {
		if ic.Type != discordgo.InteractionApplicationCommand {
			return
		}
		data := ic.ApplicationCommandData()
		log.Printf("slash: /%s by=%s guild=%s", data.Name, ic.Member.User.ID, ic.GuildID)

		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic in slash /%s: %v", data.Name, rec)
				ReplyEphemeral(s, ic, "⚠️ Ocurrió un error inesperado.")
			}
		}()

		_ = DeferEphemeral(s, ic)
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		switch data.Name {
		case "ping":
			ReplyEphemeral(s, ic, "🏓 pong")

		case "fcplayer":
			nick := data.Options[0].StringValue()
			msg, err := r.link.DescribeByNick(ctx, nick)
			if err != nil {
				msg = "⚠️ No pude obtener el jugador: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)

		case "link":
			nick := data.Options[0].StringValue()
			msg, err := r.link.Link(ctx, nick, ic.Member.User.ID, ic.GuildID)
			if err != nil {
				msg = "⚠️ No se pudo vincular: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)

		case "unlink":
			msg, err := r.link.Unlink(ctx, ic.Member.User.ID, ic.GuildID)
			if err != nil {
				msg = "⚠️ No se pudo desvincular: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)

		case "whoami":
			msg, err := r.link.WhoAmI(ctx, ic.Member.User.ID)
			if err != nil {
				msg = "No estás linkeado. Usa `/link nick:<tu_nick_FACEIT>`"
			}
			ReplyEphemeral(s, ic, msg)

		case "queue":
			if len(data.Options) == 0 {
				ReplyEphemeral(s, ic, "Usa `/queue join`, `/queue leave` o `/queue status`.")
				return
			}
			switch data.Options[0].Name {
			case "join":
				if pol, err := r.policy.GetRaw(ctx, ic.GuildID); err == nil && pol.VoiceRequired {
					ok, why := r.userInAllowedVoice(ic.GuildID, ic.Member.User.ID)
					if !ok {
						ReplyEphemeral(s, ic, "🎧 Debes estar en un **canal de voz de la categoría permitida** para unirte a la cola. "+why)
						return
					}
				}
				msg, err := r.queue.Join(ctx, ic.GuildID, ic.Member.User.ID)
				if err != nil {
					msg = "⚠️ No se pudo unir a la cola: " + err.Error()
				}
				ReplyEphemeral(s, ic, msg)
			case "leave":
				msg, err := r.queue.Leave(ctx, ic.GuildID, ic.Member.User.ID)
				if err != nil {
					msg = "⚠️ No se pudo salir de la cola: " + err.Error()
				}
				ReplyEphemeral(s, ic, msg)
			case "status":
				msg, err := r.queue.Status(ctx, ic.GuildID)
				if err != nil {
					msg = "⚠️ No se pudo consultar la cola: " + err.Error()
				}
				ReplyEphemeral(s, ic, msg)
			}

		case "policy":
			if len(data.Options) == 0 {
				ReplyEphemeral(s, ic, "Usa `/policy show` o `/policy set`.")
				return
			}
			switch data.Options[0].Name {
			case "show":
				msg, err := r.policy.Show(ctx, ic.GuildID)
				if err != nil {
					ReplyEphemeral(s, ic, "⚠️ No pude obtener la policy: "+err.Error())
					return
				}
				ReplyEphemeral(s, ic, msg)
			case "set":
				var patch service.PolicyPatch
				for _, opt := range data.Options[0].Options {
					switch opt.Name {
					case "require_member":
						v := opt.BoolValue()
						patch.RequireMember = &v
					case "voice_required":
						v := opt.BoolValue()
						patch.VoiceRequired = &v
					case "afk_timeout_seconds":
						v := int(opt.IntValue())
						patch.AFKTimeoutSeconds = &v
					case "drop_if_left_minutes":
						v := int(opt.IntValue())
						patch.DropIfLeftMinutes = &v
					}
				}
				msg, err := r.policy.Update(ctx, ic.GuildID, patch)
				if err != nil {
					ReplyEphemeral(s, ic, "⚠️ No pude actualizar: "+err.Error())
					return
				}
				ReplyEphemeral(s, ic, "✅ Policy actualizada.\n"+msg)
			}
		}
	})

	// VoiceStateUpdate → marca valid/afk/left (luego el pruner expulsa tras la “gracia”)
	r.s.AddHandler(func(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
		if vs.GuildID != r.guildID {
			return
		}
		uid := vs.UserID

		if vs.ChannelID == "" {
			_ = r.queue.MarkLeft(context.Background(), vs.GuildID, uid)
			return
		}
		if r.voice.AFKChannelID != "" && vs.ChannelID == r.voice.AFKChannelID {
			_, _ = r.queue.Leave(context.Background(), vs.GuildID, uid) // ignoramos el mensaje
			return
		}

		ch, err := r.safeGetChannel(vs.ChannelID)
		if err != nil {
			return
		}
		if r.voice.AllowedCategoryID != "" && ch.ParentID != r.voice.AllowedCategoryID {
			_ = r.queue.MarkLeft(context.Background(), vs.GuildID, uid)
			return
		}
		_ = r.queue.TouchValid(context.Background(), vs.GuildID, uid)
	})
}

// ---------- helpers ----------

func (r *Router) safeGetChannel(id string) (*discordgo.Channel, error) {
	if ch, err := r.s.State.Channel(id); err == nil && ch != nil {
		return ch, nil
	}
	ch, err := r.s.Channel(id)
	if err != nil {
		return nil, err
	}
	_ = r.s.State.ChannelAdd(ch) // ChannelAdd devuelve solo error
	return ch, nil
}

func (r *Router) userInAllowedVoice(guildID, userID string) (bool, string) {
	vs, err := r.s.State.VoiceState(guildID, userID)
	if err != nil || vs == nil || vs.ChannelID == "" {
		return false, "No estás en un canal de voz."
	}

	// AFK explícito
	if r.voice.AFKChannelID != "" && vs.ChannelID == r.voice.AFKChannelID {
		return false, "Estás en **AFK**."
	}

	// Verificamos la categoría del canal de voz
	ch, err := r.safeGetChannel(vs.ChannelID)
	if err != nil || ch == nil {
		return false, "No pude leer tu canal de voz."
	}
	if r.voice.AllowedCategoryID != "" && ch.ParentID != r.voice.AllowedCategoryID {
		return false, "Estás en una categoría de voz **no permitida**."
	}

	return true, ""
}
