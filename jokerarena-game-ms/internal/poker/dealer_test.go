package poker

import (
	mrand "math/rand/v2"
	"testing"

	"jokerarena-game-ms/internal/models"
)

func key(c models.Card) string { return c.Rank + c.Suit }

func TestDealerFirstHandMatchesShuffledDeck(t *testing.T) {
	d := NewDealer(42)
	want := NewShuffledDeck(42)[:8]
	got := d.Draw(8)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("card %d: %v vs %v", i, got[i], want[i])
		}
	}
	if d.DeckCount() != 44 || d.Total() != 44 {
		t.Fatalf("deck=%d total=%d", d.DeckCount(), d.Total())
	}
}

func TestDealerConservesCardsOver200Cycles(t *testing.T) {
	d := NewDealer(5)
	hand := d.Draw(8)
	rng := mrand.New(mrand.NewPCG(1, 2))
	for cycle := 0; cycle < 200; cycle++ {
		n := 1 + rng.IntN(5)
		perm := rng.Perm(len(hand))[:n]
		removed := map[int]bool{}
		for _, i := range perm {
			removed[i] = true
		}
		var kept, taken []models.Card
		for i, c := range hand {
			if removed[i] {
				taken = append(taken, c)
			} else {
				kept = append(kept, c)
			}
		}
		kept = append(kept, d.Draw(8-len(kept))...)
		d.Discard(taken)
		hand = kept
		if len(hand) != 8 || len(hand)+d.Total() != 52 {
			t.Fatalf("cycle %d: hand=%d total=%d", cycle, len(hand), d.Total())
		}
		seen := map[string]bool{}
		for _, c := range hand {
			seen[key(c)] = true
		}
		for _, c := range d.draw {
			seen[key(c)] = true
		}
		for _, c := range d.discards {
			seen[key(c)] = true
		}
		if len(seen) != 52 {
			t.Fatalf("cycle %d: %d distinct cards, want 52", cycle, len(seen))
		}
	}
	if d.Reshuffles() == 0 {
		t.Fatal("expected at least one reshuffle")
	}
}

func TestDealerDeterministic(t *testing.T) {
	run := func() []models.Card {
		d := NewDealer(9)
		var out []models.Card
		for i := 0; i < 30; i++ {
			c := d.Draw(5)
			out = append(out, c...)
			d.Discard(c)
		}
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("diverged at %d", i)
		}
	}
}

func TestDealerDrawZeroAndDiscardNil(t *testing.T) {
	d := NewDealer(1)
	if got := d.Draw(0); got == nil || len(got) != 0 {
		t.Fatalf("Draw(0) = %v", got)
	}
	if got := d.Draw(-1); got == nil || len(got) != 0 {
		t.Fatalf("Draw(-1) = %v", got)
	}
	d.Discard(nil)
	d.Discard([]models.Card{})
	if d.Total() != 52 {
		t.Fatalf("total %d", d.Total())
	}
}

func TestDealerConsumesRemainingDeckBeforeReshuffle(t *testing.T) {
	d := NewDealer(3)
	_ = d.Draw(49) // 3 left in the draw pile
	d.Discard(NewDeck()[:5])
	last3 := append([]models.Card(nil), d.draw...)
	got := d.Draw(5)
	for i := 0; i < 3; i++ {
		if got[i] != last3[i] {
			t.Fatalf("card %d should be the remaining deck card %v, got %v", i, last3[i], got[i])
		}
	}
	if d.Reshuffles() != 1 || len(got) != 5 {
		t.Fatalf("reshuffles=%d len=%d", d.Reshuffles(), len(got))
	}
}

func TestDealerTakenCardsDoNotReappearInSameAction(t *testing.T) {
	d := NewDealer(8)
	hand := d.Draw(8)
	_ = d.Draw(42) // draw pile: 2 cards left; the 42 are "in transit" and discarded
	// Rebuild a consistent state: discards hold everything not in hand or draw pile.
	inHand := map[string]bool{}
	for _, c := range hand {
		inHand[key(c)] = true
	}
	inDraw := map[string]bool{}
	for _, c := range d.draw {
		inDraw[key(c)] = true
	}
	for _, c := range NewDeck() {
		if !inHand[key(c)] && !inDraw[key(c)] {
			d.discards = append(d.discards, c)
		}
	}
	kept := append([]models.Card(nil), hand[:5]...)
	taken := hand[5:]
	kept = append(kept, d.Draw(8-len(kept))...)
	d.Discard(taken)
	seen := map[string]bool{}
	for _, c := range kept {
		seen[key(c)] = true
	}
	if len(kept) != 8 || len(seen) != 8 {
		t.Fatalf("hand=%d distinct=%d", len(kept), len(seen))
	}
	for _, c := range taken {
		if seen[key(c)] {
			t.Fatalf("taken card %v reappeared in hand", c)
		}
	}
	if len(kept)+d.Total() != 52 {
		t.Fatalf("hand %d + total %d != 52", len(kept), d.Total())
	}
}
