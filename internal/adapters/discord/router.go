package discord

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/jose-valero/faceit-queue-bot/internal/app/service"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// VoiceCfg delimita dónde es “válido” estar en voz y cuál es AFK.
type VoiceCfg struct {
	AllowedCategoryID string
	AFKChannelID      string
}

type Router struct {
	s       *discordgo.Session
	guildID string

	voice        VoiceCfg
	link         *service.LinkService
	queue        *service.QueueService
	policy       *service.PolicyService
	uiStorage    *storage.UIRepo
	refreshMu    sync.Mutex
	refreshTimer *time.Timer
	adminRoleIDs []string
	rooms        *service.MatchRoomsService
}

func NewRouter(
	s *discordgo.Session,
	guildID string,
	voice VoiceCfg,
	link *service.LinkService,
	queue *service.QueueService,
	policy *service.PolicyService,
	ui *storage.UIRepo,
	adminRoleIDs []string,
	rooms *service.MatchRoomsService,
) *Router {
	return &Router{
		s:            s,
		guildID:      guildID,
		voice:        voice,
		link:         link,
		queue:        queue,
		policy:       policy,
		uiStorage:    ui,
		adminRoleIDs: adminRoleIDs,
		rooms:        rooms,
	}
}

func (r *Router) Register() error {
	appID := r.s.State.User.ID

	t0 := time.Now()
	// Reemplaza todos los comandos de ese guild por los que declares en Commands
	_, err := r.s.ApplicationCommandBulkOverwrite(appID, r.guildID, Commands)
	if err != nil {
		return err
	}
	log.Printf("✅ comandos sincronizados (%d) in %s", len(Commands), time.Since(t0))
	return nil
}

var reMention = regexp.MustCompile(`<@!?(\d+)>`)

func parseIDs(raw string) []string {
	ids := []string{}
	for _, tok := range strings.Fields(raw) {
		if m := reMention.FindStringSubmatch(tok); len(m) == 2 {
			ids = append(ids, m[1])
			continue
		}
		// si son solo dígitos, lo tomamos como ID
		allDigits := true
		for _, r := range tok {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			ids = append(ids, tok)
		}
	}
	return ids
}
func (r *Router) Handlers() {
	// Slash commands
	r.s.AddHandler(func(s *discordgo.Session, ic *discordgo.InteractionCreate) {
		switch ic.Type {

		case discordgo.InteractionApplicationCommand:
			cmd := ic.ApplicationCommandData()
			log.Printf("cmd: %s by=%s guild=%s", cmd.Name, ic.Member.User.ID, ic.GuildID)

			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("panic in cmd /%s: %v", cmd.Name, rec)
					ReplyEphemeral(s, ic, "⚠️ Ocurrió un error inesperado.")
				}
			}()

			_ = DeferEphemeral(s, ic)
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()

			if cmd.TargetID != "" && cmd.Name == "FACEIT: Ver perfil" {
				_ = DeferEphemeral(s, ic)

				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()

				targetID := cmd.TargetID
				if targetID == "" && cmd.Resolved != nil && len(cmd.Resolved.Users) == 1 {
					for id := range cmd.Resolved.Users {
						targetID = id
						break
					}
				}

				msg, err := r.link.WhoAmI(ctx, targetID)
				if err != nil {
					ReplyEphemeral(s, ic, "Ese usuario no está vinculado. Que use `/link`.")
					return
				}
				ReplyEphemeral(s, ic, msg)
				return
			}

			switch cmd.Name {
			case "ping":
				ReplyEphemeral(s, ic, "🏓 pong")

			case "fcplayer":
				nick := cmd.Options[0].StringValue()
				msg, err := r.link.DescribeByNick(ctx, nick)
				if err != nil {
					msg = "⚠️ No pude obtener el jugador: " + err.Error()
				}
				ReplyEphemeral(s, ic, msg)

			case "link":
				nick := cmd.Options[0].StringValue()
				msg, err := r.link.Link(ctx, nick, ic.Member.User.ID, ic.GuildID)
				if err != nil {
					msg = "⚠️ No se pudo vincular: " + err.Error()
				}
				ReplyEphemeral(s, ic, msg)
				go r.refreshQueueUI(ic.GuildID) // actualiza UI si existe

			case "unlink":
				msg, err := r.link.Unlink(ctx, ic.Member.User.ID, ic.GuildID)
				if err != nil {
					msg = "⚠️ No se pudo desvincular: " + err.Error()
				}
				ReplyEphemeral(s, ic, msg)
				go r.refreshQueueUI(ic.GuildID)

			case "whoami":
				msg, err := r.link.WhoAmI(ctx, ic.Member.User.ID)
				if err != nil {
					msg = "No estás linkeado. Usa `/link nick:<tu_nick_FACEIT>`"
				}
				ReplyEphemeral(s, ic, msg)

			case "queue":
				if !r.requireAdminOrRoles(s, ic) {
					return
				}
				if len(cmd.Options) == 0 {
					ReplyEphemeral(s, ic, "Usa `/queue join`, `/queue leave` o `/queue status`.")
					return
				}
				sub := cmd.Options[0].Name
				switch sub {
				case "join":
					// Validación de voz (si la policy lo requiere)
					if pol, err := r.policy.GetPolicy(ctx, ic.GuildID); err == nil && pol.VoiceRequired {
						ok, why := r.userInAllowedVoice(ic.GuildID, ic.Member.User.ID)
						if !ok {
							ReplyEphemeral(s, ic, "🎮 Debes estar en un **canal de voz permitido** para unirte. "+why)
							return
						}
					}
					msg, err := r.queue.Join(ctx, ic.GuildID, ic.Member.User.ID)
					if err != nil {
						msg = "⚠️ No se pudo unir a la cola: " + err.Error()
					}
					ReplyEphemeral(s, ic, msg)
					go r.refreshQueueUI(ic.GuildID)

				case "leave":
					msg, err := r.queue.Leave(ctx, ic.GuildID, ic.Member.User.ID)
					if err != nil {
						msg = "⚠️ No se pudo salir de la cola: " + err.Error()
					}
					ReplyEphemeral(s, ic, msg)
					go r.refreshQueueUI(ic.GuildID)

				case "status":
					msg, err := r.queue.Status(ctx, ic.GuildID)
					if err != nil {
						msg = "⚠️ No se pudo consultar la cola: " + err.Error()
					}
					ReplyEphemeral(s, ic, msg)
				}

			case "policy":
				if !r.requireAdminOrRoles(s, ic) {
					return
				}
				if len(cmd.Options) == 0 {
					ReplyEphemeral(s, ic, "Usa `/policy show` o `/policy set`.")
					return
				}
				sub := cmd.Options[0].Name
				switch sub {
				case "show":
					msg, err := r.policy.Show(ctx, ic.GuildID)
					if err != nil {
						ReplyEphemeral(s, ic, "⚠️ No pude obtener la policy: "+err.Error())
						return
					}
					ReplyEphemeral(s, ic, msg)

				case "set":
					var patch service.PolicyPatch
					if len(cmd.Options[0].Options) > 0 {
						for _, opt := range cmd.Options[0].Options {
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
							case "drop_if_left_seconds":
								v := int(opt.IntValue())
								patch.DropIfLeftSeconds = &v
							case "cooldown_after_loss_seconds":
								v := int(opt.IntValue())
								patch.CooldownAfterLossSeconds = &v
							}
						}
					}
					msg, err := r.policy.Update(ctx, ic.GuildID, patch)
					if err != nil {
						ReplyEphemeral(s, ic, "⚠️ No pude actualizar: "+err.Error())
						return
					}
					ReplyEphemeral(s, ic, "✅ Policy actualizada.\n"+msg)
				}

			case "queueui":
				// Publica o repostea la UI de la cola en ESTE canal
				if err := r.publishQueueUI(ctx, ic.GuildID, ic.ChannelID); err != nil {
					ReplyEphemeral(s, ic, "⚠️ No pude publicar la UI: "+err.Error())
					return
				}
				ReplyEphemeral(s, ic, "✅ UI publicada aquí. Usa los botones para unirte/salir.")

			case "roomsdemo":
				if !r.requireAdminOrRoles(s, ic) {
					ReplyEphemeral(s, ic, "Solo admins.")
					return
				}

				var team1Raw, team2Raw, matchID, name1, name2 string
				var cleanup bool
				for _, opt := range cmd.Options {
					switch opt.Name {
					case "team1":
						team1Raw = opt.StringValue()
					case "team2":
						team2Raw = opt.StringValue()
					case "match":
						matchID = opt.StringValue()
					case "name1":
						name1 = opt.StringValue()
					case "name2":
						name2 = opt.StringValue()
					case "cleanup":
						cleanup = opt.BoolValue()
					}
				}

				if matchID == "" {
					matchID = fmt.Sprintf("demo-%d", time.Now().UnixNano())
				}

				if cleanup {
					if err := r.rooms.DebugCleanup(context.Background(), matchID); err != nil {
						ReplyEphemeral(s, ic, "⚠️ Cleanup: "+err.Error())
						return
					}
					ReplyEphemeral(s, ic, "🧹 Limpieza ok.")
					return
				}

				team1 := parseIDs(team1Raw)
				team2 := parseIDs(team2Raw)
				if len(team1) == 0 && len(team2) == 0 {
					ReplyEphemeral(s, ic, "Pasa menciones o IDs en team1/team2.")
					return
				}

				if err := r.rooms.DebugEnsureRooms(context.Background(), matchID); err != nil {
					ReplyEphemeral(s, ic, "⚠️ ensure: "+err.Error())
					return
				}
				if err := r.rooms.DebugMoveDiscord(context.Background(), matchID, team1, team2, name1, name2); err != nil {
					ReplyEphemeral(s, ic, "⚠️ move: "+err.Error())
					return
				}
				ReplyEphemeral(s, ic, "✅ Listo. Match **"+matchID+"** creado y jugadores movidos si estaban en voz.")
				return

			}

		case discordgo.InteractionMessageComponent:
			// Click en botón
			data := ic.MessageComponentData()

			_ = DeferEphemeral(s, ic)

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			switch data.CustomID {
			case "queue_join":
				// Validación de voz si aplica
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
				msg, err := r.queue.Leave(ctx, ic.GuildID, ic.Member.User.ID)
				if err != nil {
					msg = "⚠️ No se pudo salir de la cola: " + err.Error()
				}
				ReplyEphemeral(r.s, ic, msg)
				go r.refreshQueueUI(ic.GuildID)

			case "admin_panel":
				// Chequeo de permisos (usa tu helper exist. requireAdminOrRoles)
				if !r.requireAdminOrRoles(s, ic) {
					return
				}

				// Ya hay un DeferEphemeral arriba de este handler. Construimos el selector efímero.
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// Traemos hasta 25 usuarios de la cola (los primeros)
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
					desc := fmt.Sprintf("@%s · %s", it.DiscordUserID, it.Status)
					if len(desc) > 100 {
						desc = desc[:100]
					}
					opts = append(opts, discordgo.SelectMenuOption{
						Label:       label,
						Value:       "uid:" + it.DiscordUserID, // lo parseamos luego
						Description: desc,
					})
				}

				row := discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "kick_select",
							Placeholder: "Selecciona a quién kickear",
							// MinValues:   1,
							// MaxValues:   1,
							Options: opts,
						},
					},
				}

				// Respondemos con un follow-up efímero que contiene el selector
				_, err = s.FollowupMessageCreate(ic.Interaction, true, &discordgo.WebhookParams{
					Content:    "Elige un jugador para **kickear**:",
					Components: []discordgo.MessageComponent{row},
				})
				if err != nil {
					ReplyEphemeral(s, ic, "⚠️ No pude mostrar el panel admin: "+err.Error())
				}
				return

			case "kick_select":
				// Sólo admins
				if !r.requireAdminOrRoles(s, ic) {
					return
				}
				data := ic.MessageComponentData()
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
				return

			}
		}
	})

	// VoiceStateUpdate → marca valid/afk/left (luego el pruner expulsa tras la “gracia”)
	r.s.AddHandler(func(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
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
	})
	go r.runCountdownRefresher()
}

// ---------- UI helpers ----------

func (r *Router) publishQueueUI(ctx context.Context, guildID, channelID string) error {
	embed, comps, err := r.renderQueueEmbed(ctx, guildID)
	if err != nil {
		return err
	}
	msg, err := r.s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{comps}, // <- slice
	})
	if err != nil {
		return err
	}
	return r.uiStorage.Upsert(ctx, guildID, channelID, msg.ID)
}

func (r *Router) refreshQueueUI(guildID string) {
	r.refreshMu.Lock()

	if r.refreshTimer != nil {
		r.refreshTimer.Stop()
	}
	r.refreshTimer = time.AfterFunc(120*time.Millisecond, func() {
		// 1) PRUNE rápido
		ctxP, cancelP := context.WithTimeout(context.Background(), 2*time.Second)
		pol, _ := r.policy.GetPolicy(ctxP, guildID)
		afk := time.Duration(pol.AFKTimeoutSeconds) * time.Second
		left := time.Duration(pol.DropIfLeftSeconds) * time.Second
		_, _, _ = r.queue.Prune(ctxP, guildID, afk, left)
		cancelP()

		// 2) Render & Edit
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		ui, err := r.uiStorage.Get(ctx, guildID)
		if err != nil || ui.QueueChannelID == "" || ui.QueueMessageID == "" {
			return
		}

		embed, comps, err := r.renderQueueEmbed(ctx, guildID)
		if err != nil {
			return
		}

		em := []*discordgo.MessageEmbed{embed}
		cc := []discordgo.MessageComponent{comps}
		_, _ = r.s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    ui.QueueChannelID,
			ID:         ui.QueueMessageID,
			Embeds:     &em,
			Components: &cc,
		})
	})
	r.refreshMu.Unlock()
}

func faceitPlayerURL(nick string) string {
	return "https://www.faceit.com/en/players/" + url.PathEscape(nick)
}

func fmtRemain(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func (r *Router) renderQueueEmbed(ctx context.Context, guildID string) (*discordgo.MessageEmbed, discordgo.MessageComponent, error) {
	pol, _ := r.policy.GetPolicy(ctx, guildID)
	graceAFK := time.Duration(pol.AFKTimeoutSeconds) * time.Second
	graceLeft := time.Duration(pol.DropIfLeftSeconds) * time.Second

	items, err := r.queue.ListRichWithGrace(ctx, guildID, 50, graceAFK, graceLeft)
	if err != nil {
		return nil, nil, err
	}

	// Viene ordenado por joined_at; por si acaso garantizamos orden estable
	sort.SliceStable(items, func(i, j int) bool { return items[i].JoinedAt.Before(items[j].JoinedAt) })

	lines := "Nadie en cola."
	var nextRefresh time.Duration = 0

	if len(items) > 0 {
		var b strings.Builder
		for i, it := range items {
			badge := levelBadge(it.SkillLevel)
			nick := fmt.Sprintf("[%s](%s)", it.Nickname, faceitPlayerURL(it.Nickname))
			mention := "<@" + it.DiscordUserID + ">"

			suf := " (waiting)"
			switch it.Status {
			case "left":
				if graceLeft > 0 {
					until := it.LastSeenAt.Add(graceLeft)
					remain := time.Until(until)
					if remain <= 5*time.Second {
						// texto plano para evitar “ago”
						suf = " (left " + fmtRemain(remain) + ")"
					} else {
						suf = fmt.Sprintf(" (left <t:%d:R>)", until.Unix())
					}
					if remain > 0 && (nextRefresh == 0 || remain < nextRefresh) {
						nextRefresh = remain
					}
				} else {
					suf = " (left)"
				}
			case "afk":
				if graceAFK > 0 {
					until := it.LastSeenAt.Add(graceAFK)
					remain := time.Until(until)
					if remain <= 5*time.Second {
						suf = " (afk " + fmtRemain(remain) + ")"
					} else {
						suf = fmt.Sprintf(" (afk <t:%d:R>)", until.Unix())
					}
					if remain > 0 && (nextRefresh == 0 || remain < nextRefresh) {
						nextRefresh = remain
					}
				} else {
					suf = " (afk)"
				}
			}

			if badge != "" {
				fmt.Fprintf(&b, "%d) %s %s — %s%s\n", i+1, badge, nick, mention, suf)
			} else {
				fmt.Fprintf(&b, "%d) **%s** — %s%s\n", i+1, it.Nickname, mention, suf)
			}
		}
		lines = b.String()
	}

	// Agenda refresh justo después del primer vencimiento
	if nextRefresh > 0 {
		// pequeño colchón para que el prune con <= ya lo quite
		go r.scheduleRefresh(guildID, nextRefresh+200*time.Millisecond)
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

func (r *Router) runCountdownRefresher() {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for range t.C {
		// ¿Tenemos UI publicada para este guild?
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		ui, err := r.uiStorage.Get(ctx, r.guildID)
		cancel()
		if err != nil || ui.QueueChannelID == "" || ui.QueueMessageID == "" {
			continue
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
			r.refreshQueueUI(r.guildID) // usa tu debounce (120ms) para evitar flood
		}
	}
}

// requireAdminOrRoles: owner, o Administrator bit, o alguno de los adminRoleIDs
func (r *Router) requireAdminOrRoles(s *discordgo.Session, ic *discordgo.InteractionCreate) bool {
	// Owner siempre OK
	if g, _ := s.State.Guild(ic.GuildID); g != nil && ic.Member != nil && ic.Member.User != nil && ic.Member.User.ID == g.OwnerID {
		return true
	}

	// Si tiene el bit Administrator en alguno de sus roles → OK
	roles, _ := s.GuildRoles(ic.GuildID)
	var perms int64
outer:
	for _, rid := range ic.Member.Roles {
		for _, ro := range roles {
			if ro.ID == rid {
				perms |= ro.Permissions
				if (perms & discordgo.PermissionAdministrator) != 0 {
					break outer
				}
			}
		}
	}
	if (perms & discordgo.PermissionAdministrator) != 0 {
		return true
	}

	// Si pertenece a alguno de los roles admin del bot → OK
	if len(r.adminRoleIDs) > 0 {
		has := make(map[string]struct{}, len(ic.Member.Roles))
		for _, rid := range ic.Member.Roles {
			has[rid] = struct{}{}
		}
		for _, want := range r.adminRoleIDs {
			if _, ok := has[want]; ok {
				return true
			}
		}
	}

	ReplyEphemeral(s, ic, "🔒 No tienes permisos para esta acción.")
	return false
}

// ---------- helpers voz ----------

func (r *Router) safeGetChannel(id string) (*discordgo.Channel, error) {
	if ch, err := r.s.State.Channel(id); err == nil && ch != nil {
		return ch, nil
	}
	ch, err := r.s.Channel(id)
	if err != nil {
		return nil, err
	}
	_ = r.s.State.ChannelAdd(ch) // devuelve sólo error
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
