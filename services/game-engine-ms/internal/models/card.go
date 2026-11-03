package models

// French deck constants
var (
	FrenchSuits = []string{"SPADES", "HEARTS", "CLUBS", "DIAMONDS"}
	FrenchRanks = []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	RankChips   = map[string]int{
		"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9, "10": 10,
		"J": 10, "Q": 10, "K": 10, "A": 11,
	}
)

// Card represents a French deck playing card with standard chip valuation.
type Card struct {
	Rank  string `json:"rank"`
	Suit  string `json:"suit"`
	Chips int    `json:"chips"`
}
