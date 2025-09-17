-- +goose Up
CREATE TABLE IF NOT EXISTS faceit_match_status (
  match_id   TEXT PRIMARY KEY,
  hub_id     TEXT,
  status     TEXT NOT NULL,     -- created|configuring|ready|finished|cancelled|aborted
  demo_url   TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_event TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_fms_hub ON faceit_match_status (hub_id);
CREATE INDEX IF NOT EXISTS idx_fms_status ON faceit_match_status (status);

-- +goose Down
DROP INDEX IF EXISTS idx_fms_status;
DROP INDEX IF EXISTS idx_fms_hub;
DROP TABLE IF EXISTS faceit_match_status;
