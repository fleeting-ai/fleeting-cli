package bus

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE messages (
	id TEXT NOT NULL,
	to_agent TEXT NOT NULL,
	from_agent TEXT NOT NULL,
	team TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL,
	rank INTEGER NOT NULL,
	is_order INTEGER NOT NULL,
	thread TEXT NOT NULL DEFAULT '',
	reply_to TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL,
	sent_at TEXT NOT NULL,
	consumed_at TEXT NOT NULL,
	listened_at TEXT,
	PRIMARY KEY (id, to_agent)
);
CREATE TABLE rules (
	version INTEGER PRIMARY KEY,
	body TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE onboarding (
	agent TEXT NOT NULL,
	version INTEGER NOT NULL,
	body TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (agent, version)
);
CREATE TABLE memories (
	scope TEXT NOT NULL,
	key TEXT NOT NULL,
	version INTEGER NOT NULL,
	body TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (scope, key, version)
);
`

func openSQLite(path string) (Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &sqlStore{db: db, d: dialect{name: "sqlite", schema: sqliteSchema}, target: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	return s, nil
}
