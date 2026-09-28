package bus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSQLite(t *testing.T) {
	runStoreTests(t, sqliteStore(t))
}

func TestStorePostgres(t *testing.T) {
	u := os.Getenv("FLEETING_DB")
	if !strings.HasPrefix(strings.ToLower(u), "postgres") {
		t.Skip("FLEETING_DB is not a postgres URL")
	}
	st, err := Open(u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	runStoreTests(t, st)
}

func sqliteStore(t *testing.T) Store {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bus.db")
	st, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func runStoreTests(t *testing.T, st Store) {
	t.Helper()
	w := testWorld()

	r := Record{Envelope: Envelope{ID: "aaaaaaaaaaaaaaaa", Action: "fyi", From: "risa", To: "nova", Body: "hi", At: Now(), Rank: 50}, ToAgent: "nova"}
	ok, err := st.InsertMessage(r)
	if err != nil || !ok {
		t.Fatalf("insert1 %v %v", ok, err)
	}
	ok, err = st.InsertMessage(r)
	if err != nil || ok {
		t.Fatalf("dup should already-consumed, inserted=%v err=%v", ok, err)
	}

	unread, err := st.Unread("nova")
	if err != nil || unread == nil || unread.ID != r.ID {
		t.Fatalf("unread %v %+v", err, unread)
	}
	if err := st.MarkListened(r.ID, "nova"); err != nil {
		t.Fatal(err)
	}
	unread, err = st.Unread("nova")
	if err != nil || unread != nil {
		t.Fatalf("second listen %v %+v", err, unread)
	}

	if err := st.SeedRules(AgentRules); err != nil {
		t.Fatal(err)
	}
	if err := st.SeedRules("should not rewrite"); err != nil {
		t.Fatal(err)
	}
	rules, err := st.GetRules()
	if err != nil || rules == nil || rules.Body != AgentRules || rules.Version != 1 {
		t.Fatalf("seed rules %+v %v", rules, err)
	}
	if !CanAppendRules(40) || CanAppendRules(10) {
		t.Fatal("rules rank")
	}
	if err := st.AppendRules("risa", "new constraint"); err != nil {
		t.Fatal(err)
	}
	rules, _ = st.GetRules()
	if rules.Version != 2 || rules.Body != "new constraint" {
		t.Fatalf("append rules %+v", rules)
	}

	if err := st.SeedOnboarding("nova", SeedCard("nova", "worker", 10, []string{"risa"}, []string{"core"})); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendOnboarding("nova", "nova", "v2 card"); err != nil {
		t.Fatal(err)
	}
	if err := st.SeedOnboarding("nova", "clobber"); err != nil {
		t.Fatal(err)
	}
	ob, _ := st.GetOnboarding("nova")
	if ob == nil || ob.Version != 2 || ob.Body != "v2 card" {
		t.Fatalf("onboard %+v", ob)
	}
	if err := CanAppendOnboarding(w, "nova", "risa"); err == nil {
		t.Fatal("worker cannot append risa card")
	}
	if err := CanAppendOnboarding(w, "risa", "nova"); err != nil {
		t.Fatal(err)
	}

	if err := CanWriteMemory(w, "nova", "team:review"); err == nil {
		t.Fatal("nova not in review")
	}
	if _, err := st.AppendMemory("team:review", "border", "hiro", "v1"); err != nil {
		t.Fatal(err)
	}
	n, err := st.AppendMemory("team:review", "border", "hiro", "v2")
	if err != nil || n != 2 {
		t.Fatalf("mem v2 %d %v", n, err)
	}
	log, err := st.MemoryLog("team:review", "border")
	if err != nil || len(log) != 2 || log[0].Version != 2 || log[1].Version != 1 {
		t.Fatalf("mem log %+v %v", log, err)
	}

	_, _ = st.InsertMessage(Record{Envelope: Envelope{ID: "bbbbbbbbbbbbbbbb", Action: "fyi", From: "hiro", To: "veil", Team: "review", Body: "rev", At: Now()}, ToAgent: "veil"})
	rows, err := st.Log(LogQuery{As: "nova", Teams: w.TeamsOf("nova"), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Team == "review" {
			t.Fatal("nova must not see team:review mail")
		}
	}
	if CanLogAll(10) {
		t.Fatal("nova cannot --all")
	}
	if !CanLogAll(50) {
		t.Fatal("risa can --all")
	}
	all, err := st.Log(LogQuery{All: true, Limit: 20})
	if err != nil || len(all) < 2 {
		t.Fatalf("all %d %v", len(all), err)
	}

	if _, err := st.AppendMemory("fleet", "goal", "nova", "land it"); err != nil {
		t.Fatal(err)
	}
	if err := CanReadMemory(w, "nova", "team:review"); err == nil {
		t.Fatal("nova cannot read review memories")
	}
	fleet, _ := st.ListMemory("fleet", 20)
	review, _ := st.ListMemory("team:review", 20)
	card := FormatCard(ob, rules, fleet, len(fleet), map[string][]Memory{"core": nil}, map[string]int{"core": 0})
	if !strings.Contains(card, "new constraint") || !strings.Contains(card, "land it") {
		t.Fatalf("card missing rules/memory: %s", card)
	}
	if strings.Contains(card, "team:review") {
		t.Fatal("card should omit other team memory")
	}
	_ = review
}
