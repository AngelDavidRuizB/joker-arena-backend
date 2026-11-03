package ruleset

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"game-engine-ms/internal/poker"
)

func rulesetJSON(t *testing.T, version, banner int) []byte {
	rs := poker.DefaultRuleset()
	rs.Version = version
	rs.Jokers.Banner.ChipsPerDiscard = banner
	b, err := json.Marshal(rs)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProviderStartsWithDefault(t *testing.T) {
	p := NewProvider()
	if p.Current().Version != 1 || p.Source() != "default" {
		t.Fatalf("got %+v from %s", p.Current(), p.Source())
	}
}

func TestProviderSwapsOnEventAndIgnoresStaleVersions(t *testing.T) {
	p := NewProvider()
	if err := p.ApplyJSON(rulesetJSON(t, 3, 45), "liveops-event"); err != nil {
		t.Fatal(err)
	}
	if got := p.Current(); got.Version != 3 || got.Jokers.Banner.ChipsPerDiscard != 45 {
		t.Fatalf("swap failed: %+v", got)
	}
	if err := p.ApplyJSON(rulesetJSON(t, 2, 99), "liveops-event"); err == nil {
		t.Fatal("an older version must be ignored")
	}
	if p.Current().Jokers.Banner.ChipsPerDiscard != 45 {
		t.Fatal("stale event changed the ruleset")
	}
}

func TestProviderRejectsInvalidValues(t *testing.T) {
	p := NewProvider()
	if err := p.ApplyJSON(rulesetJSON(t, 2, 999), "liveops-event"); err == nil {
		t.Fatal("banner 999 is out of range")
	}
	if err := p.ApplyJSON([]byte("{roto"), "liveops-event"); err == nil {
		t.Fatal("invalid JSON must fail")
	}
	if p.Current().Version != 1 {
		t.Fatal("invalid input changed the ruleset")
	}
}

func TestSeedFromLiveops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/liveops/ruleset" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(rulesetJSON(t, 2, 45))
	}))
	defer srv.Close()
	p := NewProvider()
	if err := p.Seed(context.Background(), srv.Client(), srv.URL); err != nil {
		t.Fatal(err)
	}
	if p.Current().Version != 2 || p.Source() != "liveops-http" {
		t.Fatalf("seed failed: %+v", p.Current())
	}
}

func TestSeedFailsGracefullyWhenLiveopsIsDown(t *testing.T) {
	p := NewProvider()
	if err := p.Seed(context.Background(), &http.Client{}, "http://127.0.0.1:1"); err == nil {
		t.Fatal("expected an error")
	}
	if p.Current().Version != 1 {
		t.Fatal("must keep the default ruleset")
	}
}
