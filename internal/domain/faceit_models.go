package domain

// MatchStats es el DTO mínimo que usamos para leer equipos/jugadores.
type MatchStats struct {
	Rounds []struct {
		Teams []struct {
			Nickname string `json:"nickname"`
			Players  []struct {
				PlayerID string `json:"player_id"`
			} `json:"players"`
		} `json:"teams"`
	} `json:"rounds"`
}
