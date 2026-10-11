package poker

import (
	"errors"
	"sort"

	"jokerarena-game-ms/internal/models"
)

// HandType identifies one of the nine recognized poker hands.
type HandType string

const (
	HighCard      HandType = "HIGH_CARD"
	Pair          HandType = "PAIR"
	TwoPair       HandType = "TWO_PAIR"
	ThreeOfAKind  HandType = "THREE_OF_A_KIND"
	Straight      HandType = "STRAIGHT"
	Flush         HandType = "FLUSH"
	FullHouse     HandType = "FULL_HOUSE"
	FourOfAKind   HandType = "FOUR_OF_A_KIND"
	StraightFlush HandType = "STRAIGHT_FLUSH"
)

// HandBase holds the base chips and multiplier of a hand type (Balatro base values).
type HandBase struct {
	Chips int
	Mult  int
}

// Bases lists the base values of every hand type, as specified for Prototype 1.
var Bases = map[HandType]HandBase{
	HighCard:      {5, 1},
	Pair:          {10, 2},
	TwoPair:       {20, 2},
	ThreeOfAKind:  {30, 3},
	Straight:      {30, 4},
	Flush:         {35, 4},
	FullHouse:     {40, 4},
	FourOfAKind:   {60, 7},
	StraightFlush: {100, 8},
}

// MaxPlayCards is the maximum number of cards in a play or discard.
const MaxPlayCards = 5

// ErrInvalidSelection is returned for an empty or oversized card selection.
var ErrInvalidSelection = errors.New("a hand must contain between 1 and 5 cards")

// Evaluation is the result of recognizing a selection of cards.
type Evaluation struct {
	Type HandType
	// ScoringIdx are the indices (ascending, into the evaluated slice) of the cards that
	// form the recognized hand; only those contribute their chips.
	ScoringIdx []int
	// ContainsPair is true when two or more cards share a rank (Pair, Two Pair, Three/Four of
	// a Kind, Full House). It triggers The Duo joker.
	ContainsPair bool
}

var rankValues = map[string]int{
	"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9, "10": 10,
	"J": 11, "Q": 12, "K": 13, "A": 14,
}

func rankValue(rank string) int { return rankValues[rank] }

type rankGroup struct {
	value   int
	indices []int
}

// Evaluate recognizes the best poker hand in a selection of 1 to 5 cards. Straights and
// flushes require exactly five cards. The ace counts high, and also low for A-2-3-4-5.
func Evaluate(cards []models.Card) (Evaluation, error) {
	n := len(cards)
	if n < 1 || n > MaxPlayCards {
		return Evaluation{}, ErrInvalidSelection
	}

	byRank := map[int]*rankGroup{}
	for i, c := range cards {
		v := rankValue(c.Rank)
		g, ok := byRank[v]
		if !ok {
			g = &rankGroup{value: v}
			byRank[v] = g
		}
		g.indices = append(g.indices, i)
	}
	groups := make([]*rankGroup, 0, len(byRank))
	for _, g := range byRank {
		groups = append(groups, g)
	}
	// Largest groups first, then highest rank first (deterministic order).
	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].indices) != len(groups[j].indices) {
			return len(groups[i].indices) > len(groups[j].indices)
		}
		return groups[i].value > groups[j].value
	})

	all := make([]int, n)
	for i := range all {
		all[i] = i
	}

	flush := n == MaxPlayCards
	if flush {
		for _, c := range cards[1:] {
			if c.Suit != cards[0].Suit {
				flush = false
				break
			}
		}
	}
	straight := n == MaxPlayCards && isStraight(groups)
	top := len(groups[0].indices)
	containsPair := top >= 2

	build := func(t HandType, idx []int) (Evaluation, error) {
		out := append([]int(nil), idx...)
		sort.Ints(out)
		return Evaluation{Type: t, ScoringIdx: out, ContainsPair: containsPair}, nil
	}

	switch {
	case straight && flush:
		return build(StraightFlush, all)
	case top == 4:
		return build(FourOfAKind, groups[0].indices)
	case top == 3 && len(groups) > 1 && len(groups[1].indices) == 2:
		return build(FullHouse, all)
	case flush:
		return build(Flush, all)
	case straight:
		return build(Straight, all)
	case top == 3:
		return build(ThreeOfAKind, groups[0].indices)
	case top == 2 && len(groups) > 1 && len(groups[1].indices) == 2:
		idx := append(append([]int(nil), groups[0].indices...), groups[1].indices...)
		return build(TwoPair, idx)
	case top == 2:
		return build(Pair, groups[0].indices)
	default:
		return build(HighCard, []int{groups[0].indices[0]}) // highest card
	}
}

// isStraight reports whether five distinct ranks are consecutive (ace high or low).
func isStraight(groups []*rankGroup) bool {
	if len(groups) != MaxPlayCards {
		return false
	}
	values := make([]int, len(groups))
	for i, g := range groups {
		values[i] = g.value
	}
	sort.Ints(values)
	if values[4]-values[0] == 4 {
		return true
	}
	return values[0] == 2 && values[1] == 3 && values[2] == 4 && values[3] == 5 && values[4] == 14
}
