package poker

import (
	"fmt"
	"strings"
	"testing"

	"jokerarena-game-ms/internal/models"
)

// hand parses "KS KH 10D" style notation (rank + suit letter S/H/C/D).
func hand(t *testing.T, spec string) []models.Card {
	t.Helper()
	suits := map[byte]string{'S': "SPADES", 'H': "HEARTS", 'C': "CLUBS", 'D': "DIAMONDS"}
	var cards []models.Card
	for _, tok := range strings.Fields(spec) {
		rank, suit := tok[:len(tok)-1], suits[tok[len(tok)-1]]
		chips, ok := models.RankChips[rank]
		if !ok || suit == "" {
			t.Fatalf("bad card token %q", tok)
		}
		cards = append(cards, models.Card{Rank: rank, Suit: suit, Chips: chips})
	}
	return cards
}

func TestEvaluateHandTypes(t *testing.T) {
	tests := []struct {
		name    string
		cards   string
		want    HandType
		scoring int  // number of scoring cards
		pair    bool // contains pair
	}{
		{"high card single", "AS", HighCard, 1, false},
		{"high card five", "AS 9H 7C 4D 2S", HighCard, 1, false},
		{"pair", "9S 9H 4C", Pair, 2, true},
		{"pair with kickers", "KS KH 7C 4D 2S", Pair, 2, true},
		{"two pair four cards", "9S 9H 4C 4D", TwoPair, 4, true},
		{"two pair five cards", "9S 9H 4C 4D AS", TwoPair, 4, true},
		{"three of a kind", "QS QH QC 4D 2S", ThreeOfAKind, 3, true},
		{"three of a kind three cards", "QS QH QC", ThreeOfAKind, 3, true},
		{"straight", "5S 6H 7C 8D 9S", Straight, 5, false},
		{"straight unordered", "9S 5H 8C 6D 7S", Straight, 5, false},
		{"straight ace high", "10S JH QC KD AS", Straight, 5, false},
		{"straight ace low (wheel)", "AS 2H 3C 4D 5S", Straight, 5, false},
		{"flush", "2H 6H 9H JH KH", Flush, 5, false},
		{"full house", "KS KH KC 4D 4S", FullHouse, 5, true},
		{"four of a kind", "7S 7H 7C 7D 2S", FourOfAKind, 4, true},
		{"four of a kind four cards", "7S 7H 7C 7D", FourOfAKind, 4, true},
		{"straight flush", "5H 6H 7H 8H 9H", StraightFlush, 5, false},
		{"royal flush is a straight flush", "10S JS QS KS AS", StraightFlush, 5, false},
		{"four cards same suit is not a flush", "2H 6H 9H JH", HighCard, 1, false},
		{"four consecutive is not a straight", "5S 6H 7C 8D", HighCard, 1, false},
		{"wrap-around is not a straight", "QS KH AC 2D 3S", HighCard, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := Evaluate(hand(t, tc.cards))
			if err != nil {
				t.Fatal(err)
			}
			if ev.Type != tc.want {
				t.Errorf("type = %s, want %s", ev.Type, tc.want)
			}
			if len(ev.ScoringIdx) != tc.scoring {
				t.Errorf("scoring cards = %d (%v), want %d", len(ev.ScoringIdx), ev.ScoringIdx, tc.scoring)
			}
			if ev.ContainsPair != tc.pair {
				t.Errorf("containsPair = %v, want %v", ev.ContainsPair, tc.pair)
			}
		})
	}
}

func TestEvaluateHighCardPicksHighestRank(t *testing.T) {
	ev, _ := Evaluate(hand(t, "4S KH 9C"))
	if len(ev.ScoringIdx) != 1 || ev.ScoringIdx[0] != 1 {
		t.Fatalf("expected index 1 (K) to score, got %v", ev.ScoringIdx)
	}
}

func TestEvaluateIsOrderIndependent(t *testing.T) {
	a, _ := Evaluate(hand(t, "KS 4D KH 4S KC"))
	b, _ := Evaluate(hand(t, "4S KC 4D KS KH"))
	if a.Type != FullHouse || b.Type != FullHouse {
		t.Fatalf("both orders must be a full house, got %s and %s", a.Type, b.Type)
	}
}

func TestEvaluateInvalidSelections(t *testing.T) {
	if _, err := Evaluate(nil); err != ErrInvalidSelection {
		t.Errorf("empty: want ErrInvalidSelection, got %v", err)
	}
	if _, err := Evaluate(hand(t, "2S 3S 4S 5S 6S 7S")); err != ErrInvalidSelection {
		t.Errorf("six cards: want ErrInvalidSelection, got %v", err)
	}
}

func TestScoreSpecExampleFullHouse(t *testing.T) {
	// Example of the specification: K♠ K♥ K♣ 4♦ 4♠ with 0 discards left = 78 x 16 = 1248.
	b, err := Score(hand(t, "KS KH KC 4D 4S"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if b.PokerHand != FullHouse || b.BaseChips != 40 || b.BaseMult != 4 || b.CardChips != 38 {
		t.Fatalf("unexpected base: %+v", b)
	}
	if b.FinalChips != 78 || b.FinalMult != 16 || b.Score != 1248 {
		t.Fatalf("got %d x %d = %d, want 78 x 16 = 1248", b.FinalChips, b.FinalMult, b.Score)
	}
	t.Logf("Full House: (%d base + %d cartas + %d Banner) x ((%d + %d) x %.0f) = %d",
		b.BaseChips, b.CardChips, b.Jokers[0].ChipsAdded, b.BaseMult, b.Jokers[1].MultAdded, b.Jokers[2].MultFactor, b.Score)
}

func TestScoreBannerUsesRemainingDiscards(t *testing.T) {
	// Same Full House with 2 discards left: chips 78 + 60 = 138, mult 16 -> 2208.
	b, _ := Score(hand(t, "KS KH KC 4D 4S"), 2)
	if b.Jokers[0].ChipsAdded != 60 || !b.Jokers[0].Triggered {
		t.Fatalf("banner: %+v", b.Jokers[0])
	}
	if b.FinalChips != 138 || b.Score != 2208 {
		t.Fatalf("got chips=%d score=%d, want 138 / 2208", b.FinalChips, b.Score)
	}
}

func TestScoreTable(t *testing.T) {
	tests := []struct {
		name      string
		cards     string
		discards  int
		wantChips int
		wantMult  int
		wantScore int
	}{
		// Only the highest card scores: (5 + 11) x (1 + 4) = 80.
		{"high card ace", "AS 9H 7C 4D 2S", 0, 16, 5, 80},
		// Pair of 9s (kickers do not score): (10 + 18) x ((2 + 4) x 2) = 336.
		{"pair of nines", "9S 9H 4C", 0, 28, 12, 336},
		// Two pair: (20 + 9+9+4+4) x ((2 + 4) x 2) = 46 x 12 = 552.
		{"two pair", "9S 9H 4C 4D AS", 0, 46, 12, 552},
		// Three of a kind: (30 + 30) x ((3 + 4) x 2) = 60 x 14 = 840.
		{"three queens", "QS QH QC 4D 2S", 0, 60, 14, 840},
		// Straight has no pair: (30 + 35) x (4 + 4) = 520.
		{"straight 5-9", "5S 6H 7C 8D 9S", 0, 65, 8, 520},
		// Flush: (35 + 2+6+9+10+10) x (4 + 4) = 72 x 8 = 576.
		{"flush", "2H 6H 9H JH KH", 0, 72, 8, 576},
		// Four of a kind (kicker does not score): (60 + 28) x ((7 + 4) x 2) = 88 x 22 = 1936.
		{"four sevens", "7S 7H 7C 7D 2S", 0, 88, 22, 1936},
		// Straight flush: (100 + 35) x (8 + 4) = 1620.
		{"straight flush", "5H 6H 7H 8H 9H", 0, 135, 12, 1620},
		// With 3 discards the Banner adds 90 chips: (5+11+90) x 5 = 530.
		{"high card with discards", "AS 9H 7C 4D 2S", 3, 106, 5, 530},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := Score(hand(t, tc.cards), tc.discards)
			if err != nil {
				t.Fatal(err)
			}
			if b.FinalChips != tc.wantChips || b.FinalMult != tc.wantMult || b.Score != tc.wantScore {
				t.Errorf("got %d x %d = %d, want %d x %d = %d",
					b.FinalChips, b.FinalMult, b.Score, tc.wantChips, tc.wantMult, tc.wantScore)
			}
		})
	}
}

func TestScoreMarksOnlyFormingCardsAsScored(t *testing.T) {
	b, _ := Score(hand(t, "KS KH 7C"), 0)
	scored := 0
	for _, c := range b.Cards {
		if c.Scored {
			scored++
		}
	}
	if scored != 2 || b.Cards[2].Scored {
		t.Fatalf("expected only the two kings to score: %+v", b.Cards)
	}
}

func TestScoreTheDuoOnlyWithPair(t *testing.T) {
	noPair, _ := Score(hand(t, "5S 6H 7C 8D 9S"), 0)
	if noPair.Jokers[2].Triggered || noPair.Jokers[2].MultFactor != 1 {
		t.Errorf("The Duo must not trigger on a straight: %+v", noPair.Jokers[2])
	}
	withPair, _ := Score(hand(t, "5S 5H"), 0)
	if !withPair.Jokers[2].Triggered || withPair.Jokers[2].MultFactor != 2 {
		t.Errorf("The Duo must trigger on a pair: %+v", withPair.Jokers[2])
	}
}

func TestJokerOrderIsFixed(t *testing.T) {
	b, _ := Score(hand(t, "AS"), 1)
	ids := []string{b.Jokers[0].ID, b.Jokers[1].ID, b.Jokers[2].ID}
	if fmt.Sprint(ids) != fmt.Sprint(ActiveJokers) {
		t.Fatalf("joker order %v, want %v", ids, ActiveJokers)
	}
}

func TestDeckHas52UniqueCards(t *testing.T) {
	deck := NewDeck()
	if len(deck) != 52 {
		t.Fatalf("deck size %d", len(deck))
	}
	seen := map[string]bool{}
	for _, c := range deck {
		key := c.Rank + c.Suit
		if seen[key] {
			t.Fatalf("duplicate card %s", key)
		}
		seen[key] = true
		if c.Chips != models.RankChips[c.Rank] {
			t.Fatalf("wrong chips for %s: %d", key, c.Chips)
		}
	}
}

func TestShuffleIsDeterministicAndComplete(t *testing.T) {
	a, b := NewShuffledDeck(42), NewShuffledDeck(42)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed produced different order at %d", i)
		}
	}
	c := NewShuffledDeck(43)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical decks")
	}
	seen := map[string]bool{}
	for _, card := range a {
		seen[card.Rank+card.Suit] = true
	}
	if len(a) != 52 || len(seen) != 52 {
		t.Fatalf("shuffled deck lost cards: %d unique of %d", len(seen), len(a))
	}
	ordered := NewDeck()
	identical := true
	for i := range a {
		if a[i] != ordered[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Fatal("shuffle left the deck in its original order")
	}
}
