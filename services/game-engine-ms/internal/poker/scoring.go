package poker

import (
	"fmt"

	"game-engine-ms/internal/models"
)

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
	Enabled    bool    `json:"enabled"` // false: disabled by LiveOps (not a game-state miss)
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
	// RulesetVersion is the LiveOps ruleset version used to score this hand.
	RulesetVersion int `json:"ruleset_version"`
}

// Score scores a hand with the default ruleset (Prototype 1 values). Kept so that the
// canonical tests and the local fallback bot keep their exact behavior.
func Score(cards []models.Card, discardsLeft int) (*Breakdown, error) {
	return ScoreWithRuleset(cards, discardsLeft, DefaultRuleset())
}

// ScoreWithRuleset recognizes the selected cards and applies the Joker cascade in order,
// with the values of the given ruleset:
//
//	Banner  (+chips_per_discard chips per remaining discard)
//	Classic (+mult_bonus mult)
//	The Duo (x mult_factor mult if the hand contains a pair)
//	Happy Hour (x global_mult on the final mult, when enabled)
//
// A disabled Joker stays in the audit with Triggered=false and contributes nothing, so the
// shape of the breakdown is stable for every client.
func ScoreWithRuleset(cards []models.Card, discardsLeft int, rs Ruleset) (*Breakdown, error) {
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
	bannerOn := rs.Jokers.Banner.Enabled
	banner := JokerEffect{ID: JokerBanner, Name: "Banner", MultFactor: 1, Enabled: bannerOn,
		Effect:    fmt.Sprintf("+%d Chips por cada descarte restante", rs.Jokers.Banner.ChipsPerDiscard),
		Triggered: bannerOn && discardsLeft > 0}
	if banner.Triggered {
		banner.ChipsAdded = rs.Jokers.Banner.ChipsPerDiscard * discardsLeft
	}
	chips += banner.ChipsAdded

	// 2. Joker Clasico: ADD_MULT, always active when enabled.
	classic := JokerEffect{ID: JokerClassic, Name: "Joker Clásico", MultFactor: 1, Enabled: rs.Jokers.Classic.Enabled,
		Effect:    fmt.Sprintf("+%d Mult en toda mano", rs.Jokers.Classic.MultBonus),
		Triggered: rs.Jokers.Classic.Enabled}
	if classic.Triggered {
		classic.MultAdded = rs.Jokers.Classic.MultBonus
	}
	mult += classic.MultAdded

	// 3. The Duo: X_MULT, only if the hand contains a pair.
	duo := JokerEffect{ID: JokerDuo, Name: "The Duo", MultFactor: 1, Enabled: rs.Jokers.Duo.Enabled,
		Effect:    fmt.Sprintf("x%d Mult si la mano contiene un Par", rs.Jokers.Duo.MultFactor),
		Triggered: rs.Jokers.Duo.Enabled && ev.ContainsPair}
	if duo.Triggered {
		duo.MultFactor = float64(rs.Jokers.Duo.MultFactor)
		mult *= rs.Jokers.Duo.MultFactor
	}

	// 4. Happy Hour (LiveOps): global multiplier on the final mult.
	if rs.HappyHour.Enabled && rs.HappyHour.GlobalMult > 1 {
		mult *= rs.HappyHour.GlobalMult
	}

	return &Breakdown{
		PokerHand:      ev.Type,
		BaseChips:      base.Chips,
		BaseMult:       base.Mult,
		CardChips:      cardChips,
		Cards:          played,
		Jokers:         []JokerEffect{banner, classic, duo},
		DiscardsLeft:   discardsLeft,
		FinalChips:     chips,
		FinalMult:      mult,
		Score:          chips * mult,
		RulesetVersion: rs.Version,
	}, nil
}
