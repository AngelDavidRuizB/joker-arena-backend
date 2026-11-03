// Package events connects the engine to RabbitMQ (exchange jokerarena.events).
package events

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// Exchange is the shared topic exchange of the system.
	Exchange = "jokerarena.events"
	// RulesetQueue is the engine's queue for LiveOps updates.
	RulesetQueue = "game-engine-ms.ruleset"
	// RulesetBinding receives every liveops.ruleset.* event.
	RulesetBinding = "liveops.ruleset.#"
	producer       = "game-engine-ms"
)

// Envelope is the common event wrapper (contracts/events.md).
type Envelope struct {
	EventID    string          `json:"event_id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt string          `json:"occurred_at"`
	Producer   string          `json:"producer"`
	Payload    json.RawMessage `json:"payload"`
}

// NewEventID returns a random UUID v4.
func NewEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// BuildEnvelope wraps a payload in the common envelope.
func BuildEnvelope(routingKey string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{
		EventID:    NewEventID(),
		Type:       routingKey,
		Version:    1,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Producer:   producer,
		Payload:    raw,
	})
}

type outMsg struct {
	key  string
	body []byte
}

// Bus publishes events asynchronously (a bounded queue drained by one goroutine) and
// consumes LiveOps updates. It reconnects on its own; while the broker is down the engine
// keeps playing and events are buffered up to the queue size, then dropped with a log line.
type Bus struct {
	url       string
	onRuleset func([]byte) error
	out       chan outMsg
	connected atomic.Bool
	dropped   atomic.Int64

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewBus creates a bus. onRuleset is called for every liveops.ruleset.* message.
func NewBus(url string, onRuleset func([]byte) error) *Bus {
	return &Bus{url: url, onRuleset: onRuleset, out: make(chan outMsg, 1024)}
}

// Connected reports whether the broker connection is up (for /health).
func (b *Bus) Connected() bool { return b.connected.Load() }

// Publish enqueues an event; it never blocks the caller.
func (b *Bus) Publish(routingKey string, payload any) {
	body, err := BuildEnvelope(routingKey, payload)
	if err != nil {
		log.Printf("[ERROR] event %s not serializable: %v", routingKey, err)
		return
	}
	select {
	case b.out <- outMsg{key: routingKey, body: body}:
	default:
		n := b.dropped.Add(1)
		log.Printf("[WARN] event queue full, dropped %s (total dropped: %d)", routingKey, n)
	}
}

// Run connects, consumes and publishes until ctx is cancelled.
func (b *Bus) Run(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		err := b.session(ctx)
		b.connected.Store(false)
		if ctx.Err() != nil {
			return
		}
		log.Printf("[WARN] RabbitMQ session ended (%v); reconnecting in %s", err, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 15*time.Second {
			delay *= 2
		}
	}
}

func (b *Bus) session(ctx context.Context) error {
	conn, err := amqp.Dial(b.url)
	if err != nil {
		return err
	}
	defer conn.Close()
	pubCh, err := conn.Channel()
	if err != nil {
		return err
	}
	if err := pubCh.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := pubCh.Confirm(false); err != nil {
		return err
	}
	confirms := pubCh.NotifyPublish(make(chan amqp.Confirmation, 1))

	subCh, err := conn.Channel()
	if err != nil {
		return err
	}
	if _, err := subCh.QueueDeclare(RulesetQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := subCh.QueueBind(RulesetQueue, RulesetBinding, Exchange, false, nil); err != nil {
		return err
	}
	deliveries, err := subCh.Consume(RulesetQueue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	b.connected.Store(true)
	log.Printf("[INFO] RabbitMQ connected: publishing to %s, consuming %s", Exchange, RulesetQueue)
	closed := conn.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-closed:
			return fmt.Errorf("connection closed: %v", e)
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("ruleset consumer cancelled")
			}
			b.handleRuleset(d)
		case m := <-b.out:
			if err := b.publishOne(ctx, pubCh, confirms, m); err != nil {
				// Put it back (best effort) so it is retried after reconnecting.
				select {
				case b.out <- m:
				default:
				}
				return err
			}
		}
	}
}

func (b *Bus) publishOne(ctx context.Context, ch *amqp.Channel, confirms chan amqp.Confirmation, m outMsg) error {
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := ch.PublishWithContext(pctx, Exchange, m.key, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         m.body,
	})
	if err != nil {
		return err
	}
	select {
	case c := <-confirms:
		if !c.Ack {
			return fmt.Errorf("broker nacked %s", m.key)
		}
		return nil
	case <-pctx.Done():
		return fmt.Errorf("publish confirm timeout for %s", m.key)
	}
}

func (b *Bus) handleRuleset(d amqp.Delivery) {
	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil || len(env.Payload) == 0 {
		log.Printf("[ERROR] invalid liveops message discarded: %v", err)
		_ = d.Nack(false, false)
		return
	}
	if err := b.onRuleset(env.Payload); err != nil {
		log.Printf("[WARN] ruleset event not applied: %v", err)
	} else {
		log.Printf("[INFO] ruleset hot-reloaded from event %s", env.EventID)
	}
	_ = d.Ack(false)
}

// Noop is a publisher used when RABBITMQ_URL is not configured (standalone mode).
type Noop struct{}

// Publish does nothing.
func (Noop) Publish(string, any) {}
