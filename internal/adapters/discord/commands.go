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
}
