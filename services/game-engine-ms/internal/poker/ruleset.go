package poker

// Ruleset holds the tunable balance values of the Joker cascade (contracts/ruleset.md).
// It is owned by liveops-rules-ms and hot-reloaded by the engine.
type Ruleset struct {
	Version   int    `json:"version"`
	UpdatedAt string `json:"updated_at,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`
	Jokers    struct {
		Banner struct {
			Enabled         bool `json:"enabled"`
			ChipsPerDiscard int  `json:"chips_per_discard"`
		} `json:"banner"`
		Classic struct {
			Enabled   bool `json:"enabled"`
			MultBonus int  `json:"mult_bonus"`
		} `json:"classic"`
		Duo struct {
			Enabled    bool `json:"enabled"`
			MultFactor int  `json:"mult_factor"`
		} `json:"duo"`
	} `json:"jokers"`
	HappyHour struct {
		Enabled    bool `json:"enabled"`
		GlobalMult int  `json:"global_mult"`
	} `json:"happy_hour"`
}

// DefaultRuleset reproduces exactly the Prototype 1 values (version 1).
func DefaultRuleset() Ruleset {
	var rs Ruleset
	rs.Version = 1
	rs.UpdatedBy = "default"
	rs.Jokers.Banner.Enabled = true
	rs.Jokers.Banner.ChipsPerDiscard = bannerChipsPerDiscard
	rs.Jokers.Classic.Enabled = true
	rs.Jokers.Classic.MultBonus = classicMultBonus
	rs.Jokers.Duo.Enabled = true
	rs.Jokers.Duo.MultFactor = duoMultFactor
	rs.HappyHour.Enabled = false
	rs.HappyHour.GlobalMult = 1
	return rs
}

// Valid reports whether every value is inside the ranges of the contract.
func (rs Ruleset) Valid() bool {
	in := func(v, lo, hi int) bool { return v >= lo && v <= hi }
	return rs.Version >= 1 &&
		in(rs.Jokers.Banner.ChipsPerDiscard, 0, 200) &&
		in(rs.Jokers.Classic.MultBonus, 0, 50) &&
		in(rs.Jokers.Duo.MultFactor, 1, 10) &&
		in(rs.HappyHour.GlobalMult, 1, 5)
}
