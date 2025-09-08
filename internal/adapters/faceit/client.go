package faceit

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

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

// PlayerInOngoingHub: revisa /hubs/{hubID}/matches?state=ongoing y busca al player en los rosters.
func (c *Client) PlayerInOngoingHub(ctx context.Context, playerID, hubID string) (bool, error) {
	offset := 0
	limit := 50
	for {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))
		// FACEIT usa "status" o "state" según recurso; probamos con "state=ongoing".
		// Si tu endpoint requiere "status", cambia la clave.
		q.Set("state", "ongoing")

		var dto hubMatchListDTO
		if err := c.doJSON(ctx, "GET", fmt.Sprintf("/hubs/%s/matches", hubID), q, &dto); err != nil {
			return false, err
		}
		if len(dto.Items) == 0 {
			return false, nil
		}
		for _, m := range dto.Items {
			for _, t := range m.Teams {
				for _, p := range t.Players {
					if p.UserID == playerID {
						return true, nil
					}
				}
			}
		}
		if len(dto.Items) < limit {
			return false, nil
		}
		offset += limit
	}
}

// LastMatchLossWithin: mira el último match del player y si fue derrota dentro de 'within', devuelve true y el "finished_at".
func (c *Client) LastMatchLossWithin(ctx context.Context, playerID, game string, within time.Duration) (bool, time.Time, error) {
	q := url.Values{}
	q.Set("game", game)
	q.Set("limit", "1")

	var dto playerHistoryDTO
	if err := c.doJSON(ctx, "GET", fmt.Sprintf("/players/%s/history", playerID), q, &dto); err != nil {
		return false, time.Time{}, err
	}
	if len(dto.Items) == 0 {
		return false, time.Time{}, nil
	}
	it := dto.Items[0]

	// ⚠️ Algunas respuestas vienen en milisegundos. Si notas que el tiempo es enorme, divide por 1000.
	ended := time.Unix(it.FinishedAt, 0)
	// Heurística de resultado
	res := strings.ToLower(strings.TrimSpace(it.Result))
	lost := res == "lose" || res == "lost" || res == "defeat" || res == "defeated"

	if !lost {
		return false, time.Time{}, nil
	}
	if time.Since(ended) < within {
		return true, ended, nil
	}
	return false, time.Time{}, nil
}
