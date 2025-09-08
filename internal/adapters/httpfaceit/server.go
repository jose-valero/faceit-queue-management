package httpfaceit

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/jose-valero/faceit-queue-bot/internal/infra/storage"
)

type Server struct {
	secret string
	users  *storage.UserRepo
	mux    *http.ServeMux
}

func New(secret string, users *storage.UserRepo) *Server {
	s := &Server{secret: secret, users: users, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/faceit/webhook", s.handleWebhook)
}

// sin uso por ahora
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

	playerID := ""
	if p, ok := evt["payload"].(map[string]any); ok {
		if s, ok := p["user_id"].(string); ok {
			playerID = s
		}
		if s, ok := p["player_id"].(string); ok {
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
	w.WriteHeader(http.StatusOK)
}

func (s *Server) Start(addr string) {
	log.Printf("🌐 HTTP listening on %s", addr)
	if err := http.ListenAndServe(addr, s.mux); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
