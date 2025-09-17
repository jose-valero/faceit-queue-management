package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	db          *pgxpool.Pool
	secretHdr   = getenv("WEBHOOK_HEADER_NAME", "X-Faceit-Secret")
	secretValue = os.Getenv("WEBHOOK_HEADER_VALUE")
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func init() {
	// DB opcional (si DATABASE_URL está vacío, igual respondemos 200)
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Println("DATABASE_URL empty; running without DB")
		return
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Println("pgx ParseConfig:", err)
		return
	}
	cfg.MaxConns = 4
	cfg.MaxConnLifetime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fmt.Println("pgxpool New:", err)
		return
	}
	db = pool

	// defensivo: asegurar tablas mínimas
	_, _ = db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS webhook_dedup (
  dedup_key  TEXT PRIMARY KEY,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS faceit_match_status (
  match_id   TEXT PRIMARY KEY,
  hub_id     TEXT,
  status     TEXT NOT NULL,
  demo_url   TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_event TIMESTAMPTZ
);`)
}

func readSecret(req events.APIGatewayV2HTTPRequest) string {
	// headers candidatos
	for _, k := range []string{
		strings.ToLower(os.Getenv("WEBHOOK_HEADER_NAME")), // ej: x-faceit-wh
		"x-faceit-wh",
		"x-faceit-secret",
	} {
		if k == "" {
			continue
		}
		if v := req.Headers[k]; v != "" {
			return v
		}
		if v := req.Headers[strings.ToUpper(k)]; v != "" {
			return v
		}
	}
	// query param candidato (configurable)
	qname := getenv("WEBHOOK_QUERY_NAME", "wh")
	if v := req.QueryStringParameters[strings.ToLower(qname)]; v != "" {
		return v
	}
	if v := req.QueryStringParameters[qname]; v != "" {
		return v
	}
	return ""
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// LOGS ÚTILES (se ven siempre)
	ua := ""
	ip := ""
	if req.RequestContext.HTTP.Method != "" { // HTTP API v2
		ua = req.RequestContext.HTTP.UserAgent
		ip = req.RequestContext.HTTP.SourceIP
	}
	fmt.Printf("webhook hit | path=%s method=%s ip=%s ua=%q b64=%v headers=%d\n",
		req.RawPath, req.RequestContext.HTTP.Method, ip, ua, req.IsBase64Encoded, len(req.Headers))

	// 1) validar secreto (una sola vez)
	got := readSecret(req)
	if secretValue == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secretValue)) != 1 {
		fmt.Println("auth: unauthorized (missing/invalid secret)")
		return events.APIGatewayV2HTTPResponse{StatusCode: 401, Body: "unauthorized"}, nil
	}

	// 2) body crudo
	body := req.Body
	if req.IsBase64Encoded {
		dec, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			fmt.Println("body: invalid base64")
			return events.APIGatewayV2HTTPResponse{StatusCode: 400, Body: "invalid base64"}, nil
		}
		body = string(dec)
	}

	// 2.5) persistir evento y notificar al bot (si hay DB)
	if db != nil && body != "" {
		var evt map[string]any
		_ = json.Unmarshal([]byte(body), &evt)
		t := str(get(evt, "type"))
		if t == "" {
			// algunos payloads usan "event", caemos ahí
			t = str(get(evt, "event"))
			if t == "" {
				t = "unknown"
			}
		}

		// insert + notify
		var id int64
		ctxIns, cancelIns := context.WithTimeout(ctx, 2*time.Second)
		err := db.QueryRow(ctxIns,
			`INSERT INTO webhook_events(type, payload) VALUES ($1, $2::jsonb) RETURNING id`,
			t, body,
		).Scan(&id)
		cancelIns()
		if err != nil {
			fmt.Println("events insert:", err)
		} else {
			// pg_notify para que el bot escuche en tiempo real
			_, _ = db.Exec(context.Background(),
				`SELECT pg_notify('faceit_webhook', $1)`, fmt.Sprint(id),
			)
		}
	}

	// 3) dedup (si hay DB)
	if db != nil {
		sum := sha256.Sum256([]byte(body))
		key := hex.EncodeToString(sum[:])

		dctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if _, err := db.Exec(dctx, `INSERT INTO webhook_dedup(dedup_key) VALUES ($1) ON CONFLICT DO NOTHING`, key); err != nil {
			fmt.Println("dedup insert error:", err)
		}
		cancel()
	}

	// 4) router (opcional)
	if db != nil && body != "" {
		var evt map[string]any
		_ = json.Unmarshal([]byte(body), &evt)
		_ = processEvent(ctx, db, evt)
	}

	// 5) OK
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"ok":true}`,
	}, nil
}

func main() { lambda.Start(handler) }

// ---------- dominio mínimo ----------
type Repo struct{ db *pgxpool.Pool }

func (r Repo) UpsertMatchStatus(ctx context.Context, matchID, hubID, status, demoURL string, lastEvent *time.Time) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO faceit_match_status (match_id, hub_id, status, demo_url, last_event)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (match_id) DO UPDATE
SET status = EXCLUDED.status,
    demo_url = EXCLUDED.demo_url,
    last_event = EXCLUDED.last_event,
    updated_at = now()
`, matchID, hubID, status, nullIfEmpty(demoURL), lastEvent)
	return err
}

func processEvent(ctx context.Context, pool *pgxpool.Pool, evt map[string]any) error {
	r := Repo{db: pool}
	t := str(get(evt, "type"))

	// Faceit suele usar "data" o "payload"
	data := obj(evt["data"])
	if len(data) == 0 {
		data = obj(evt["payload"])
	}

	matchID := firstNonEmpty(str(get(data, "match_id")), str(get(data, "matchId")))
	hubID := firstNonEmpty(str(get(data, "hub_id")), str(get(data, "hubId")), str(get(evt, "hub_id")))
	demoURL := firstNonEmpty(str(get(data, "demo_url")), str(get(data, "demoUrl")))
	now := time.Now().UTC()

	var err error
	switch t {
	case "match_object_created":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "created", "", &now)
	case "match_status_configuring":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "configuring", "", &now)
	case "match_status_ready":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "ready", "", &now)
	case "match_demo_ready":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "finished", demoURL, &now)
	case "match_status_finished":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "finished", "", &now)
	case "match_status_cancelled":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "cancelled", "", &now)
	case "match_status_aborted":
		err = r.UpsertMatchStatus(ctx, matchID, hubID, "aborted", "", &now)
	default:
		// otros eventos los ignoramos por ahora
		return nil
	}
	if err != nil {
		fmt.Println("UpsertMatchStatus:", err)
	}
	return err
}

// ---------- helpers JSON ----------
func get(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}
func obj(v any) map[string]any {
	if o, ok := v.(map[string]any); ok {
		return o
	}
	return map[string]any{}
}
func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
