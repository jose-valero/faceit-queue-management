package storage

import "time"

type QueueEntry struct {
	GuildID       string
	DiscordUserID string
	FaceitUserID  string
	Nickname      string
	JoinedAt      time.Time
	LastSeenAt    time.Time
	Status        string // waiting | afk | left
}

type GuildPolicy struct {
	GuildID           string
	RequireMember     bool
	AFKTimeoutSeconds int
	DropIfLeftMinutes int
	VoiceRequired     bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Para updates parciales desde /policy set
type GuildPolicyUpdate struct {
	RequireMember     *bool
	AFKTimeoutSeconds *int
	DropIfLeftMinutes *int
	VoiceRequired     *bool
}
