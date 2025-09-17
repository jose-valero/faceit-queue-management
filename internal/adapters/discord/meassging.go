package discord

import (
	"errors"
	"log"

	"github.com/bwmarrin/discordgo"
)

func SendEphemeral(s *discordgo.Session, ic *discordgo.InteractionCreate, msg string) error {
	err := s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("SendEphemeral error: %v", err)
	}
	return err
}

// Defer efímero (para trabajos >3s)
func DeferEphemeral(s *discordgo.Session, ic *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			// AllowedMentions: &discordgo.AllowedMentions{},
		},
	})
	if err != nil {
		log.Printf("DeferEphemeral error: %v", err)
	}
	return err
}

func ReplyEphemeral(s *discordgo.Session, ic *discordgo.InteractionCreate, content string, embeds ...*discordgo.MessageEmbed) {
	_, err := s.FollowupMessageCreate(ic.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Embeds:  embeds,
		// AllowedMentions: &discordgo.AllowedMentions{},
	})

	if err != nil {
		// Fallback sólo si todavía no hay respuesta (webhook desconocido)
		var reqErr *discordgo.RESTError
		if errors.As(err, &reqErr) && reqErr.Message != nil && reqErr.Message.Code == 10015 {
			_ = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: content,
					Flags:   discordgo.MessageFlagsEphemeral,
					Embeds:  embeds,
				},
			})
			return
		}
		log.Printf("ReplyEphemeral error: %v", err)
	}
}

func EditOriginalEphemeral(s *discordgo.Session, ic *discordgo.InteractionCreate, params *discordgo.WebhookEdit) {
	_, err := s.InteractionResponseEdit(ic.Interaction, params)
	if err != nil {
		log.Printf("EditOriginalEphemeral error: %v", err)
	}
}

// respuesta publica para algun comando que no sea efimero
func SendResponse(s *discordgo.Session, ic *discordgo.InteractionCreate, msg string) error {
	err := s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			// AllowedMentions: &discordgo.AllowedMentions{},
		},
	})
	if err != nil {
		log.Printf("SendResponse error: %v", err)
	}
	return err
}
