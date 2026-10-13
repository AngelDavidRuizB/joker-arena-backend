package poker

import (
	"testing"

	"jokerarena-game-ms/internal/models"
)

func TestBestPlayPicksHighestScore(t *testing.T) {
	// Eight cards containing a four-of-a-kind of sevens plus junk. The best subset is the
	// four sevens (strongest hand available), not any larger-but-weaker selection.
	cards := hand(t, "7S 7H 7C 7D 2S 3H 4C 9D")
	idx, b := BestPlay(cards, 0)
	if b == nil || b.PokerHand != FourOfAKind {
		t.Fatalf("want FOUR_OF_A_KIND, got %+v", b)
	}
	// Verify no other subset scores higher.
	best := b.Score
	limit := MaxPlayCards
	for size := 1; size <= limit; size++ {
		forEachSubset(len(cards), size, func(sub []int) {
			sel := make([]models.Card, len(sub))
			for i, j := range sub {
				sel[i] = cards[j]
			}
			s, err := Score(sel, 0)
			if err == nil && s.Score > best {
				t.Fatalf("subset %v scores %d > reported best %d", sub, s.Score, best)
			}
		})
	}
	if len(idx) != 4 {
		t.Fatalf("expected 4 indices for the four of a kind, got %v", idx)
	}
}

func TestBestPlayLexicographicTieBreak(t *testing.T) {
	// Two identical high cards (aces) of different suits produce equal single-card scores;
	// the deterministic tie-break keeps the lowest index set.
	cards := hand(t, "AS AH")
	idx, b := BestPlay(cards, 0)
	if b == nil || b.PokerHand != Pair {
		t.Fatalf("want PAIR for two aces, got %+v", b)
	}
	if len(idx) != 2 || idx[0] != 0 || idx[1] != 1 {
		t.Fatalf("tie-break indices %v, want [0 1]", idx)
	}

	// Determinism: repeated calls return identical results.
	idx2, b2 := BestPlay(cards, 0)
	if b2.Score != b.Score || len(idx2) != len(idx) {
		t.Fatalf("non-deterministic BestPlay: %v/%d vs %v/%d", idx, b.Score, idx2, b2.Score)
	}
}

func TestBotDecisionDiscardsJunkWhenDiscardsLeft(t *testing.T) {
	// Best hand is a Pair of nines (weaker than THREE_OF_A_KIND) with discards available:
	// the bot discards instead of playing.
	cards := hand(t, "9S 9H 2C 3D 4H 5S 7C 8D")
	action, idx := BotDecision(cards, 2)
	if action != "DISCARD" {
		t.Fatalf("want DISCARD for junk with discards left, got %s", action)
	}
	if len(idx) < 1 || len(idx) > MaxPlayCards {
		t.Fatalf("discard count %d out of range: %v", len(idx), idx)
	}
	// The discarded indices must exclude the largest rank group (the two nines at 0,1).
	for _, i := range idx {
		if i == 0 || i == 1 {
			t.Fatalf("discard %v must not include the nines (indices 0,1)", idx)
		}
	}
}

func TestBotDecisionPlaysStrongHand(t *testing.T) {
	// Three of a kind is strong enough to PLAY even with discards available.
	cards := hand(t, "QS QH QC 2D 3H 4C 5S 7D")
	action, idx := BotDecision(cards, 3)
	if action != "PLAY" {
		t.Fatalf("want PLAY for THREE_OF_A_KIND, got %s", action)
	}
	if len(idx) != 3 {
		t.Fatalf("want the three queens, got %v", idx)
	}
}

func TestBotDecisionPlaysWhenNoDiscardsLeft(t *testing.T) {
	// Even a weak hand must be PLAYed once no discards remain.
	cards := hand(t, "2S 3H 4C 5S 7D 9H JC KD")
	action, idx := BotDecision(cards, 0)
	if action != "PLAY" {
		t.Fatalf("want PLAY when discardsLeft==0, got %s", action)
	}
	if len(idx) < 1 {
		t.Fatalf("expected a non-empty play, got %v", idx)
	}
}

func TestBotDecisionIsDeterministic(t *testing.T) {
	cards := hand(t, "9S 9H 2C 3D 4H 5S 7C 8D")
	a1, i1 := BotDecision(cards, 2)
	a2, i2 := BotDecision(cards, 2)
	if a1 != a2 || len(i1) != len(i2) {
		t.Fatalf("non-deterministic: %s/%v vs %s/%v", a1, i1, a2, i2)
	}
	for k := range i1 {
		if i1[k] != i2[k] {
			t.Fatalf("non-deterministic indices: %v vs %v", i1, i2)
		}
	}
}

// forEachSubset invokes fn for every ascending-index subset of the given size.
func forEachSubset(n, size int, fn func([]int)) {
	var rec func(start int, cur []int)
	rec = func(start int, cur []int) {
		if len(cur) == size {
			fn(cur)
			return
		}
		for i := start; i < n; i++ {
			rec(i+1, append(cur, i))
		}
	}
	rec(0, nil)
}
