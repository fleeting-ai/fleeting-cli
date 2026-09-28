package bus

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const pgSchema = `
CREATE TABLE IF NOT EXISTS messages (
	id TEXT NOT NULL,
	to_agent TEXT NOT NULL,
	from_agent TEXT NOT NULL,
	team TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL,
	rank INTEGER NOT NULL,
	is_order BOOLEAN NOT NULL,
	thread TEXT NOT NULL DEFAULT '',
	reply_to TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL,
	sent_at TEXT NOT NULL,
	consumed_at TEXT NOT NULL,
	listened_at TEXT,
	PRIMARY KEY (id, to_agent)
);
CREATE TABLE IF NOT EXISTS rules (
	version INTEGER PRIMARY KEY,
	body TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS onboarding (
	agent TEXT NOT NULL,
	version INTEGER NOT NULL,
	body TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (agent, version)
);
CREATE TABLE IF NOT EXISTS memories (
	scope TEXT NOT NULL,
	key TEXT NOT NULL,
	version INTEGER NOT NULL,
	body TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (scope, key, version)
);
`

func openPostgres(dsn string) (Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: %w", err)
	}
	s := &sqlStore{db: db, d: dialect{name: "postgres", schema: pgSchema}, target: dsn}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}
	return s, nil
}
