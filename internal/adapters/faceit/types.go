package faceit

// --- Players ---
type playerDTO struct {
	PlayerID string `json:"player_id"`
	Nickname string `json:"nickname"`
	Games    map[string]struct {
		FaceitElo  int `json:"faceit_elo"`
		SkillLevel int `json:"skill_level"`
	} `json:"games"`
}

type playerHubsDTO struct {
	Items []struct {
		HubID string `json:"hub_id"`
		Name  string `json:"name"`
	} `json:"items"`
}

type playerStatsDTO struct {
	Lifetime map[string]string `json:"lifetime"`
}

// --- Hubs  ---
type hubMembersDTO struct {
	Items []struct {
		UserID   string `json:"user_id"`
		Nickname string `json:"nickname"`
	} `json:"items"`
}

// --- Matches (detalle) ---
type matchDTO struct {
	MatchID string   `json:"match_id"`
	DemoURL []string `json:"demo_url"`
	// agrega campos según necesites
}

// --- Match Stats ---
type matchStatsDTO struct {
	Rounds []struct {
		Teams []struct {
			Players []struct {
				PlayerID  string `json:"player_id"`
				Nickname  string `json:"nickname"`
				Kills     int    `json:"kills"`
				Deaths    int    `json:"deaths"`
				Headshots int    `json:"headshots"`
				// agrega ADR, etc. cuando lo uses
			} `json:"players"`
		} `json:"teams"`
	} `json:"rounds"`
}
