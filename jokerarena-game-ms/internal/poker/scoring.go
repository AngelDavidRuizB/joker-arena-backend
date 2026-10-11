package poker

import "jokerarena-game-ms/internal/models"

// Joker identifiers of the Prototype 1 catalog (evaluated strictly in this order).
const (
	JokerBanner  = "banner"
	JokerClassic = "joker"
	JokerDuo     = "the_duo"
)

// ActiveJokers is the ordered list of Jokers every player owns in Prototype 1.
var ActiveJokers = []string{JokerBanner, JokerClassic, JokerDuo}

const (
	bannerChipsPerDiscard = 30
	classicMultBonus      = 4
	duoMultFactor         = 2
)

// ScoredCard is a played card with the flag telling whether it contributed its chips.
type ScoredCard struct {
	Rank   string `json:"rank"`
	Suit   string `json:"suit"`
	Chips  int    `json:"chips"`
	Scored bool   `json:"scored"`
}

// JokerEffect describes what a Joker added to the cascade (mirrors telemetry.proto).
type JokerEffect struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Effect     string  `json:"effect"`
	Triggered  bool    `json:"triggered"`
	ChipsAdded int     `json:"chips_added"`
	MultAdded  int     `json:"mult_added"`
	MultFactor float64 `json:"mult_factor"`
}

// Breakdown is the full algebraic audit of one played hand:
//
//	Score = (BaseChips + CardChips + 30*discardsLeft) x ((BaseMult + 4) x (2 if contains a pair))
type Breakdown struct {
	PokerHand    HandType      `json:"poker_hand"`
	BaseChips    int           `json:"base_chips"`
	BaseMult     int           `json:"base_mult"`
	CardChips    int           `json:"card_chips"`
	Cards        []ScoredCard  `json:"cards_played"`
	Jokers       []JokerEffect `json:"jokers"`
	DiscardsLeft int           `json:"discards_left"`
	FinalChips   int           `json:"final_chips"`
	FinalMult    int           `json:"final_mult"`
	Score        int           `json:"score"`
}

// Score recognizes the selected cards and applies the Joker cascade in order:
// Banner (+30 chips per remaining discard), Joker Clasico (+4 mult), The Duo (x2 mult if the
// hand contains a pair). discardsLeft is the number of discards still available when the
// hand is played.
func Score(cards []models.Card, discardsLeft int) (*Breakdown, error) {
	ev, err := Evaluate(cards)
	if err != nil {
		return nil, err
	}
	if discardsLeft < 0 {
		discardsLeft = 0
	}
	base := Bases[ev.Type]

	scoring := make(map[int]bool, len(ev.ScoringIdx))
	for _, i := range ev.ScoringIdx {
		scoring[i] = true
	}
	played := make([]ScoredCard, len(cards))
	cardChips := 0
	for i, c := range cards {
		played[i] = ScoredCard{Rank: c.Rank, Suit: c.Suit, Chips: c.Chips, Scored: scoring[i]}
		if scoring[i] {
			cardChips += c.Chips
		}
	}

	chips := base.Chips + cardChips
	mult := base.Mult

	// 1. Banner: ADD_CHIPS, proportional to remaining discards.
	banner := JokerEffect{ID: JokerBanner, Name: "Banner", MultFactor: 1,
		Effect: "+30 Chips por cada descarte restante", Triggered: discardsLeft > 0,
		ChipsAdded: bannerChipsPerDiscard * discardsLeft}
	chips += banner.ChipsAdded

	// 2. Joker Clasico: ADD_MULT, always active.
	classic := JokerEffect{ID: JokerClassic, Name: "Joker Clásico", MultFactor: 1,
		Effect: "+4 Mult en toda mano", Triggered: true, MultAdded: classicMultBonus}
	mult += classic.MultAdded

	// 3. The Duo: X_MULT, only if the hand contains a pair.
	duo := JokerEffect{ID: JokerDuo, Name: "The Duo", MultFactor: 1,
		Effect: "x2 Mult si la mano contiene un Par", Triggered: ev.ContainsPair}
	if ev.ContainsPair {
		duo.MultFactor = duoMultFactor
		mult *= duoMultFactor
	}

	return &Breakdown{
		PokerHand:    ev.Type,
		BaseChips:    base.Chips,
		BaseMult:     base.Mult,
		CardChips:    cardChips,
		Cards:        played,
		Jokers:       []JokerEffect{banner, classic, duo},
		DiscardsLeft: discardsLeft,
		FinalChips:   chips,
		FinalMult:    mult,
		Score:        chips * mult,
	}, nil
}
