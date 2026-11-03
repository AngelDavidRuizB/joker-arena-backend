package poker

import "testing"

// Full House K K K 4 4: base 40 chips, card chips 38, base mult 4.
func TestRulesetDefaultKeepsCanonical1248(t *testing.T) {
	b, err := ScoreWithRuleset(hand(t, "KS KH KC 4D 4S"), 0, DefaultRuleset())
	if err != nil {
		t.Fatal(err)
	}
	if b.Score != 1248 || b.RulesetVersion != 1 {
		t.Fatalf("default ruleset must score 1248 (v1), got %d (v%d)", b.Score, b.RulesetVersion)
	}
}

func TestRulesetVariants(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*Ruleset)
		discards int
		chips    int
		mult     int
		score    int
	}{
		// banner 45 x 2 discards = 90 -> (78+90) x 16 = 2688
		{"banner 45", func(r *Ruleset) { r.Jokers.Banner.ChipsPerDiscard = 45 }, 2, 168, 16, 2688},
		// duo disabled -> mult 4+4 = 8 -> 78 x 8 = 624
		{"duo disabled", func(r *Ruleset) { r.Jokers.Duo.Enabled = false }, 0, 78, 8, 624},
		// happy hour x2 -> mult 32 -> 2496
		{"happy hour x2", func(r *Ruleset) { r.HappyHour.Enabled = true; r.HappyHour.GlobalMult = 2 }, 0, 78, 32, 2496},
		// happy hour disabled ignores global_mult
		{"happy hour off", func(r *Ruleset) { r.HappyHour.GlobalMult = 3 }, 0, 78, 16, 1248},
		// all jokers off -> base only: 78 x 4 = 312 (banner off even with discards)
		{"all jokers off", func(r *Ruleset) {
			r.Jokers.Banner.Enabled, r.Jokers.Classic.Enabled, r.Jokers.Duo.Enabled = false, false, false
		}, 3, 78, 4, 312},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := DefaultRuleset()
			rs.Version = 2
			c.mutate(&rs)
			b, err := ScoreWithRuleset(hand(t, "KS KH KC 4D 4S"), c.discards, rs)
			if err != nil {
				t.Fatal(err)
			}
			if b.FinalChips != c.chips || b.FinalMult != c.mult || b.Score != c.score {
				t.Fatalf("got %d x %d = %d, want %d x %d = %d", b.FinalChips, b.FinalMult, b.Score, c.chips, c.mult, c.score)
			}
			if len(b.Jokers) != 3 || b.RulesetVersion != 2 {
				t.Fatalf("audit must keep the 3 jokers and the version: %+v", b.Jokers)
			}
		})
	}
}

func TestRulesetDisabledJokerIsNotTriggered(t *testing.T) {
	rs := DefaultRuleset()
	rs.Jokers.Classic.Enabled = false
	b, _ := ScoreWithRuleset(hand(t, "KS KH KC 4D 4S"), 0, rs)
	if b.Jokers[1].Triggered || b.Jokers[1].MultAdded != 0 {
		t.Fatalf("disabled classic must not trigger: %+v", b.Jokers[1])
	}
}

func TestRulesetValid(t *testing.T) {
	if !DefaultRuleset().Valid() {
		t.Fatal("default must be valid")
	}
	rs := DefaultRuleset()
	rs.Jokers.Duo.MultFactor = 0
	if rs.Valid() {
		t.Fatal("duo factor 0 must be invalid")
	}
}
