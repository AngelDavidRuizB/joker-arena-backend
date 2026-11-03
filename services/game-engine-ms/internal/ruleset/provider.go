// Package ruleset keeps the LiveOps ruleset currently in force (hot reload).
package ruleset

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"game-engine-ms/internal/poker"
)

// Provider holds the active ruleset behind a lock. Rooms read it at every hand, so a new
// version applies to new rooms immediately and to active rooms from their next hand.
type Provider struct {
	mu      sync.RWMutex
	current poker.Ruleset
	source  string
}

// NewProvider starts with the default (Prototype 1) ruleset.
func NewProvider() *Provider {
	return &Provider{current: poker.DefaultRuleset(), source: "default"}
}

// Current returns a copy of the active ruleset.
func (p *Provider) Current() poker.Ruleset {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current
}

// Source tells where the active ruleset came from: default | liveops-http | liveops-event.
func (p *Provider) Source() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.source
}

// Apply swaps the ruleset if it is valid and not older than the current one.
func (p *Provider) Apply(rs poker.Ruleset, source string) error {
	if !rs.Valid() {
		return fmt.Errorf("ruleset v%d rejected: values out of range", rs.Version)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.source != "default" && rs.Version < p.current.Version {
		return fmt.Errorf("ruleset v%d ignored: v%d already active", rs.Version, p.current.Version)
	}
	p.current = rs
	p.source = source
	return nil
}

// ApplyJSON parses and applies a ruleset document (HTTP body or event payload).
func (p *Provider) ApplyJSON(body []byte, source string) error {
	var rs poker.Ruleset
	if err := json.Unmarshal(body, &rs); err != nil {
		return fmt.Errorf("invalid ruleset JSON: %w", err)
	}
	return p.Apply(rs, source)
}

// Seed fetches GET {baseURL}/liveops/ruleset once.
func (p *Provider) Seed(ctx context.Context, client *http.Client, baseURL string) error {
	url := strings.TrimRight(baseURL, "/") + "/liveops/ruleset"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("liveops answered %d", resp.StatusCode)
	}
	var rs poker.Ruleset
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return fmt.Errorf("invalid ruleset from liveops: %w", err)
	}
	return p.Apply(rs, "liveops-http")
}

// SeedInBackground retries Seed until it succeeds or ctx ends. Meanwhile the engine keeps
// working with the default ruleset (no hard dependency on liveops-rules-ms).
func (p *Provider) SeedInBackground(ctx context.Context, baseURL string) {
	if baseURL == "" {
		log.Printf("[INFO] LIVEOPS_URL not set: using the default ruleset")
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	go func() {
		delay := time.Second
		for {
			if err := p.Seed(ctx, client, baseURL); err == nil {
				rs := p.Current()
				log.Printf("[INFO] ruleset v%d loaded from liveops-rules-ms", rs.Version)
				return
			} else {
				log.Printf("[WARN] could not load the ruleset from liveops (%v); using v%d meanwhile", err, p.Current().Version)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < 30*time.Second {
				delay *= 2
			}
		}
	}()
}
