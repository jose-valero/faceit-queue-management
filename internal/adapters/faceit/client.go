package faceit

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/jose-valero/faceit-queue-bot/internal/domain"
)

func (c *Client) IsMemberOfHub(ctx context.Context, playerID, hubID string) (bool, error) {
	offset := 0
	limit := 50 // debe ser un string no admite numericos che
	for {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))

		var dto hubMembersDTO
		if err := c.doJSON(ctx, "GET", fmt.Sprintf("/hubs/%s/members", hubID), q, &dto); err != nil {
			return false, err
		}

		if len(dto.Items) == 0 {
			return false, nil // fin de lista
		}
		for _, it := range dto.Items {
			if it.UserID == playerID {
				return true, nil
			}
		}
		if len(dto.Items) < limit { // <= 50: última página
			return false, nil
		}
		offset += limit
	}
}

// func (c *Client) PlayerHasHub(ctx context.Context, playerID, hubID string) (bool, error) {
// 	q := url.Values{}
// 	q.Set("limit", "100") // más que suficiente para la mayoría de cuentas
// 	var dto playerHubsDTO
// 	if err := c.doJSON(ctx, "GET", fmt.Sprintf("/players/%s/hubs", playerID), q, &dto); err != nil {
// 		return false, err
// 	}
// 	for _, it := range dto.Items {
// 		if it.HubID == hubID {
// 			return true, nil
// 		}
// 	}
// 	return false, nil
// }

// GetPlayerByNickname: ya existente, ahora usando doJSON
func (c *Client) GetPlayerByNickname(ctx context.Context, nick, game string) (*domain.Player, error) {
	q := url.Values{}
	q.Set("nickname", nick)
	q.Set("game", game)

	var dto playerDTO
	if err := c.doJSON(ctx, "GET", "/players", q, &dto); err != nil {
		return nil, err
	}
	g := dto.Games[game]
	return &domain.Player{ID: dto.PlayerID, Nickname: dto.Nickname, Elo: g.FaceitElo, Skill: g.SkillLevel}, nil
}

// Ejemplos de métodos que vas a necesitar pronto:

func (c *Client) GetMatch(ctx context.Context, matchID string) (*matchDTO, error) {
	var dto matchDTO
	err := c.doJSON(ctx, "GET", fmt.Sprintf("/matches/%s", matchID), nil, &dto)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func (c *Client) GetMatchStats(ctx context.Context, matchID string) (*matchStatsDTO, error) {
	var dto matchStatsDTO
	err := c.doJSON(ctx, "GET", fmt.Sprintf("/matches/%s/stats", matchID), nil, &dto)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// Si tu puerto domain.FaceitAPI aún no define estos, podés agregarlos luego.
