package discord

import "github.com/bwmarrin/discordgo"

var Commands = []*discordgo.ApplicationCommand{
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
		Name:        "policy",
		Description: "Ver o cambiar reglas de la cola (admins)",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "show", Description: "Ver configuración"},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "Actualizar configuración (sólo lo que pases)",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionBoolean, Name: "require_member", Description: "Requerir membresía del hub"},
					{Type: discordgo.ApplicationCommandOptionBoolean, Name: "voice_required", Description: "Requerir estar en voz"},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "afk_timeout_seconds", Description: "AFK timeout en segundos"},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "drop_if_left_minutes", Description: "Drop si deja el server (minutos)"},
				},
			},
		},
	},
}
