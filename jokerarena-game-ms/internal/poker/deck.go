// Package poker implements the pure game rules of JokerArena: the 52-card French deck,
// the deterministic recognition of the nine poker hands and the Balatro-style scoring
// cascade (hand chips x mult with the three canonical Jokers). It has no I/O.
package poker

import (
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand/v2"

	"jokerarena-game-ms/internal/models"
)

// NewDeck returns the ordered 52-card French deck (4 suits x 13 ranks).
func NewDeck() []models.Card {
	deck := make([]models.Card, 0, 52)
	for _, suit := range models.FrenchSuits {
		for _, rank := range models.FrenchRanks {
			deck = append(deck, models.Card{Rank: rank, Suit: suit, Chips: models.RankChips[rank]})
		}
	}
	return deck
}

// Shuffle performs an in-place Fisher-Yates shuffle driven by a PCG generator seeded with
// seed, so the same seed always yields the same order (reproducible matches and tests).
func Shuffle(deck []models.Card, seed uint64) {
	r := mrand.New(mrand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

// NewShuffledDeck returns a fresh 52-card deck shuffled deterministically from seed.
func NewShuffledDeck(seed uint64) []models.Card {
	deck := NewDeck()
	Shuffle(deck, seed)
	return deck
}

// RandomSeed returns a cryptographically random seed for a new match.
func RandomSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("poker: crypto/rand unavailable: " + err.Error())
	}
	return binary.LittleEndian.Uint64(b[:])
}
