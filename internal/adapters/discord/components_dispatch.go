package discord

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (r *Router) handleMessageComponent(s *discordgo.Session, ic *discordgo.InteractionCreate) {
	data := ic.MessageComponentData()

	_ = DeferEphemeral(s, ic)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	switch data.CustomID {

	case "queue_join":
		if !r.clickLimiter.Allow(ic.Member.User.ID) {
			ReplyEphemeral(s, ic, "⏳ Esperá un segundo…")
			return
		}
		if pol, err := r.policy.GetPolicy(ctx, ic.GuildID); err == nil && pol.VoiceRequired {
			ok, why := r.userInAllowedVoice(ic.GuildID, ic.Member.User.ID)
			if !ok {
				ReplyEphemeral(r.s, ic, "🎮 "+why)
				return
			}
		}
		msg, err := r.queue.Join(ctx, ic.GuildID, ic.Member.User.ID)
		if err != nil {
			msg = "⚠️ No se pudo unir a la cola: " + err.Error()
		}
		ReplyEphemeral(r.s, ic, msg)
		go r.refreshQueueUI(ic.GuildID)

	case "queue_leave":
		if !r.clickLimiter.Allow(ic.Member.User.ID) {
			ReplyEphemeral(s, ic, "⏳ Esperá un segundo…")
			return
		}
		msg, err := r.queue.Leave(ctx, ic.GuildID, ic.Member.User.ID)
		if err != nil {
			msg = "⚠️ No se pudo salir de la cola: " + err.Error()
		}
		ReplyEphemeral(r.s, ic, msg)
		go r.refreshQueueUI(ic.GuildID)

	case "admin_panel":
		if !r.clickLimiter.Allow(ic.Member.User.ID) {
			ReplyEphemeral(s, ic, "⏳ Esperá un segundo…")
			return
		}
		if !r.requireAdminOrRoles(s, ic) {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		items, err := r.queue.ListRich(ctx, ic.GuildID, 25)
		if err != nil {
			ReplyEphemeral(s, ic, "⚠️ No pude listar la cola: "+err.Error())
			return
		}
		if len(items) == 0 {
			ReplyEphemeral(s, ic, "ℹ️ La cola está vacía.")
			return
		}

		opts := make([]discordgo.SelectMenuOption, 0, len(items))
		for i, it := range items {
			label := fmt.Sprintf("%02d) %s", i+1, it.Nickname)

			if len(label) > 100 {
				label = label[:100]
			}

			desc := it.DiscordUserID + " · " + it.Status
			if len(desc) > 100 {
				desc = desc[:100]
			}
			opts = append(opts, discordgo.SelectMenuOption{
				Label:       label,
				Value:       "uid:" + it.DiscordUserID,
				Description: desc,
			})
		}
		row := discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "kick_select",
					Placeholder: "Selecciona a quién kickear",
					Options:     opts,
				},
			},
		}
		_, err = s.FollowupMessageCreate(ic.Interaction, true, &discordgo.WebhookParams{
			Content:    "Elige un jugador para **kickear**:",
			Components: []discordgo.MessageComponent{row},
		})
		if err != nil {
			ReplyEphemeral(s, ic, "⚠️ No pude mostrar el panel admin: "+err.Error())
		}
		return

		//--> solo admins
	case "kick_select":
		if !r.requireAdminOrRoles(s, ic) {
			return
		}
		if len(data.Values) == 0 {
			ReplyEphemeral(s, ic, "⚠️ Selección inválida.")
			return
		}
		uid := strings.TrimPrefix(data.Values[0], "uid:")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := r.queue.Leave(ctx, ic.GuildID, uid)
		if err != nil {
			ReplyEphemeral(s, ic, "⚠️ Error al kickear: "+err.Error())
			return
		}

		if strings.Contains(msg, "No estabas") {
			ReplyEphemeral(s, ic, "ℹ️ Ese jugador no estaba en la cola.")
		} else {
			ReplyEphemeral(s, ic, "✅ Jugador kickeado.")
		}
		go r.refreshQueueUI(ic.GuildID)
	}
}
