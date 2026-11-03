package events

import (
	"encoding/json"
	"regexp"
	"testing"
)

func TestBuildEnvelopeFollowsTheContract(t *testing.T) {
	body, err := BuildEnvelope("game.hand.scored", map[string]any{"score": 1248})
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if env.Type != "game.hand.scored" || env.Version != 1 || env.Producer != "game-engine-ms" || env.OccurredAt == "" {
		t.Fatalf("bad envelope: %+v", env)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(env.EventID) {
		t.Fatalf("event_id is not a UUID v4: %s", env.EventID)
	}
	if string(env.Payload) != `{"score":1248}` {
		t.Fatalf("payload: %s", env.Payload)
	}
}

func TestPublishNeverBlocksWhenTheQueueIsFull(t *testing.T) {
	b := NewBus("amqp://unused", func([]byte) error { return nil })
	for i := 0; i < cap(b.out)+10; i++ {
		b.Publish("game.hand.scored", i)
	}
	if b.dropped.Load() != 10 {
		t.Fatalf("expected 10 dropped events, got %d", b.dropped.Load())
	}
}
