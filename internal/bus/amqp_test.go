package bus

import (
	"context"
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestAMQPIntegration(t *testing.T) {
	url := os.Getenv("FLEETING_AMQP")
	if url == "" {
		t.Skip("FLEETING_AMQP not set")
	}
	b := NewBroker(url)
	if err := b.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	for _, n := range []string{"risa", "nova", "hiro"} {
		if err := b.DeclareAgent(n); err != nil {
			t.Fatal(err)
		}
		purge(t, b, AgentQueue(n))
	}
	if err := b.DeclareTeam("core"); err != nil {
		t.Fatal(err)
	}
	purge(t, b, TeamQueue("core"))

	env := Envelope{ID: NewID(), Action: "fyi", From: "risa", Team: "core", Rank: 50, Body: "standup", At: Now()}
	if err := b.PublishTeam("core", env); err != nil {
		t.Fatal(err)
	}
	raw := getOne(t, b, TeamQueue("core"), false)
	var got Envelope
	if err := json.Unmarshal(raw.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != env.ID {
		t.Fatalf("team payload %s", got.ID)
	}
	_ = raw.Nack(false, true)

	ranks := map[string]int{"risa": 50, "nova": 10, "hiro": 40}
	go func() {
		b.StartTeamFanout(context.Background(), "core", []string{"risa", "nova", "hiro"}, ranks)
	}()
	time.Sleep(400 * time.Millisecond)
	novaD := getOne(t, b, AgentQueue("nova"), true)
	hiroD := getOne(t, b, AgentQueue("hiro"), true)
	if qlen(t, b, AgentQueue("risa")) != 0 {
		t.Fatal("sender must not get a copy")
	}
	_ = novaD
	_ = hiroD

	st := sqliteStore(t)
	var alive atomic.Bool
	b.SetHooks(st, func(string) bool { return alive.Load() }, func(string, string) error { return nil })
	purge(t, b, AgentQueue("nova"))
	b.StartAgentConsumer("nova")
	msg := Envelope{ID: NewID(), Action: "fyi", From: "risa", To: "nova", Body: "gate", At: Now()}
	if err := b.PublishAgent(msg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	u, _ := st.Unread("nova")
	if u != nil {
		t.Fatal("stopped consumer must not insert")
	}
	if qlen(t, b, AgentQueue("nova")) == 0 {
		t.Fatal("message should wait on the agent queue")
	}
	alive.Store(true)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		u, _ = st.Unread("nova")
		if u != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if u == nil || u.ID != msg.ID {
		t.Fatalf("after start %+v", u)
	}

	fail := &failStore{Store: sqliteStore(t)}
	b.SetHooks(fail, func(string) bool { return true }, func(string, string) error { return nil })
	purge(t, b, AgentQueue("hiro"))
	b.StartAgentConsumer("hiro")
	closed := Envelope{ID: NewID(), Action: "fyi", From: "risa", To: "hiro", Body: "store-down", At: Now()}
	if err := b.PublishAgent(closed); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	fail.ok.Store(true)
	deadline = time.Now().Add(3 * time.Second)
	var rec *Record
	for time.Now().Before(deadline) {
		rec, _ = fail.Unread("hiro")
		if rec != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if rec == nil {
		t.Fatal("later open should insert once")
	}
	ok, err := fail.InsertMessage(Record{Envelope: closed, ToAgent: "hiro"})
	if err != nil || ok {
		t.Fatalf("second insert inserted=%v err=%v", ok, err)
	}
}

func purge(t *testing.T, b *Broker, q string) {
	t.Helper()
	ch, err := b.conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	_, _ = ch.QueuePurge(q, false)
}

func qlen(t *testing.T, b *Broker, q string) int {
	t.Helper()
	ch, err := b.conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	qi, err := ch.QueueInspect(q)
	if err != nil {
		t.Fatal(err)
	}
	return qi.Messages
}

func getOne(t *testing.T, b *Broker, q string, ack bool) amqp.Delivery {
	t.Helper()
	ch, err := b.conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		d, ok, err := ch.Get(q, false)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			if ack {
				_ = d.Ack(false)
			}
			return d
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no message on %s", q)
	return amqp.Delivery{}
}

type failStore struct {
	Store
	ok atomic.Bool
}

func (f *failStore) InsertMessage(r Record) (bool, error) {
	if !f.ok.Load() {
		return false, os.ErrClosed
	}
	return f.Store.InsertMessage(r)
}
