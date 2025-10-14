-- +goose Up
CREATE TABLE IF NOT EXISTS webhook_events (
  id         BIGSERIAL PRIMARY KEY,
  type       TEXT NOT NULL,
  payload    JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- opcional, si no la tenías:
CREATE TABLE IF NOT EXISTS webhook_dedup (
  dedup_key   TEXT PRIMARY KEY,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS webhook_dedup;