package poker

import "jokerarena-game-ms/internal/models"

// Dealer deals cards from a 52-card deck and recycles played/discarded cards once the draw
// pile runs out, so matches of many hands never exhaust the deck. It is a pure type with no
// I/O; it is not safe for concurrent use (the room actor owns it). Deterministic per seed.
type Dealer struct {
	draw       []models.Card // draw pile (front = next card)
	discards   []models.Card // played/discarded cards waiting to be reshuffled
	seed       uint64
	reshuffles uint64
}

// NewDealer returns a dealer whose first draws match NewShuffledDeck(seed).
func NewDealer(seed uint64) *Dealer {
	return &Dealer{draw: NewShuffledDeck(seed), seed: seed}
}

// Draw returns n cards. The remaining draw pile is consumed first; only when it is empty
// are the discards reshuffled into a new draw pile. Call Draw BEFORE Discard of the cards
// removed in the same action so they cannot reappear immediately.
func (d *Dealer) Draw(n int) []models.Card {
	if n <= 0 {
		return []models.Card{}
	}
	out := make([]models.Card, 0, n)
	for len(out) < n {
		if len(d.draw) == 0 {
			if len(d.discards) == 0 {
				break // invariant violated: return fewer cards
			}
			d.reshuffles++
			Shuffle(d.discards, d.seed^(d.reshuffles*0x9E3779B97F4A7C15))
			d.draw, d.discards = d.discards, nil
		}
		take := min(n-len(out), len(d.draw))
		out = append(out, d.draw[:take]...)
		d.draw = d.draw[take:]
	}
	return out
}

// Discard moves cards to the discard pile. Nil or empty is a no-op.
func (d *Dealer) Discard(cs []models.Card) {
	if len(cs) == 0 {
		return
	}
	d.discards = append(d.discards, cs...)
}

// DeckCount returns the number of cards left in the draw pile.
func (d *Dealer) DeckCount() int { return len(d.draw) }

// Total returns the cards held by the dealer (draw pile plus discards).
func (d *Dealer) Total() int { return len(d.draw) + len(d.discards) }

// Reshuffles returns how many times the discards were recycled.
func (d *Dealer) Reshuffles() uint64 { return d.reshuffles }
