package bus

import (
	"database/sql"
	"fmt"
	"strings"
)

type dialect struct {
	name   string
	schema string
}

func (d dialect) Q(q string) string {
	if d.name != "postgres" {
		return q
	}
	n := 0
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (d dialect) boolVal(v bool) any {
	if d.name == "postgres" {
		return v
	}
	if v {
		return 1
	}
	return 0
}

type sqlStore struct {
	db     *sql.DB
	d      dialect
	target string
}

func (s *sqlStore) Driver() string { return s.d.name }
func (s *sqlStore) Target() string { return RedactURL(s.target) }
func (s *sqlStore) Ping() error    { return s.db.Ping() }
func (s *sqlStore) Close() error   { return s.db.Close() }

func (s *sqlStore) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=1`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := s.db.Exec(s.d.schema); err != nil {
		return err
	}
	_, err := s.db.Exec(s.d.Q(`INSERT INTO schema_migrations(version, applied_at) VALUES (1, ?)`), stamp())
	return err
}

func (s *sqlStore) InsertMessage(r Record) (bool, error) {
	if r.ToAgent == "" {
		r.ToAgent = r.To
	}
	if r.ConsumedAt == "" {
		r.ConsumedAt = stamp()
	}
	q := s.d.Q(`INSERT INTO messages(id, to_agent, from_agent, team, action, rank, is_order, thread, reply_to, body, sent_at, consumed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	res, err := s.db.Exec(q, r.ID, r.ToAgent, r.From, r.Team, r.Action, r.Rank, s.d.boolVal(r.Order), r.Thread, r.ReplyTo, r.Body, r.At, r.ConsumedAt)
	if err != nil {
		if isUnique(err) {
			return false, nil
		}
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *sqlStore) Unread(to string) (*Record, error) {
	q := s.d.Q(`SELECT id, to_agent, from_agent, team, action, rank, is_order, thread, reply_to, body, sent_at, consumed_at, listened_at
		FROM messages WHERE to_agent=? AND listened_at IS NULL ORDER BY consumed_at ASC, sent_at ASC LIMIT 1`)
	return s.scanOne(s.db.QueryRow(q, to))
}

func (s *sqlStore) MarkListened(id, to string) error {
	q := s.d.Q(`UPDATE messages SET listened_at=? WHERE id=? AND to_agent=? AND listened_at IS NULL`)
	_, err := s.db.Exec(q, stamp(), id, to)
	return err
}

func (s *sqlStore) LastConsumed(n int) ([]Record, error) {
	if n <= 0 {
		n = 4
	}
	q := s.d.Q(`SELECT id, to_agent, from_agent, team, action, rank, is_order, thread, reply_to, body, sent_at, consumed_at, listened_at
		FROM messages ORDER BY consumed_at DESC LIMIT ?`)
	rows, err := s.db.Query(q, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *sqlStore) Log(q LogQuery) ([]Record, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	var args []any
	var where []string
	if q.All {
		// no filter
	} else if q.Team != "" {
		where = append(where, "team=?")
		args = append(args, q.Team)
	} else {
		parts := []string{"to_agent=?", "from_agent=?"}
		args = append(args, q.As, q.As)
		if len(q.Teams) > 0 {
			ph := make([]string, len(q.Teams))
			for i, t := range q.Teams {
				ph[i] = "?"
				args = append(args, t)
			}
			parts = append(parts, "team IN ("+strings.Join(ph, ",")+")")
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}
	sqlq := `SELECT id, to_agent, from_agent, team, action, rank, is_order, thread, reply_to, body, sent_at, consumed_at, listened_at FROM messages`
	if len(where) > 0 {
		sqlq += " WHERE " + strings.Join(where, " AND ")
	}
	sqlq += " ORDER BY consumed_at DESC LIMIT ?"
	args = append(args, q.Limit)
	rows, err := s.db.Query(s.d.Q(sqlq), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *sqlStore) scanOne(row *sql.Row) (*Record, error) {
	r, err := scanRecord(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *sqlStore) scanRows(rows *sql.Rows) ([]Record, error) {
	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(sc scanner) (Record, error) {
	var r Record
	var order any
	var listened sql.NullString
	err := sc.Scan(&r.ID, &r.ToAgent, &r.From, &r.Team, &r.Action, &r.Rank, &order, &r.Thread, &r.ReplyTo, &r.Body, &r.At, &r.ConsumedAt, &listened)
	if err != nil {
		return r, err
	}
	r.To = r.ToAgent
	r.Order = asBool(order)
	r.ListenedAt = scanNull(listened)
	return r, nil
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int64:
		return x != 0
	case int:
		return x != 0
	case []byte:
		s := string(x)
		return s == "t" || s == "true" || s == "1"
	case string:
		return x == "t" || x == "true" || x == "1"
	default:
		return false
	}
}

func (s *sqlStore) GetRules() (*Doc, error) {
	q := s.d.Q(`SELECT version, body, updated_by, updated_at FROM rules ORDER BY version DESC LIMIT 1`)
	var d Doc
	err := s.db.QueryRow(q).Scan(&d.Version, &d.Body, &d.UpdatedBy, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &d, err
}

func (s *sqlStore) SeedRules(body string) error {
	cur, err := s.GetRules()
	if err != nil || cur != nil {
		return err
	}
	q := s.d.Q(`INSERT INTO rules(version, body, updated_by, updated_at) VALUES (1, ?, 'fleeting', ?)`)
	_, err = s.db.Exec(q, body, stamp())
	return err
}

func (s *sqlStore) AppendRules(by, body string) error {
	body, err := checkBody(body)
	if err != nil {
		return err
	}
	var next int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version),0)+1 FROM rules`).Scan(&next); err != nil {
		return err
	}
	q := s.d.Q(`INSERT INTO rules(version, body, updated_by, updated_at) VALUES (?, ?, ?, ?)`)
	_, err = s.db.Exec(q, next, body, by, stamp())
	return err
}

func (s *sqlStore) RulesLog() ([]Doc, error) {
	rows, err := s.db.Query(`SELECT version, body, updated_by, updated_at FROM rules ORDER BY version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Doc
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.Version, &d.Body, &d.UpdatedBy, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *sqlStore) GetOnboarding(agent string) (*Doc, error) {
	q := s.d.Q(`SELECT agent, version, body, updated_by, updated_at FROM onboarding WHERE agent=? ORDER BY version DESC LIMIT 1`)
	var d Doc
	err := s.db.QueryRow(q, agent).Scan(&d.Agent, &d.Version, &d.Body, &d.UpdatedBy, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &d, err
}

func (s *sqlStore) SeedOnboarding(agent, body string) error {
	cur, err := s.GetOnboarding(agent)
	if err != nil || cur != nil {
		return err
	}
	q := s.d.Q(`INSERT INTO onboarding(agent, version, body, updated_by, updated_at) VALUES (?, 1, ?, 'fleeting', ?)`)
	_, err = s.db.Exec(q, agent, body, stamp())
	return err
}

func (s *sqlStore) AppendOnboarding(agent, by, body string) error {
	body, err := checkBody(body)
	if err != nil {
		return err
	}
	var next int
	qmax := s.d.Q(`SELECT COALESCE(MAX(version),0)+1 FROM onboarding WHERE agent=?`)
	if err := s.db.QueryRow(qmax, agent).Scan(&next); err != nil {
		return err
	}
	q := s.d.Q(`INSERT INTO onboarding(agent, version, body, updated_by, updated_at) VALUES (?, ?, ?, ?, ?)`)
	_, err = s.db.Exec(q, agent, next, body, by, stamp())
	return err
}

func (s *sqlStore) GetMemory(scope, key string) (*Memory, error) {
	q := s.d.Q(`SELECT scope, key, version, body, updated_by, updated_at FROM memories WHERE scope=? AND key=? ORDER BY version DESC LIMIT 1`)
	var m Memory
	err := s.db.QueryRow(q, scope, key).Scan(&m.Scope, &m.Key, &m.Version, &m.Body, &m.UpdatedBy, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (s *sqlStore) ListMemory(scope string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 20
	}
	q := s.d.Q(`SELECT scope, key, version, body, updated_by, updated_at FROM memories m
		WHERE scope=? AND version=(SELECT MAX(version) FROM memories m2 WHERE m2.scope=m.scope AND m2.key=m.key)
		ORDER BY updated_at DESC LIMIT ?`)
	rows, err := s.db.Query(q, scope, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.Scope, &m.Key, &m.Version, &m.Body, &m.UpdatedBy, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *sqlStore) CountMemory(scope string) (int, error) {
	q := s.d.Q(`SELECT COUNT(DISTINCT key) FROM memories WHERE scope=?`)
	var n int
	err := s.db.QueryRow(q, scope).Scan(&n)
	return n, err
}

func (s *sqlStore) AppendMemory(scope, key, by, body string) (int, error) {
	body, err := checkBody(body)
	if err != nil {
		return 0, err
	}
	if !ValidMemoryKey(key) {
		return 0, fmt.Errorf("invalid memory key %q", key)
	}
	if _, _, err := ParseScope(scope); err != nil {
		return 0, err
	}
	var next int
	qmax := s.d.Q(`SELECT COALESCE(MAX(version),0)+1 FROM memories WHERE scope=? AND key=?`)
	if err := s.db.QueryRow(qmax, scope, key).Scan(&next); err != nil {
		return 0, err
	}
	q := s.d.Q(`INSERT INTO memories(scope, key, version, body, updated_by, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	_, err = s.db.Exec(q, scope, key, next, body, by, stamp())
	return next, err
}

func (s *sqlStore) MemoryLog(scope, key string) ([]Memory, error) {
	q := s.d.Q(`SELECT scope, key, version, body, updated_by, updated_at FROM memories WHERE scope=? AND key=? ORDER BY version DESC`)
	rows, err := s.db.Query(q, scope, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.Scope, &m.Key, &m.Version, &m.Body, &m.UpdatedBy, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}
