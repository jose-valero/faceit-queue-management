package discord

import "github.com/bwmarrin/discordgo"

var adminPerms int64 = discordgo.PermissionAdministrator

var Commands = []*discordgo.ApplicationCommand{
	{
		Name:                     "queueui",
		Description:              "Publica o reposta la UI de la cola en este canal",
		DefaultMemberPermissions: &adminPerms,
	},
	{
		Name:        "fcplayer",
		Description: "FACEIT: busca a jugador por nickname",
		Options: []*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "nick",
			Description: "Nickname en FACEIT",
			Required:    true,
		}},
	},
	{
		Name:        "link",
		Description: "XCG: Vincula tu cuenta de FACEIT (via nickname)",
		Options: []*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "nick",
			Description: "Tu nickname en FACEIT",
			Required:    true,
		}},
	},
	{
		Name:        "whoami",
		Description: "Muestra tu link FACEIT ↔ Discord",
	},
	{
		Name:        "unlink",
		Description: "Desvincula tu cuenta FACEIT del bot",
	},
	{
		Name:        "queue",
		Description: "Gestiona la cola XCG",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "join", Description: "Unirte a la cola"},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "leave", Description: "Salir de la cola"},
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "status", Description: "Ver la cola"},
		},
	},
	{
		Name:                     "policy",
		Description:              "Ver o cambiar reglas de la cola (admins)",
		DefaultMemberPermissions: &adminPerms,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "show", Description: "Ver configuración"},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "Actualizar configuración (sólo lo que pases)",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionBoolean, Name: "require_member", Description: "Requerir membresía del hub"},
					{Type: discordgo.ApplicationCommandOptionBoolean, Name: "voice_required", Description: "Requerir estar en voz"},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "afk_timeout_seconds", Description: "AFK timeout (segundos)"},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "drop_if_left_seconds", Description: "Drop si deja el server (segundos)"},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "cooldown_after_loss_seconds", Description: "Cooldown tras derrota (segundos)"},
				},
			},
		},
	},
	{
		Name:                     "roomsdemo",
		Description:              "(Admin) Crea salas demo y mueve usuarios",
		DefaultMemberPermissions: &adminPerms,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "team1", Description: "Menciones o IDs separados por espacio", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "team2", Description: "Menciones o IDs separados por espacio", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "match", Description: "Match ID (opcional)"},
			{Type: discordgo.ApplicationCommandOptionString, Name: "name1", Description: "Nombre canal 1 (opcional)"},
			{Type: discordgo.ApplicationCommandOptionString, Name: "name2", Description: "Nombre canal 2 (opcional)"},
			{Type: discordgo.ApplicationCommandOptionBoolean, Name: "cleanup", Description: "Borra las salas del match"},
		},
	},
}
