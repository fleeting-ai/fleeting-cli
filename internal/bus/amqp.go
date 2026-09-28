package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExAgents = "fleeting.agents"
	ExTeams  = "fleeting.teams"
)

func AgentQueue(name string) string { return "fleeting.agent." + name }
func TeamQueue(id string) string    { return "fleeting.team." + id }

type ConsumeFunc func(env Envelope) error

type Broker struct {
	URL string

	mu       sync.Mutex
	pubMu    sync.Mutex
	conn     *amqp.Connection
	pub      *amqp.Channel
	confirms <-chan amqp.Confirmation
	err      error
	alive    func(string) bool
	store    Store
	write    func(name, frame string) error
	stop     map[string]context.CancelFunc
	teamStop context.CancelFunc
}

func NewBroker(url string) *Broker {
	return &Broker{URL: url, stop: map[string]context.CancelFunc{}}
}

func (b *Broker) SetHooks(store Store, alive func(string) bool, write func(name, frame string) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.store = store
	b.alive = alive
	b.write = write
}

func (b *Broker) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func (b *Broker) Up() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conn != nil && !b.conn.IsClosed()
}

func (b *Broker) Connect() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil && !b.conn.IsClosed() {
		return nil
	}
	conn, err := amqp.Dial(b.URL)
	if err != nil {
		b.err = err
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		b.err = err
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		b.err = err
		return err
	}
	b.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 32))
	if err := ch.ExchangeDeclare(ExAgents, "direct", true, false, false, false, nil); err != nil {
		b.err = err
		return err
	}
	if err := ch.ExchangeDeclare(ExTeams, "direct", true, false, false, false, nil); err != nil {
		b.err = err
		return err
	}
	b.conn = conn
	b.pub = ch
	b.err = nil
	return nil
}

func (b *Broker) DeclareAgent(name string) error {
	return b.declare(AgentQueue(name), ExAgents, name)
}

func (b *Broker) DeclareTeam(id string) error {
	return b.declare(TeamQueue(id), ExTeams, id)
}

func (b *Broker) declare(queue, ex, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pub == nil {
		return fmt.Errorf("bus down")
	}
	if _, err := b.pub.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	return b.pub.QueueBind(queue, key, ex, false, nil)
}

func (b *Broker) PublishAgent(env Envelope) error {
	return b.publish(ExAgents, env.To, env)
}

func (b *Broker) PublishTeam(team string, env Envelope) error {
	return b.publish(ExTeams, team, env)
}

func (b *Broker) publish(ex, key string, env Envelope) error {
	b.mu.Lock()
	ch := b.pub
	conf := b.confirms
	b.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("bus down")
	}
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	b.pubMu.Lock()
	defer b.pubMu.Unlock()
	if err := ch.Publish(ex, key, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		MessageId:    env.ID,
		Body:         body,
	}); err != nil {
		return err
	}
	if conf == nil {
		return nil
	}
	select {
	case c, ok := <-conf:
		if !ok || !c.Ack {
			return fmt.Errorf("publish not confirmed")
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("publish confirm timeout")
	}
}

func (b *Broker) StartTeamFanout(ctx context.Context, team string, members []string, ranks map[string]int) {
	b.consumeLoop(ctx, TeamQueue(team), func(env Envelope, d amqp.Delivery) {
		var copies []Envelope
		for _, m := range members {
			if m == env.From {
				continue
			}
			cp := env
			cp.To = m
			cp.Team = team
			dst := ranks[m]
			switch env.Action {
			case "todo":
				cp.Order = env.Rank > dst
			case "order":
				cp.Order = true
			default:
				cp.Order = false
			}
			if err := b.PublishAgent(cp); err != nil {
				_ = d.Nack(false, true)
				return
			}
			copies = append(copies, cp)
		}
		_ = copies
		_ = d.Ack(false)
	})
}

func (b *Broker) StartAgentConsumer(name string) {
	b.mu.Lock()
	if _, ok := b.stop[name]; ok {
		b.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.stop[name] = cancel
	b.mu.Unlock()
	go b.consumeLoop(ctx, AgentQueue(name), func(env Envelope, d amqp.Delivery) {
		b.mu.Lock()
		alive := b.alive
		st := b.store
		write := b.write
		b.mu.Unlock()
		if alive != nil && !alive(name) {
			_ = d.Nack(false, true)
			time.Sleep(200 * time.Millisecond)
			return
		}
		if st == nil {
			_ = d.Nack(false, true)
			return
		}
		env.To = name
		rec := Record{Envelope: env, ToAgent: name}
		inserted, err := st.InsertMessage(rec)
		if err != nil {
			_ = d.Nack(false, true)
			return
		}
		if inserted && write != nil {
			_ = write(name, env.Frame())
		}
		_ = d.Ack(false)
	})
}

func (b *Broker) StopAgentConsumer(name string) {
	b.mu.Lock()
	cancel := b.stop[name]
	delete(b.stop, name)
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (b *Broker) consumeLoop(ctx context.Context, queue string, handle func(Envelope, amqp.Delivery)) {
	for {
		if ctx.Err() != nil {
			return
		}
		ch, deliveries, err := b.openConsume(queue)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}
		func() {
			defer ch.Close()
			for {
				select {
				case <-ctx.Done():
					return
				case d, ok := <-deliveries:
					if !ok {
						return
					}
					var env Envelope
					if err := json.Unmarshal(d.Body, &env); err != nil {
						_ = d.Nack(false, false)
						continue
					}
					handle(env, d)
				}
			}
		}()
	}
}

func (b *Broker) openConsume(queue string) (*amqp.Channel, <-chan amqp.Delivery, error) {
	b.mu.Lock()
	conn := b.conn
	b.mu.Unlock()
	if conn == nil {
		return nil, nil, fmt.Errorf("bus down")
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		_ = ch.Close()
		return nil, nil, err
	}
	d, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return nil, nil, err
	}
	return ch, d, nil
}

func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.stop {
		c()
	}
	b.stop = map[string]context.CancelFunc{}
	if b.pub != nil {
		_ = b.pub.Close()
		b.pub = nil
	}
	if b.conn != nil {
		_ = b.conn.Close()
		b.conn = nil
	}
}
