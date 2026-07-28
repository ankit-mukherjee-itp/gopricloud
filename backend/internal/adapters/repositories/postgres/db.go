package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// schema is applied on startup so the service can run against a bare
// Postgres instance without a separate migration step.
const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            UUID PRIMARY KEY,
	name          TEXT NOT NULL,
	email         TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at    TIMESTAMPTZ NOT NULL,
	updated_at    TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
	id         UUID PRIMARY KEY,
	user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at TIMESTAMPTZ NOT NULL,
	revoked    BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE TABLE IF NOT EXISTS compute (
	id                  UUID PRIMARY KEY,
	user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	compute_service_id  TEXT NOT NULL UNIQUE,
	name                TEXT NOT NULL,
	status              TEXT NOT NULL,
	created_at          TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_compute_user_id ON compute(user_id);
`

// Open connects to Postgres via dsn and ensures the schema exists.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return db, nil
}
