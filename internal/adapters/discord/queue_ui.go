package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// atajos de tunning (para los timers y ajustar aqui)
const (
	uiDebounce   = 80 * time.Millisecond
	ctxRenderMax = 900 * time.Millisecond
	ctxPruneMax  = 600 * time.Millisecond
)

// Publica o reposta la UI de la cola en ESTE canal
func (r *Router) publishQueueUI(ctx context.Context, guildID, channelID string) error {
	embed, comps, err := r.renderQueueEmbed(ctx, guildID)
	if err != nil {
		return err
	}
	msg, err := r.s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{comps}, // slice
	})
	if err != nil {
		return err
	}
	return r.uiStorage.Upsert(ctx, guildID, channelID, msg.ID)
}

// Debounce + re-render + edit de la UI
func (r *Router) refreshQueueUI(guildID string) {
	r.refreshMu.Lock()
	if r.refreshTimer != nil {
		r.refreshTimer.Stop()
	}
	r.refreshTimer = time.AfterFunc(uiDebounce, func() {
		start := time.Now()

		// 1) Render & Edit PRIMERO (rápido)
		ctxR, cancelR := context.WithTimeout(context.Background(), ctxRenderMax)
		tGet := time.Now()
		ui, err := r.uiStorage.Get(ctxR, guildID)
		log.Printf("[ui.refresh] getUI dur=%s err=%v", time.Since(tGet), err)
		if err == nil && ui.QueueChannelID != "" && ui.QueueMessageID != "" {
			tR := time.Now()
			embed, comps, rErr := r.renderQueueEmbed(ctxR, guildID)
			log.Printf("[ui.refresh] render dur=%s err=%v", time.Since(tR), err)
			if rErr == nil {
				em := []*discordgo.MessageEmbed{embed}
				cc := []discordgo.MessageComponent{comps}
				tE := time.Now()
				_, editErr := r.s.ChannelMessageEditComplex(&discordgo.MessageEdit{
					Channel:    ui.QueueChannelID,
					ID:         ui.QueueMessageID,
					Embeds:     &em,
					Components: &cc,
				})
				log.Printf("[ui.refresh] edit dur=%s err=%v", time.Since(tE), editErr)
				dur := time.Since(tE)
				if editErr != nil {
					var re *discordgo.RESTError
					if errors.As(editErr, &re) && re.Response != nil {
						ra := re.Response.Header.Get("Retry-After")
						rem := re.Response.Header.Get("X-RateLimit-Remaining")
						bkt := re.Response.Header.Get("X-RateLimit-Bucket")
						log.Printf("[ui.edit] status=%d retryAfter=%s remaining=%s bucket=%s dur=%s body=%s",
							re.Response.StatusCode, ra, rem, bkt, dur, string(re.ResponseBody))
					} else {
						log.Printf("[ui.edit] err=%v dur=%s", err, dur)
					}
				} else {
					log.Printf("[ui.edit] ok dur=%s", dur)
				}
			}
		}
		cancelR()

		// 2) PRUNE en paralelo (no bloquea el repaint)
		go func() {
			ctxP, cancelP := context.WithTimeout(context.Background(), ctxPruneMax)
			defer cancelP()
			pol, _ := r.policy.GetPolicy(ctxP, guildID)
			afk := time.Duration(pol.AFKTimeoutSeconds) * time.Second
			left := time.Duration(pol.DropIfLeftSeconds) * time.Second
			_, _, _ = r.queue.Prune(ctxP, guildID, afk, left)
		}()

		// tracing opcional
		log.Printf("[ui.refresh] total=%s", time.Since(start))
	})
	r.refreshMu.Unlock()

}

// Render del embed + botones, con countdowns
func (r *Router) renderQueueEmbed(ctx context.Context, guildID string) (*discordgo.MessageEmbed, discordgo.MessageComponent, error) {
	tPol := time.Now()
	const groupSize = 5
	const softGap = "\n\u200B\n" // separador vertical suave entre grupos
	pol, _ := r.policy.GetPolicy(ctx, guildID)
	log.Printf("[ui.render] policy dur=%s", time.Since(tPol))
	graceAFK := time.Duration(pol.AFKTimeoutSeconds) * time.Second
	graceLeft := time.Duration(pol.DropIfLeftSeconds) * time.Second

	tList := time.Now()
	items, err := r.queue.ListRichWithGrace(ctx, guildID, 50, graceAFK, graceLeft)
	log.Printf("[ui.render] list dur=%s err=%v n=%d", time.Since(tList), err, len(items))
	if err != nil {
		return nil, nil, err
	}

	// Garantiza orden estable por joined_at
	sort.SliceStable(items, func(i, j int) bool { return items[i].JoinedAt.Before(items[j].JoinedAt) })

	lines := "Nadie en cola."
	var nextRefresh time.Duration = 0

	if len(items) > 0 {
		var b strings.Builder

		for start := 0; start < len(items); start += groupSize {
			end := min(start+groupSize, len(items))
			group := items[start:end]
			groupN := (start / groupSize) + 1

			// Título de grupo
			fmt.Fprintf(&b, "**Fila #%d**  _( %d–%d )_\n", groupN, start+1, end)

			// Filas del grupo
			for i := range group {
				globalIdx := start + i
				it := group[i]

				// badge + nick + mention + status
				lvl := 0
				if it.SkillLevelSnapshot != nil {
					lvl = *it.SkillLevelSnapshot
				}
				badge := r.levelBadge(lvl)
				nick := fmt.Sprintf("[%s](%s)", it.Nickname, faceitPlayerURL(it.Nickname))
				mention := "<@" + it.DiscordUserID + ">"

				suf, nref := r.statusSuffix(it, graceAFK, graceLeft)
				if nref > 0 && (nextRefresh == 0 || nref < nextRefresh) {
					nextRefresh = nref
				}

				const sep = "\u2007"
				if badge != "" {
					// Ej: "1) <:faceitlvl6:...> **[Nick](url)** — @user (waiting)"
					fmt.Fprintf(&b, "%d. %s **%s%s** — %s%s\n", globalIdx+1, badge, sep, nick, mention, suf)
				} else {
					fmt.Fprintf(&b, "%d. **%s** — %s%s\n", globalIdx+1, nick, mention, suf)
				}
			}

			// Separación visual entre grupos
			if end < len(items) {
				b.WriteString(softGap)
			}
		}

		lines = b.String()
	}

	// Agenda refresh justo después del primer vencimiento
	if nextRefresh > 0 {
		go r.scheduleRefresh(guildID, nextRefresh+200*time.Millisecond) // colchón
	}

	embed := &discordgo.MessageEmbed{
		Title:       "XCG — Cola",
		Description: lines,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	comps := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Style:    discordgo.PrimaryButton,
				Label:    "La llevo",
				CustomID: "queue_join",
				Emoji:    &discordgo.ComponentEmoji{Name: "🌕"},
			},
			discordgo.Button{
				Style:    discordgo.SecondaryButton,
				Label:    "Chau",
				CustomID: "queue_leave",
				Emoji:    &discordgo.ComponentEmoji{Name: "👋"},
			},
			discordgo.Button{
				Style:    discordgo.SecondaryButton,
				Label:    "Admin",
				CustomID: "admin_panel",
				Emoji:    &discordgo.ComponentEmoji{Name: "👮"},
			},
		},
	}

	return embed, comps, nil
}

func (r *Router) scheduleRefresh(guildID string, d time.Duration) {
	time.AfterFunc(d, func() { r.refreshQueueUI(guildID) })
}

// Ticker 1s para actualizar countdowns sin flood (usa tu debounce)
func (r *Router) runCountdownRefresher() {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	var lastRecon time.Time

	for range t.C {
		// ¿Tenemos UI publicada para este guild?
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		ui, err := r.uiStorage.Get(ctx, r.guildID)
		cancel()
		if err != nil || ui.QueueChannelID == "" || ui.QueueMessageID == "" {
			continue
		}

		if time.Since(lastRecon) >= 5*time.Second {
			if changed := r.reconcileWaitingStatuses(r.guildID); changed {
				// si hubo cambios de estado, re-render (usa debounce)
				r.refreshQueueUI(r.guildID)
			}
			lastRecon = time.Now()
		}

		// leer policy
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		pol, err := r.policy.GetPolicy(ctx2, r.guildID)
		cancel2()
		if err != nil {
			continue
		}
		graceAFK := time.Duration(pol.AFKTimeoutSeconds) * time.Second
		graceLeft := time.Duration(pol.DropIfLeftSeconds) * time.Second
		if graceAFK <= 0 && graceLeft <= 0 {
			continue
		}

		// ¿Hay algún countdown activo?
		ctx3, cancel3 := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		items, err := r.queue.List(ctx3, r.guildID, 50)
		cancel3()
		if err != nil {
			continue
		}
		now := time.Now()
		hasCountdown := false
		for _, it := range items {
			if it.Status == "left" && graceLeft > 0 && now.Before(it.LastSeenAt.Add(graceLeft)) {
				hasCountdown = true
				break
			}
			if it.Status == "afk" && graceAFK > 0 && now.Before(it.LastSeenAt.Add(graceAFK)) {
				hasCountdown = true
				break
			}
		}
		if hasCountdown {
			r.refreshQueueUI(r.guildID) // usa el debounce (120ms)
		}
	}
}

// Recorre los 'waiting' y ajusta estado a 'afk' o 'left' según voz/policy.
// Devuelve true si cambió algo.
func (r *Router) reconcileWaitingStatuses(guildID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// List() ya trae solo 'waiting'
	items, err := r.queue.List(ctx, guildID, 200)
	if err != nil || len(items) == 0 {
		return false
	}

	// policy una sola vez
	pol, _ := r.policy.GetPolicy(ctx, guildID)
	changed := false

	for _, it := range items {
		vs, _ := r.s.State.VoiceState(guildID, it.DiscordUserID)

		// 1) No está en voz
		if vs == nil || vs.ChannelID == "" {
			// si requerís voz: se marca LEFT (queda visible por gracia LEFT, luego prune)
			if pol.VoiceRequired {
				if err := r.queue.MarkLeft(ctx, guildID, it.DiscordUserID); err == nil {
					changed = true
				}
			}
			// si NO requerís voz, lo dejamos en waiting (no tocar)
			continue
		}

		// 2) Está en AFK
		if r.voice.AFKChannelID != "" && vs.ChannelID == r.voice.AFKChannelID {
			if err := r.queue.MarkAFK(ctx, guildID, it.DiscordUserID); err == nil {
				changed = true
			}
			continue
		}

		// 3) Está en voz, ¿pero en categoría permitida?
		if r.voice.AllowedCategoryID != "" {
			ch, err := r.safeGetChannel(vs.ChannelID)
			if err != nil {
				// si no pudimos leer el canal, no toques estado
				continue
			}
			if ch.ParentID != r.voice.AllowedCategoryID {
				// si requerís voz válida: LEFT
				if pol.VoiceRequired {
					if err := r.queue.MarkLeft(ctx, guildID, it.DiscordUserID); err == nil {
						changed = true
					}
				}
				continue
			}
		}

		// 4) Voz válida → refrescá 'waiting' (last_seen_at)
		_ = r.queue.TouchValid(ctx, guildID, it.DiscordUserID)
	}

	return changed
}
