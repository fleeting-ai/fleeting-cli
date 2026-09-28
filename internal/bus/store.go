package bus

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store interface {
	InsertMessage(r Record) (inserted bool, err error)
	Unread(to string) (*Record, error)
	MarkListened(id, to string) error
	LastConsumed(n int) ([]Record, error)
	Log(q LogQuery) ([]Record, error)

	GetRules() (*Doc, error)
	AppendRules(by, body string) error
	RulesLog() ([]Doc, error)
	SeedRules(body string) error

	GetOnboarding(agent string) (*Doc, error)
	AppendOnboarding(agent, by, body string) error
	SeedOnboarding(agent, body string) error

	GetMemory(scope, key string) (*Memory, error)
	ListMemory(scope string, limit int) ([]Memory, error)
	AppendMemory(scope, key, by, body string) (int, error)
	MemoryLog(scope, key string) ([]Memory, error)
	CountMemory(scope string) (int, error)

	Ping() error
	Close() error
	Driver() string
	Target() string
}

type Doc struct {
	Agent     string
	Version   int
	Body      string
	UpdatedBy string
	UpdatedAt string
}

type Memory struct {
	Scope     string
	Key       string
	Version   int
	Body      string
	UpdatedBy string
	UpdatedAt string
}

type LogQuery struct {
	As    string
	Team  string
	Teams []string
	All   bool
	Limit int
}

func Open(raw string) (Store, error) {
	kind, target := ParseDB(raw)
	if kind == "postgres" {
		return openPostgres(target)
	}
	return openSQLite(target)
}

func ParseDB(raw string) (kind, target string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "sqlite", filepath.Join(DirFallback(), "bus.db")
	}
	low := strings.ToLower(raw)
	if strings.HasPrefix(low, "postgres://") || strings.HasPrefix(low, "postgresql://") {
		return "postgres", raw
	}
	raw = strings.TrimPrefix(raw, "sqlite://")
	if strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			raw = filepath.Join(home, raw[2:])
		}
	}
	return "sqlite", raw
}

func DBURL(cfgBus string) string {
	if v := strings.TrimSpace(os.Getenv("FLEETING_DB")); v != "" {
		return v
	}
	if strings.TrimSpace(cfgBus) != "" {
		return cfgBus
	}
	return filepath.Join(DirFallback(), "bus.db")
}

func AMQPURL(cfgURL string) string {
	if v := strings.TrimSpace(os.Getenv("FLEETING_AMQP")); v != "" {
		return v
	}
	if strings.TrimSpace(cfgURL) != "" {
		return cfgURL
	}
	return "amqp://guest:guest@127.0.0.1:5672/"
}

func DirFallback() string {
	if d := os.Getenv("FLEETING_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fleeting"
	}
	return filepath.Join(home, ".fleeting")
}

func RedactURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "****")
	}
	return u.String()
}

func ValidMemoryKey(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for i, r := range key {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		if i > 0 && (r == '_' || r == '-') {
			continue
		}
		return false
	}
	return true
}

func ParseScope(scope string) (kind, team string, err error) {
	scope = strings.TrimSpace(scope)
	if scope == "fleet" {
		return "fleet", "", nil
	}
	if strings.HasPrefix(scope, "team:") {
		id := strings.TrimPrefix(scope, "team:")
		if id == "" {
			return "", "", fmt.Errorf("unknown scope %s", scope)
		}
		return "team", id, nil
	}
	return "", "", fmt.Errorf("unknown scope %s", scope)
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func scanNull(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
