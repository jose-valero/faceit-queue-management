-- +goose Up
CREATE TABLE IF NOT EXISTS webhook_dedup (
  dedup_key  TEXT PRIMARY KEY,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS webhook_dedup;
