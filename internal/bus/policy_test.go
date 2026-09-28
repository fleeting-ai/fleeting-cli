package bus

import (
	"strings"
	"testing"

	"github.com/fleeting-ai/fleeting-cli/internal/config"
)

func testWorld() World {
	cfg := &config.File{
		Teams: []config.Team{
			{ID: "core", Members: []string{"risa", "hiro", "nova"}},
			{ID: "review", Members: []string{"hiro", "veil"}},
		},
		Fleets: []config.Fleet{{ID: "local-core", Agents: []config.Agent{
			{Name: "nova", Role: "worker", Rank: 10, Peers: []string{"risa"}},
			{Name: "bolt", Role: "worker", Rank: 10, Peers: []string{"risa"}},
			{Name: "veil", Role: "worker", Rank: 10, Peers: []string{"hiro", "risa"}},
			{Name: "hiro", Role: "judge", Rank: 40, Peers: []string{"risa", "nova", "bolt", "kite", "veil"}},
			{Name: "risa", Role: "foreman", Rank: 50, Peers: []string{"hiro", "nova", "bolt", "kite", "veil"}},
		}}},
	}
	_ = cfg.Validate()
	return WorldFrom(cfg, nil)
}

func TestDirectDenyUnknownSelf(t *testing.T) {
	w := testWorld()
	if _, err := w.Allow(Send{From: "nova", To: "bolt", Action: "fyi", Body: "hi"}); err == nil || !strings.Contains(err.Error(), "default deny") {
		t.Fatalf("peer deny: %v", err)
	}
	if _, err := w.Allow(Send{From: "nova", To: "zzzz", Action: "fyi", Body: "hi"}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := w.Allow(Send{From: "nova", To: "nova", Action: "fyi", Body: "hi"}); err == nil || !strings.Contains(err.Error(), "self") {
		t.Fatalf("self: %v", err)
	}
}

func TestTeamMembershipAndExcludeSender(t *testing.T) {
	w := testWorld()
	if _, err := w.Allow(Send{From: "nova", Team: "review", Action: "fyi", Body: "hi"}); err == nil || !strings.Contains(err.Error(), "not a member") {
		t.Fatalf("non-member: %v", err)
	}
	got, err := w.Allow(Send{From: "risa", Team: "core", Action: "fyi", Body: "standup"})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range got {
		names[e.To] = true
		if e.To == "risa" {
			t.Fatal("sender should not receive")
		}
		if e.Team != "core" {
			t.Fatalf("team %q", e.Team)
		}
	}
	if !names["nova"] || !names["hiro"] || names["risa"] {
		t.Fatalf("recips %v", names)
	}
}

func TestOrderRejectedEqualOrHigher(t *testing.T) {
	w := testWorld()
	pub := &fakePub{}
	if _, err := w.Allow(Send{From: "hiro", Team: "core", Action: "order", Body: "no"}); err == nil {
		t.Fatal("hiro cannot order risa")
	}
	if pub.n != 0 {
		t.Fatalf("rejected send must not publish, got %d", pub.n)
	}
}

func TestTodoOrderStamp(t *testing.T) {
	w := testWorld()
	got, err := w.Allow(Send{From: "risa", To: "nova", Action: "todo", Body: "fix"})
	if err != nil || len(got) != 1 || !got[0].Order {
		t.Fatalf("50→10 todo should order: %v %+v", err, got)
	}
	got, err = w.Allow(Send{From: "risa", Team: "core", Action: "todo", Body: "fix"})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]bool{}
	for _, e := range got {
		by[e.To] = e.Order
	}
	if !by["nova"] {
		t.Fatal("worker should be ordered")
	}
	w.Agents["peer"] = Agent{Name: "peer", Rank: 50, Peers: []string{"risa"}}
	w.Teams["core"] = []string{"risa", "nova", "peer"}
	got, err = w.Allow(Send{From: "risa", Team: "core", Action: "todo", Body: "fix"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.To == "peer" && e.Order {
			t.Fatal("equal rank is not an order")
		}
		if e.To == "nova" && !e.Order {
			t.Fatal("worker todo is an order")
		}
	}
	got, err = w.Allow(Send{From: "nova", To: "risa", Action: "fyi", Body: "hi"})
	if err != nil || got[0].Order {
		t.Fatalf("fyi never orders: %v %+v", err, got)
	}
}

func TestAckRules(t *testing.T) {
	w := testWorld()
	if _, err := w.Allow(Send{From: "nova", To: "risa", Action: "ack", Body: "done"}); err == nil {
		t.Fatal("ack without reply_to")
	}
	if _, err := w.Allow(Send{From: "risa", Team: "core", Action: "ack", Body: "done", ReplyTo: "abc"}); err == nil {
		t.Fatal("ack to team")
	}
	got, err := w.Allow(Send{From: "nova", To: "risa", Action: "ack", Body: "done", ReplyTo: "deadbeefdeadbeef"})
	if err != nil || got[0].Order {
		t.Fatalf("ack: %v %+v", err, got)
	}
}

func TestBodyControlAndNewline(t *testing.T) {
	w := testWorld()
	if _, err := w.Allow(Send{From: "nova", To: "risa", Body: "   "}); err == nil {
		t.Fatal("empty")
	}
	if _, err := w.Allow(Send{From: "nova", To: "risa", Body: "x\x01y"}); err == nil {
		t.Fatal("control")
	}
	got, err := w.Allow(Send{From: "nova", To: "risa", Body: "line1\nline2"})
	if err != nil || !strings.Contains(got[0].Body, "\n") {
		t.Fatalf("newline: %v %+v", err, got)
	}
}

func TestRankDefaultInWorld(t *testing.T) {
	cfg := &config.File{Fleets: []config.Fleet{{ID: "f", Agents: []config.Agent{
		{Name: "risa", Role: "foreman", Harness: "omp"},
		{Name: "nova", Role: "worker", Harness: "omp", Rank: 7},
	}}}}
	_ = cfg.Validate()
	w := WorldFrom(cfg, nil)
	if w.Agents["risa"].Rank != 50 {
		t.Fatalf("foreman %d", w.Agents["risa"].Rank)
	}
	if w.Agents["nova"].Rank != 7 {
		t.Fatalf("explicit %d", w.Agents["nova"].Rank)
	}
}

func TestFrameText(t *testing.T) {
	e := Envelope{ID: "0123abcdffffeeee", Action: "todo", From: "risa", To: "nova", Rank: 50, Order: true, Body: "fix the border"}
	got := e.Frame()
	if got != "[fleeting todo risa→nova #0123abcd order rank=50] fix the border" {
		t.Fatalf("got %q", got)
	}
	e = Envelope{ID: "0123abcdffffeeee", Action: "fyi", From: "risa", To: "nova", Team: "core", Body: "standup"}
	if e.Frame() != "[fleeting fyi team:core risa→nova #0123abcd] standup" {
		t.Fatalf("got %q", e.Frame())
	}
	e = Envelope{ID: "0123abcdffffeeee", Action: "ack", From: "nova", To: "risa", ReplyTo: "abcdef00deadbeef", Body: "done"}
	if e.Frame() != "[fleeting ack nova→risa #0123abcd reply=abcdef00] done" && e.Frame() != "[fleeting ack nova→risa #0123abcd reply=abcdef] done" {
		// spec example uses reply=abcdef (8 chars of reply_to)
		if !strings.Contains(e.Frame(), "reply=") || strings.Contains(e.Frame(), "order") {
			t.Fatalf("ack frame %q", e.Frame())
		}
	}
}

type fakePub struct{ n int }

func (f *fakePub) Publish() { f.n++ }
