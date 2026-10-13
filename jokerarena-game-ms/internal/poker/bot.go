package poker

import (
	"sort"

	"jokerarena-game-ms/internal/models"
)

// handTypeRank orders the hand types from weakest (0) to strongest. It mirrors the
// strength order used throughout the engine and lets the bot compare its best hand against
// a threshold (ThreeOfAKind) without string comparisons.
var handTypeRank = map[HandType]int{
	HighCard:      0,
	Pair:          1,
	TwoPair:       2,
	ThreeOfAKind:  3,
	Straight:      4,
	Flush:         5,
	FullHouse:     6,
	FourOfAKind:   7,
	StraightFlush: 8,
}

// BestPlay enumerates every subset of size 1..5 of the (up to 8) hand cards, scores each
// with Score(subset, discardsLeft) and returns the indices (into hand, ascending) of the
// highest-scoring subset together with its Breakdown. Ties are broken deterministically by
// the lexicographically smallest index set: subsets are visited in ascending-index order
// and the first maximum is kept. hand may hold fewer than 8 cards; empty hand yields nil.
func BestPlay(hand []models.Card, discardsLeft int) (indices []int, breakdown *Breakdown) {
	n := len(hand)
	if n == 0 {
		return nil, nil
	}
	limit := MaxPlayCards
	if n < limit {
		limit = n
	}

	var best *Breakdown
	var bestIdx []int

	// combos appends to bestIdx/best the lexicographically-first subset reaching the max.
	var choose func(start, size int, cur []int)
	consider := func(cur []int) {
		subset := make([]models.Card, len(cur))
		for i, idx := range cur {
			subset[i] = hand[idx]
		}
		b, err := Score(subset, discardsLeft)
		if err != nil {
			return
		}
		if best == nil || b.Score > best.Score {
			best = b
			bestIdx = append([]int(nil), cur...)
		}
	}
	choose = func(start, size int, cur []int) {
		if len(cur) == size {
			consider(cur)
			return
		}
		for i := start; i < n; i++ {
			choose(i+1, size, append(cur, i))
		}
	}

	// Visit sizes 1..limit; within each size subsets come out in ascending-index order, so
	// the first subset reaching a new maximum is the lexicographically smallest one.
	for size := 1; size <= limit; size++ {
		choose(0, size, make([]int, 0, size))
	}
	return bestIdx, best
}

// BotDecision picks the bot's single action for its turn. It computes BestPlay; when a
// discard is still available (discardsLeft > 0) and the best hand is weaker than
// THREE_OF_A_KIND (i.e. HIGH_CARD / PAIR / TWO_PAIR) it returns "DISCARD" with the 1..5
// lowest-rank cards that are NOT part of the largest rank group; otherwise it returns
// "PLAY" with the BestPlay indices. Fully deterministic.
func BotDecision(hand []models.Card, discardsLeft int) (action string, indices []int) {
	bestIdx, breakdown := BestPlay(hand, discardsLeft)
	if breakdown != nil && discardsLeft > 0 && handTypeRank[breakdown.PokerHand] < handTypeRank[ThreeOfAKind] {
		return "DISCARD", discardIndices(hand)
	}
	return "PLAY", bestIdx
}

// discardIndices returns the indices (into hand, ascending) of the 1..5 lowest-rank cards
// that are NOT part of the largest rank group. The largest rank group is the rank that
// appears most often (ties broken by higher rank, matching Evaluate's ordering); its cards
// are the ones worth keeping. Deterministic.
func discardIndices(hand []models.Card) []int {
	if len(hand) == 0 {
		return nil
	}

	byRank := map[int][]int{}
	for i, c := range hand {
		v := rankValue(c.Rank)
		byRank[v] = append(byRank[v], i)
	}
	// Pick the largest rank group (ties -> higher rank), as Evaluate does.
	bestVal, bestCount := 0, 0
	for v, idx := range byRank {
		if len(idx) > bestCount || (len(idx) == bestCount && v > bestVal) {
			bestVal, bestCount = v, len(idx)
		}
	}
	keep := map[int]bool{}
	for _, i := range byRank[bestVal] {
		keep[i] = true
	}

	candidates := make([]int, 0, len(hand))
	for i := range hand {
		if !keep[i] {
			candidates = append(candidates, i)
		}
	}
	// All cards share one rank: nothing to discard outside the group, discard the lowest.
	if len(candidates) == 0 {
		candidates = make([]int, len(hand))
		for i := range hand {
			candidates[i] = i
		}
	}

	// Lowest rank first; ties by ascending index for determinism.
	sort.Slice(candidates, func(a, b int) bool {
		va, vb := rankValue(hand[candidates[a]].Rank), rankValue(hand[candidates[b]].Rank)
		if va != vb {
			return va < vb
		}
		return candidates[a] < candidates[b]
	})

	n := len(candidates)
	if n > MaxPlayCards {
		n = MaxPlayCards
	}
	out := append([]int(nil), candidates[:n]...)
	sort.Ints(out)
	return out
}
