package discord

import "github.com/bwmarrin/discordgo"

func (r *Router) requireAdminOrRoles(s *discordgo.Session, ic *discordgo.InteractionCreate) bool {
	// Owner
	if g, _ := s.State.Guild(ic.GuildID); g != nil && ic.Member != nil && ic.Member.User != nil && ic.Member.User.ID == g.OwnerID {
		return true
	}

	// Administrator bit
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

	// Roles explícitos del bot
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
