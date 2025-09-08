package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/jose-valero/faceit-queue-bot/internal/domain"
	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

// Dependencias mínimas para este servicio
type RoomsFaceit interface {
	GetMatchStats(ctx context.Context, matchID string) (*domain.MatchStats, error)
}

type RoomsUserRepo interface {
	FindDiscordByFaceitIDs(ctx context.Context, ids []string) (map[string]string, error)
}

type RoomsRepo interface {
	Get(ctx context.Context, matchID string) (storage.MatchVoiceRoom, error)
	Upsert(ctx context.Context, m storage.MatchVoiceRoom) error
	UpdateStatus(ctx context.Context, matchID string, status string) error
	Delete(ctx context.Context, matchID string) error
}

type MatchRoomsService struct {
	s              *discordgo.Session
	fc             RoomsFaceit
	users          RoomsUserRepo
	repo           RoomsRepo
	guildID        string
	categoryPrefix string // ej: "XCG Match"
}

func NewMatchRoomsService(s *discordgo.Session, fc RoomsFaceit, users RoomsUserRepo, repo RoomsRepo, guildID, categoryPrefix string) *MatchRoomsService {
	if categoryPrefix == "" {
		categoryPrefix = "XCG Faceit Match"
	}
	return &MatchRoomsService{s: s, fc: fc, users: users, repo: repo, guildID: guildID, categoryPrefix: categoryPrefix}
}

// HandleMatchEvent: llamalo con webhooks "match_status_*"
func (m *MatchRoomsService) HandleMatchEvent(ctx context.Context, matchID, status string) {
	status = strings.ToLower(status)
	log.Printf("[rooms] evt match=%s status=%s", matchID, status)

	switch {
	case strings.Contains(status, "ready") || strings.Contains(status, "ongoing") || strings.Contains(status, "started"):
		// Asegura salas y lanza un polling corto para detectar teams y mover
		if err := m.ensureRooms(ctx, matchID); err != nil {
			log.Printf("[rooms] ensureRooms: %v", err)
			return
		}
		_ = m.repo.UpdateStatus(ctx, matchID, status)
		go m.pollAndMove(ctx, matchID)

	case strings.Contains(status, "finished") || strings.Contains(status, "cancelled"):
		// Limpia
		if err := m.cleanup(ctx, matchID); err != nil {
			log.Printf("[rooms] cleanup: %v", err)
		}
		_ = m.repo.Delete(ctx, matchID)

	default:
		// otros estados: no hacemos nada
		_ = m.repo.UpdateStatus(ctx, matchID, status)
	}
}

// ---------- internos ----------

func (m *MatchRoomsService) ensureRooms(ctx context.Context, matchID string) error {
	// ¿ya existen?
	if _, err := m.repo.Get(ctx, matchID); err == nil {
		return nil
	}

	// Crea categoría y 2 voice channels
	cat, err := m.s.GuildChannelCreate(m.guildID, fmt.Sprintf("%s %s", m.categoryPrefix, shortID(matchID)), discordgo.ChannelTypeGuildCategory)
	if err != nil {
		return err
	}
	t1, err := m.s.GuildChannelCreate(m.guildID, "Team A", discordgo.ChannelTypeGuildVoice)
	if err != nil {
		return err
	}
	t2, err := m.s.GuildChannelCreate(m.guildID, "Team B", discordgo.ChannelTypeGuildVoice)
	if err != nil {
		return err
	}
	// moverlos bajo la categoría
	_, _ = m.s.ChannelEdit(t1.ID, &discordgo.ChannelEdit{ParentID: cat.ID})
	_, _ = m.s.ChannelEdit(t2.ID, &discordgo.ChannelEdit{ParentID: cat.ID})

	mv := storage.MatchVoiceRoom{
		MatchID:        matchID,
		GuildID:        m.guildID,
		CategoryID:     cat.ID,
		Team1ChannelID: t1.ID,
		Team2ChannelID: t2.ID,
	}
	return m.repo.Upsert(ctx, mv)
}

func (m *MatchRoomsService) pollAndMove(ctx context.Context, matchID string) {
	// Hasta 2 min, cada 5s, buscamos los equipos (post-knife ya queda estable)
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		t1, t2, name1, name2, err := m.readTeams(ctx, matchID)
		if err == nil && len(t1)+len(t2) > 0 {
			if err := m.moveTeams(ctx, matchID, t1, t2, name1, name2); err != nil {
				log.Printf("[rooms] moveTeams: %v", err)
			}
			return
		}
		time.Sleep(5 * time.Second)
	}
	log.Printf("[rooms] poll timeout waiting teams for match=%s", matchID)
}

func (m *MatchRoomsService) readTeams(ctx context.Context, matchID string) (team1 []string, team2 []string, name1, name2 string, err error) {
	stats, err := m.fc.GetMatchStats(ctx, matchID)
	if err != nil {
		return nil, nil, "", "", err
	}
	if stats == nil || len(stats.Rounds) == 0 {
		return nil, nil, "", "", errors.New("no rounds yet")
	}
	rd := stats.Rounds[0]
	if len(rd.Teams) < 2 {
		return nil, nil, "", "", errors.New("not enough teams")
	}
	// Tomamos "order" como Team1/Team2 (sides reales pueden ser CT/T, pero para mover sólo importa separar)
	t1 := rd.Teams[0]
	t2 := rd.Teams[1]
	for _, p := range t1.Players {
		team1 = append(team1, p.PlayerID)
	}
	for _, p := range t2.Players {
		team2 = append(team2, p.PlayerID)
	}
	name1 = firstNonEmpty(t1.Nickname, "Team A")
	name2 = firstNonEmpty(t2.Nickname, "Team B")
	return
}

func (m *MatchRoomsService) moveTeams(ctx context.Context, matchID string, team1Faceit []string, team2Faceit []string, name1, name2 string) error {
	mv, err := m.repo.Get(ctx, matchID)
	if err != nil {
		return err
	}

	// mapear a Discord IDs
	map1, err := m.users.FindDiscordByFaceitIDs(ctx, team1Faceit)
	if err != nil {
		return err
	}
	map2, err := m.users.FindDiscordByFaceitIDs(ctx, team2Faceit)
	if err != nil {
		return err
	}

	// renombrar canales con labels si tenemos
	if name1 != "" {
		_, _ = m.s.ChannelEdit(mv.Team1ChannelID, &discordgo.ChannelEdit{Name: name1})
	}
	if name2 != "" {
		_, _ = m.s.ChannelEdit(mv.Team2ChannelID, &discordgo.ChannelEdit{Name: name2})
	}
	_ = m.repo.Upsert(ctx, storage.MatchVoiceRoom{
		MatchID:        mv.MatchID,
		GuildID:        mv.GuildID,
		CategoryID:     mv.CategoryID,
		Team1ChannelID: mv.Team1ChannelID,
		Team2ChannelID: mv.Team2ChannelID,
		Team1Label:     &name1,
		Team2Label:     &name2,
	})

	// mover a cada jugador (si está en el guild y en voz en cualquier canal)
	moveOne := func(discordID, channelID string) {
		if err := m.s.GuildMemberMove(m.guildID, discordID, &channelID); err != nil {
			log.Printf("[rooms] move %s -> %s: %v", discordID, channelID, err)
		}
	}

	for _, did := range map1 {
		moveOne(did, mv.Team1ChannelID)
	}
	for _, did := range map2 {
		moveOne(did, mv.Team2ChannelID)
	}
	return nil
}

func (m *MatchRoomsService) cleanup(ctx context.Context, matchID string) error {
	mv, err := m.repo.Get(ctx, matchID)
	if err != nil {
		return err
	}
	// borrar canales y categoría
	_, _ = m.s.ChannelDelete(mv.Team1ChannelID)
	_, _ = m.s.ChannelDelete(mv.Team2ChannelID)
	_, _ = m.s.ChannelDelete(mv.CategoryID)
	return nil
}

func shortID(s string) string {
	if len(s) <= 6 {
		return s
	}
	return s[len(s)-6:]
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// DebugEnsureRooms: fuerza crear salas para un matchID (si no existen)
func (m *MatchRoomsService) DebugEnsureRooms(ctx context.Context, matchID string) error {
	return m.ensureRooms(ctx, matchID)
}

// DebugMoveDiscord: mueve directamente por Discord IDs (sin Faceit)
func (m *MatchRoomsService) DebugMoveDiscord(ctx context.Context, matchID string, team1DiscordIDs, team2DiscordIDs []string, name1, name2 string) error {
	mv, err := m.repo.Get(ctx, matchID)
	if err != nil {
		return err
	}

	// Renombra si pasaste nombres
	if strings.TrimSpace(name1) != "" {
		_, _ = m.s.ChannelEdit(mv.Team1ChannelID, &discordgo.ChannelEdit{Name: name1})
	}
	if strings.TrimSpace(name2) != "" {
		_, _ = m.s.ChannelEdit(mv.Team2ChannelID, &discordgo.ChannelEdit{Name: name2})
	}

	_ = m.repo.Upsert(ctx, storage.MatchVoiceRoom{
		MatchID:        mv.MatchID,
		GuildID:        mv.GuildID,
		CategoryID:     mv.CategoryID,
		Team1ChannelID: mv.Team1ChannelID,
		Team2ChannelID: mv.Team2ChannelID,
		Team1Label:     &name1,
		Team2Label:     &name2,
	})

	moveOne := func(discordID, channelID string) {
		if err := m.s.GuildMemberMove(m.guildID, discordID, &channelID); err != nil {
			log.Printf("[rooms] move %s -> %s: %v", discordID, channelID, err)
		}
	}

	for _, did := range team1DiscordIDs {
		moveOne(did, mv.Team1ChannelID)
	}
	for _, did := range team2DiscordIDs {
		moveOne(did, mv.Team2ChannelID)
	}
	return nil
}

// DebugCleanup: borra categoría y canales de un matchID
func (m *MatchRoomsService) DebugCleanup(ctx context.Context, matchID string) error {
	return m.cleanup(ctx, matchID)
}
