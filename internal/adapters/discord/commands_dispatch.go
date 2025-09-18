// esta es la logica de InteractionApplicationCommand de discordgo
// aqui solo vamos a manejar logica de la interaccion del usuario y despachar a los servicios correspondientes
package discord

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jose-valero/faceit-queue-bot/internal/app/service"
)

// esto es basicamente mi reciver function
func (r *Router) handleSlashCommand(s *discordgo.Session, ic *discordgo.InteractionCreate) {

	cmd := ic.ApplicationCommandData()
	log.Printf("cmd: %s by=%s guild=%s", cmd.Name, ic.Member.User.ID, ic.GuildID)

	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("panic in cmd /%s: %v", cmd.Name, rec)
			ReplyEphemeral(s, ic, "❌ Ocurrió un error inesperado procesando el comando. Contacta con un administrador.")
		}
	}()

	_ = DeferEphemeral(s, ic)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	switch cmd.Name {

	//--> caso de test, luego lo scamos che
	case "ping":
		ReplyEphemeral(s, ic, "🏓 Pong!")

	//--> para ver al jugador vinculado
	case "fcplayer":
		nick, _ := optStr(ic, "nick")
		msg, err := r.link.DescribeByNick(ctx, nick)
		if err != nil {
			msg = "⚠️ No pude obtener el jugador: " + err.Error()
		}
		ReplyEphemeral(s, ic, msg)

		//--> para vincular jugador, saber quien es quien
	case "link":
		nick, _ := optStr(ic, "nick")
		msg, err := r.link.Link(ctx, nick, ic.Member.User.ID, ic.GuildID)
		if err != nil {
			msg = "⚠️ No se pudo vincular: " + err.Error()
		}
		ReplyEphemeral(s, ic, msg)

		//--> para desvincular jugador
	case "unlink":
		msg, err := r.link.Unlink(ctx, ic.Member.User.ID, ic.GuildID)
		if err != nil {
			msg = "⚠️ No se pudo desvincular: " + err.Error()
		}
		ReplyEphemeral(s, ic, msg)

	//--> datos basicos para saber si estas vinculado o no, muestra hace cuando te vinculaste y el id de Faceit
	case "whoami":
		msg, err := r.link.WhoAmI(ctx, ic.Member.User.ID)
		if err != nil {
			msg = "No estás linkeado. Usa `/link nick:<tu_nick_FACEIT_tal_cual_como_esta_en_tu perfil>`"
		}
		ReplyEphemeral(s, ic, msg)

	//--> para ver e
	case "queue":
		// la interaccion por comandos la hacemos solo para admins por que es modo de prueba
		// la intencion es que el jugador se una con la UI y no con los comandos.
		if !r.requireAdminOrRoles(s, ic) {
			return
		}
		if len(cmd.Options) == 0 {
			ReplyEphemeral(s, ic, "Usa `/queue join`, `/queue leave` o `/queue status`.")
			return
		}

		sub := cmd.Options[0].Name

		// casos para interactuar con la cola
		switch sub {
		//--> para unirte en la cola
		case "join":
			// primero validamos que el usuario este en un canal de voz permitido
			stop := step("component.queue_join.total")
			defer stop()
			if pol, err := r.policy.GetPolicy(ctx, ic.GuildID); err == nil && pol.VoiceRequired {
				ok, why := r.userInAllowedVoice(ic.GuildID, ic.Member.User.ID)
				if !ok {
					ReplyEphemeral(s, ic, "🎮 Debes estar en voz. "+why)
					return
				}
			}
			msg, err := r.queue.Join(ctx, ic.GuildID, ic.Member.User.ID)
			if err != nil {
				msg = "⚠️ No se pudo unir a la cola: " + err.Error()
			}
			ReplyEphemeral(s, ic, msg)
			defer step("queue.join")()
			defer step("ui.fast")()
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

	//--> para configurar las policy por los comandos
	case "policy":
		if !r.requireAdminOrRoles(s, ic) {
			return
		}
		if sub, ok := subcmdName(ic); ok && sub == "set" {
			var patch service.PolicyPatch
			if v, ok := optBool(ic, "require_member"); ok {
				patch.RequireMember = &v
			}
			if v, ok := optBool(ic, "voice_required"); ok {
				patch.VoiceRequired = &v
			}
			if v, ok := optInt(ic, "afk_timeout_seconds"); ok {
				patch.AFKTimeoutSeconds = &v
			}
			if v, ok := optInt(ic, "drop_if_left_seconds"); ok {
				patch.DropIfLeftSeconds = &v
			}
			if v, ok := optInt(ic, "cooldown_after_loss_seconds"); ok {
				patch.CooldownAfterLossSeconds = &v
			}

			msg, err := r.policy.Update(ctx, ic.GuildID, patch)
			if err != nil {
				ReplyEphemeral(s, ic, "⚠️ No pude actualizar: "+err.Error())
				return
			}
			ReplyEphemeral(s, ic, "✅ Policy actualizada.\n"+msg)
			return
		}

	case "queueui":
		if err := r.publishQueueUI(ctx, ic.GuildID, ic.ChannelID); err != nil {
			ReplyEphemeral(s, ic, "⚠️ No pude publicar la UI: "+err.Error())
			return
		}
		ReplyEphemeral(s, ic, "✅ UI publicada aquí. Usa los botones para unirte/salir.")

		//--> este caso es para prueba
		// agregos ids de personas en canales y deberia crear y mover a los jugadores
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
		ReplyEphemeral(s, ic, "✅ Match **"+matchID+"** creado y jugadores movidos si estaban en voz.")
	}
}
