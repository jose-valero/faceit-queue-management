package httpfaceit

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

type Server struct {
	secret       string
	users        *storage.UserRepo
	mux          *http.ServeMux
	onMatchEvent func(ctx context.Context, matchID, status string)
}

func New(secret string, users *storage.UserRepo, onMatch func(ctx context.Context, matchID, status string)) *Server {
	s := &Server{secret: secret, users: users, mux: http.NewServeMux(), onMatchEvent: onMatch}
	s.routes()
	return s
}

func NewCompat(secret string, users *storage.UserRepo) *Server {
	return New(secret, users, nil)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/faceit/webhook", s.handleWebhook)
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-FACEIT-WH") != s.secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	_ = r.Body.Close()

	var evt map[string]any
	_ = json.Unmarshal(body, &evt)
	t, _ := evt["event"].(string)

	// payload puede traer player/match
	var payload map[string]any
	if p, ok := evt["payload"].(map[string]any); ok {
		payload = p
	}

	// ——— Membresía hub (tu lógica actual) ———
	playerID := ""
	if payload != nil {
		if s, ok := payload["user_id"].(string); ok {
			playerID = s
		}
		if s, ok := payload["player_id"].(string); ok {
			playerID = s
		}
	}

	switch strings.ToLower(t) {
	case "hub_user_added":
		if playerID != "" {
			_ = s.users.UpdateMembershipByFaceitID(r.Context(), playerID, true)
		}
		log.Printf("webhook: member_added player=%s", playerID)

	case "hub_user_removed":
		if playerID != "" {
			_ = s.users.UpdateMembershipByFaceitID(r.Context(), playerID, false)
		}
		log.Printf("webhook: member_removed player=%s", playerID)
	}

	// ——— Match status (opcional) ———
	if s.onMatchEvent != nil && strings.HasPrefix(strings.ToLower(t), "match_status_") {
		matchID := ""
		if payload != nil {
			if mid, ok := payload["match_id"].(string); ok {
				matchID = mid
			}
		}
		if matchID != "" {
			status := strings.TrimPrefix(strings.ToLower(t), "match_status_")
			if st, ok := payload["status"].(string); ok && st != "" {
				status = st
			}
			go s.onMatchEvent(r.Context(), matchID, status)
			log.Printf("webhook: match %s status=%s", matchID, status)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) Start(addr string) {
	log.Printf("🌐 HTTP listening on %s", addr)
	if err := http.ListenAndServe(addr, s.mux); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
