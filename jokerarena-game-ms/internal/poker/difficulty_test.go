package poker_test

import (
	"math"
	"sort"
	"testing"

	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/poker"
	"jokerarena-game-ms/internal/room"
)

// Thresholds of the duration/difficulty requirement (see .agents/tasks/plan.md).
const (
	games               = 1000
	maxSimHands         = 60
	simCap              = 33000
	minP5Classic        = 13
	minP1               = 8
	minWinOptClassic    = 0.90
	minWinCasualClassic = 0.50
	maxWinOptEpic       = 0.30
	paceSeconds         = 25.0
	targetSeconds       = 300.0
	marginPct           = 0.08
)

var goals = []struct {
	name  string
	score int
}{
	{"Clásica", 22000},
	{"Desafío", 26000},
	{"Épica", 33000}, // ajustada de 30000 por la regla A (30000 daba 60,7 % de victoria óptima)
}

// combos lists every index combination of 1..maxCards cards out of n.
func combos(n, maxCards int) [][]int {
	var out [][]int
	var rec func(start int, cur []int)
	rec = func(start int, cur []int) {
		if len(cur) > 0 {
			out = append(out, append([]int(nil), cur...))
		}
		if len(cur) == maxCards {
			return
		}
		for i := start; i < n; i++ {
			rec(i+1, append(cur, i))
		}
	}
	rec(0, nil)
	return out
}

// trajectory plays one match without a hand limit and returns the cumulative score after
// each hand (stops above simCap or after maxSimHands). No discards are used.
func trajectory(seed uint64, cs [][]int) []int {
	dealer := poker.NewDealer(seed)
	hand := dealer.Draw(room.HandSize)
	var acc int
	var traj []int
	for len(traj) < maxSimHands && acc < simCap {
		best, bestScore := cs[0], -1
		for _, c := range cs {
			sel := make([]models.Card, len(c))
			for i, idx := range c {
				sel[i] = hand[idx]
			}
			b, err := poker.Score(sel, room.InitialDiscards)
			if err != nil {
				panic(err)
			}
			if b.Score > bestScore {
				best, bestScore = c, b.Score
			}
		}
		acc += bestScore
		traj = append(traj, acc)
		removed := map[int]bool{}
		for _, i := range best {
			removed[i] = true
		}
		kept := make([]models.Card, 0, room.HandSize)
		var taken []models.Card
		for i, c := range hand {
			if removed[i] {
				taken = append(taken, c)
			} else {
				kept = append(kept, c)
			}
		}
		kept = append(kept, dealer.Draw(room.HandSize-len(kept))...)
		dealer.Discard(taken)
		hand = kept
	}
	return traj
}

// nMin is the first hand number whose cumulative score reaches goal (maxSimHands+1 if never).
func nMin(traj []int, goal int) int {
	for i, v := range traj {
		if v >= goal {
			return i + 1
		}
	}
	return maxSimHands + 1
}

type stats struct {
	p1, p5, median int
	win            float64
}

func summarize(ns []int) stats {
	s := append([]int(nil), ns...)
	sort.Ints(s)
	pct := func(p float64) int { return s[int(math.Ceil(p*float64(len(s))))-1] }
	wins := 0
	for _, n := range s {
		if n <= room.InitialHands {
			wins++
		}
	}
	return stats{p1: pct(0.01), p5: pct(0.05), median: pct(0.5), win: float64(wins) / float64(len(s))}
}

func TestDifficultySimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("difficulty simulation skipped with -short")
	}
	policies := []struct {
		name string
		cs   [][]int
	}{
		{"óptima", combos(room.HandSize, poker.MaxPlayCards)},
		{"casual", combos(room.HandSize, 3)},
	}
	res := map[string]map[int]stats{}
	goalList := []int{20000, 21000, 22000, 26000, 30000, 33000}
	for _, p := range policies {
		ns := map[int][]int{}
		for seed := uint64(1); seed <= games; seed++ {
			traj := trajectory(seed, p.cs)
			for _, g := range goalList {
				ns[g] = append(ns[g], nMin(traj, g))
			}
		}
		res[p.name] = map[int]stats{}
		for _, g := range goalList {
			res[p.name][g] = summarize(ns[g])
		}
	}

	t.Logf("InitialHands=%d, %d partidas por política (semillas 1..%d)", room.InitialHands, games, games)
	t.Logf("%-7s %-9s %4s %4s %7s %9s", "meta", "política", "P1", "P5", "mediana", "victoria")
	for _, g := range goalList {
		for _, p := range policies {
			st := res[p.name][g]
			t.Logf("%-7d %-9s %4d %4d %7d %8.1f%%", g, p.name, st.p1, st.p5, st.median, st.win*100)
		}
	}

	classic := res["óptima"][22000]
	need := targetSeconds * (1 + marginPct)
	t.Logf("Clásica óptima: P5 x %.0f s = %.0f s vs %.0f s requeridos (300 s +%.0f%%); ritmo de equilibrio = %.1f s/jugada",
		paceSeconds, float64(classic.p5)*paceSeconds, need, marginPct*100, targetSeconds/float64(classic.p5))

	if classic.p5 < minP5Classic {
		t.Errorf("Clásica N_min P5 = %d, want >= %d", classic.p5, minP5Classic)
	}
	if float64(classic.p5)*paceSeconds < need {
		t.Errorf("P5 x %.0f s = %.0f s < %.0f s", paceSeconds, float64(classic.p5)*paceSeconds, need)
	}
	for _, g := range goals {
		if st := res["óptima"][g.score]; st.p1 < minP1 {
			t.Errorf("%s (%d): N_min P1 = %d, want >= %d", g.name, g.score, st.p1, minP1)
		}
	}
	if w := classic.win; w < minWinOptClassic {
		t.Errorf("Clásica victoria óptima %.1f%% < %.0f%%", w*100, minWinOptClassic*100)
	}
	if w := res["casual"][22000].win; w < minWinCasualClassic {
		t.Errorf("Clásica victoria casual %.1f%% < %.0f%%", w*100, minWinCasualClassic*100)
	}
	if w := res["óptima"][33000].win; w > maxWinOptEpic {
		t.Errorf("Épica victoria óptima %.1f%% > %.0f%%", w*100, maxWinOptEpic*100)
	}
}
