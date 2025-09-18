package discord

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

func (r *Router) safeGetChannel(id string) (*discordgo.Channel, error) {
	if ch, err := r.s.State.Channel(id); err == nil && ch != nil {
		return ch, nil
	}
	ch, err := r.s.Channel(id)
	if err != nil {
		return nil, err
	}
	_ = r.s.State.ChannelAdd(ch)
	return ch, nil
}

func (r *Router) userInAllowedVoice(guildID, userID string) (bool, string) {
	vs, err := r.s.State.VoiceState(guildID, userID)
	if err != nil || vs == nil {
		return false, "No estás en voz."
	}
	if r.voice.AFKChannelID != "" && vs.ChannelID == r.voice.AFKChannelID {
		return false, "Estás en **AFK**."
	}
	ch, err := r.safeGetChannel(vs.ChannelID)
	if err != nil {
		return false, "No pude leer tu canal de voz."
	}
	if r.voice.AllowedCategoryID != "" && ch.ParentID != r.voice.AllowedCategoryID {
		return false, "Estás en una categoría de voz **no permitida**."
	}
	return true, ""
}

func (r *Router) onVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	if vs.GuildID != r.guildID {
		return
	}
	uid := vs.UserID

	// Sin canal -> left
	if vs.ChannelID == "" {
		_ = r.queue.MarkLeft(context.Background(), vs.GuildID, uid)
		go r.refreshQueueUI(vs.GuildID)
		return
	}
	// AFK explícito
	if r.voice.AFKChannelID != "" && vs.ChannelID == r.voice.AFKChannelID {
		_ = r.queue.MarkAFK(context.Background(), vs.GuildID, uid)
		go r.refreshQueueUI(vs.GuildID)
		return
	}

	// ¿Está en una categoría permitida?
	ch, err := r.safeGetChannel(vs.ChannelID)
	if err != nil {
		return
	}
	if r.voice.AllowedCategoryID != "" && ch.ParentID != r.voice.AllowedCategoryID {
		_ = r.queue.MarkLeft(context.Background(), vs.GuildID, uid)
		go r.refreshQueueUI(vs.GuildID)
		return
	}
	// OK válido → refresca last_seen
	_ = r.queue.TouchValid(context.Background(), vs.GuildID, uid)
	go r.refreshQueueUI(vs.GuildID)
}
