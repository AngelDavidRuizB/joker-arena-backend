package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"jokerarena-game-ms/internal/models"
)

// repoContract exercises the Repository semantics shared by the memory and PostgreSQL
// implementations. Aliases/room codes carry a unique suffix so the same suite can run
// against a database that already contains data (it never deletes anything).
func repoContract(t *testing.T, repo Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)

	t.Run("register is idempotent and case-insensitive", func(t *testing.T) {
		alias := "Tester" + suffix
		p1, isNew, err := repo.RegisterPlayer(ctx, alias)
		if err != nil || !isNew {
			t.Fatalf("first register: isNew=%v err=%v", isNew, err)
		}
		if !models.IsValidPlayerID(p1.ID) || p1.Alias != alias {
			t.Fatalf("unexpected player %+v", p1)
		}
		p2, isNew, err := repo.RegisterPlayer(ctx, "TESTER"+suffix)
		if err != nil || isNew || p2.ID != p1.ID {
			t.Fatalf("second register: isNew=%v err=%v id=%s want %s", isNew, err, p2.ID, p1.ID)
		}
	})

	t.Run("concurrent registration yields a single player", func(t *testing.T) {
		alias := "Race" + suffix
		var wg sync.WaitGroup
		ids := make(chan string, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, _, err := repo.RegisterPlayer(ctx, alias)
				if err != nil {
					t.Errorf("register: %v", err)
					return
				}
				ids <- p.ID
			}()
		}
		wg.Wait()
		close(ids)
		seen := map[string]bool{}
		for id := range ids {
			seen[id] = true
		}
		if len(seen) != 1 {
			t.Fatalf("expected exactly one id, got %v", seen)
		}
	})

	t.Run("get player", func(t *testing.T) {
		p, _, _ := repo.RegisterPlayer(ctx, "Lookup"+suffix)
		got, err := repo.GetPlayerByID(ctx, p.ID)
		if err != nil || got.Alias != p.Alias {
			t.Fatalf("get: %v %+v", err, got)
		}
		if _, err := repo.GetPlayerByID(ctx, "999999999"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
		if _, err := repo.GetPlayerByID(ctx, "not-a-number"); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("want ErrInvalidID, got %v", err)
		}
	})

	t.Run("room with bot lifecycle", func(t *testing.T) {
		host, _, _ := repo.RegisterPlayer(ctx, "Host"+suffix)
		code := "JKR-T" + suffix[len(suffix)-3:]
		matchID, err := repo.CreateRoomMatch(ctx, code, 300, host.ID, true)
		if err != nil || matchID == "" {
			t.Fatalf("create: %v id=%q", err, matchID)
		}
		if _, err := repo.CreateRoomMatch(ctx, code, 300, host.ID, true); !errors.Is(err, ErrCodeTaken) {
			t.Fatalf("duplicate code: want ErrCodeTaken, got %v", err)
		}

		m, players, err := repo.GetMatchByCode(ctx, "jkr-t"+suffix[len(suffix)-3:])
		if err != nil {
			t.Fatalf("get match: %v", err)
		}
		if m.ID != matchID || m.TargetScore != 300 || !m.WithBot || m.Status != "WAITING" || m.HostID != host.ID {
			t.Fatalf("unexpected match %+v", m)
		}
		if len(players) != 2 || players[0] != host.ID {
			t.Fatalf("unexpected players %v", players)
		}

		if err := repo.MarkMatchInProgress(ctx, code); err != nil {
			t.Fatalf("in progress: %v", err)
		}
		if m, _, _ = repo.GetMatchByCode(ctx, code); m.Status != "IN_PROGRESS" {
			t.Fatalf("want IN_PROGRESS, got %s", m.Status)
		}
		if err := repo.FinishMatch(ctx, code, true); err != nil {
			t.Fatalf("finish: %v", err)
		}
		m, _, _ = repo.GetMatchByCode(ctx, code)
		if m.Status != "FINISHED" || m.Result != "WIN" || m.FinishedAt == nil {
			t.Fatalf("unexpected finished match %+v", m)
		}
	})

	t.Run("room without bot and custom target", func(t *testing.T) {
		host, _, _ := repo.RegisterPlayer(ctx, "Solo"+suffix)
		code := "JKR-S" + suffix[len(suffix)-3:]
		if _, err := repo.CreateRoomMatch(ctx, code, 450, host.ID, false); err != nil {
			t.Fatalf("create: %v", err)
		}
		m, players, err := repo.GetMatchByCode(ctx, code)
		if err != nil || m.WithBot || m.TargetScore != 450 || len(players) != 1 {
			t.Fatalf("unexpected: %v %+v %v", err, m, players)
		}
		if err := repo.FinishMatch(ctx, code, false); err != nil {
			t.Fatalf("finish: %v", err)
		}
		if m, _, _ = repo.GetMatchByCode(ctx, code); m.Result != "LOSE" {
			t.Fatalf("want LOSE, got %s", m.Result)
		}
	})

	t.Run("unknown room and invalid host", func(t *testing.T) {
		if _, _, err := repo.GetMatchByCode(ctx, "JKR-NOPE"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
		if _, err := repo.CreateRoomMatch(ctx, "JKR-BAD1", 300, "abc", false); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("want ErrInvalidID, got %v", err)
		}
	})
}

func TestMemoryRepositoryContract(t *testing.T) {
	repoContract(t, NewMemoryRepository())
}

// TestPostgresRepositoryContract runs against a real PostgreSQL when TEST_DATABASE_URL is set
// (for example the docker compose database). It only inserts uniquely named rows.
func TestPostgresRepositoryContract(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := NewPgxPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	repoContract(t, NewPostgresRepositoryFromPool(pool))
}

func TestDescriptionRoundTrip(t *testing.T) {
	target, bot := decodeRoomDescription(encodeRoomDescription(1234, true))
	if target != 1234 || !bot {
		t.Fatalf("got %d %v", target, bot)
	}
	target, bot = decodeRoomDescription("legacy text")
	if target != 300 || bot {
		t.Fatalf("defaults expected, got %d %v", target, bot)
	}
}

func TestPostgresRepositoryUnavailable(t *testing.T) {
	repo := &PostgresRepository{} // never connected
	if _, _, err := repo.RegisterPlayer(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if err := repo.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}
