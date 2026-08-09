package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "scanner.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store
}

func observation(symbol, buySource, sellSource string, spread float64, timestamp int64) Observation {
	return Observation{
		Symbol: symbol, BuySource: buySource, SellSource: sellSource,
		BuyPrice: 0.01131, SellPrice: 0.01140723, GrossSpreadPct: spread, ObservedAtMS: timestamp,
	}
}

func TestSQLiteStoreAggregatesOneOpenRouteSession(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.Observe(ctx, observation("COTIUSDT", "gate_futures", "binance_spot", 0.80, 1_000)); err != nil {
		t.Fatalf("first Observe: %v", err)
	}
	second := observation("COTIUSDT", "gate_futures", "binance_spot", 0.62, 5_000)
	second.BuyPrice = 0.01132
	if err := store.Observe(ctx, second); err != nil {
		t.Fatalf("second Observe: %v", err)
	}

	items, err := store.List(ctx, Query{Symbol: "COTIUSDT", Limit: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.StartedAtMS != 1_000 || item.LastSeenAtMS != 5_000 {
		t.Fatalf("session range = %d..%d, want 1000..5000", item.StartedAtMS, item.LastSeenAtMS)
	}
	if item.FirstSpreadPct != 0.80 || item.LatestSpreadPct != 0.62 || item.PeakSpreadPct != 0.80 {
		t.Fatalf("spread values = first %.2f latest %.2f peak %.2f", item.FirstSpreadPct, item.LatestSpreadPct, item.PeakSpreadPct)
	}
	if item.BuyPrice != 0.01132 {
		t.Fatalf("buy price = %.8f, want latest %.8f", item.BuyPrice, 0.01132)
	}
}

func TestSQLiteStoreBoundsTheWALJournal(t *testing.T) {
	store := openTestStore(t)
	var limit int64
	if err := store.db.QueryRow(`PRAGMA journal_size_limit`).Scan(&limit); err != nil {
		t.Fatal(err)
	}
	if limit != sqliteJournalSizeLimitBytes {
		t.Fatalf("journal_size_limit = %d, want %d", limit, sqliteJournalSizeLimitBytes)
	}
}

func TestCheckpointOutcomeRejectsABusyWAL(t *testing.T) {
	if err := checkpointOutcomeError(1, 120, 80); err == nil {
		t.Fatal("busy WAL checkpoint was accepted")
	}
	if err := checkpointOutcomeError(0, 0, 0); err != nil {
		t.Fatalf("successful WAL checkpoint returned %v", err)
	}
}

func TestSQLiteStoreObservesABatchAcrossRoutes(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	items := []Observation{
		observation("COTIUSDT", "gate_spot", "binance_spot", 0.8, 1_000),
		observation("COTIUSDT", "gate_spot", "binance_spot", 0.9, 2_000),
		observation("BTCUSDT", "gate_futures", "binance_futures", 0.4, 2_000),
	}

	if err := store.ObserveBatch(ctx, items); err != nil {
		t.Fatalf("ObserveBatch: %v", err)
	}
	stored, err := store.List(ctx, Query{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored routes = %d, want 2", len(stored))
	}
	for _, item := range stored {
		if item.Symbol == "COTIUSDT" && (item.FirstSpreadPct != 0.8 || item.PeakSpreadPct != 0.9) {
			t.Fatalf("batched COTI route = %+v", item)
		}
	}
}

func TestSQLiteStoreClosesStaleSessionsAndPrunesRetention(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.Observe(ctx, observation("COTIUSDT", "gate_futures", "binance_spot", 0.80, 1_000)); err != nil {
		t.Fatal(err)
	}
	if err := store.Observe(ctx, observation("BTCUSDT", "binance_spot", "bybit_futures", 0.20, 20_000)); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseStale(ctx, 10_000); err != nil {
		t.Fatalf("CloseStale: %v", err)
	}

	items, err := store.List(ctx, Query{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if items[1].EndedAtMS == nil || *items[1].EndedAtMS != 1_000 {
		t.Fatalf("old route ended_at = %v, want 1000", items[1].EndedAtMS)
	}
	if items[0].EndedAtMS != nil {
		t.Fatalf("fresh route ended unexpectedly at %v", *items[0].EndedAtMS)
	}

	if err := store.Prune(ctx, 10_000); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	items, err = store.List(ctx, Query{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Symbol != "BTCUSDT" {
		t.Fatalf("remaining items = %+v, want only BTCUSDT", items)
	}
}

func TestSQLiteStoreFiltersAndCapsListQueries(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	for _, item := range []Observation{
		observation("COTIUSDT", "gate_futures", "binance_spot", 0.90, 3_000),
		observation("COTIUSDT", "kraken_futures", "binance_spot", 0.40, 2_000),
		observation("BTCUSDT", "binance_spot", "bybit_futures", 0.70, 1_000),
	} {
		if err := store.Observe(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	items, err := store.List(ctx, Query{Symbol: "COTIUSDT", MinSpread: 0.5, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].PeakSpreadPct != 0.90 {
		t.Fatalf("filtered items = %+v", items)
	}
}

func TestSQLiteStoreLimitsAfterSelectingTheLatestSessionPerRoute(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	for _, timestamp := range []int64{1_000, 3_000, 4_000} {
		if err := store.Observe(ctx, observation("COTIUSDT", "gate_spot", "binance_spot", 0.8, timestamp)); err != nil {
			t.Fatal(err)
		}
		if err := store.CloseStale(ctx, timestamp+1); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Observe(ctx, observation("COTIUSDT", "bybit_spot", "binance_spot", 0.7, 2_000)); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseStale(ctx, 2_001); err != nil {
		t.Fatal(err)
	}

	items, err := store.List(ctx, Query{Symbol: "COTIUSDT", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want one latest session for each route", items)
	}
	routes := map[string]bool{}
	for _, item := range items {
		routes[item.BuySource+"->"+item.SellSource] = true
	}
	if !routes["gate_spot->binance_spot"] || !routes["bybit_spot->binance_spot"] {
		t.Fatalf("routes = %+v, want both distinct routes", routes)
	}
}

func TestOpenSQLiteClosesSessionsLeftOpenByPreviousProcess(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "scanner.db")
	ctx := context.Background()
	first, err := OpenSQLite(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Observe(ctx, observation("COTIUSDT", "gate_futures", "binance_spot", 0.80, 1_000)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := OpenSQLite(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	items, err := second.List(ctx, Query{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].EndedAtMS == nil || *items[0].EndedAtMS != items[0].LastSeenAtMS {
		t.Fatalf("startup did not close previous session: %+v", items[0])
	}
}

func TestUnavailableStoreReportsDegradationWithoutClosingError(t *testing.T) {
	want := context.DeadlineExceeded
	store := NewUnavailable(want)
	if err := store.Health(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Health error = %v, want %v", err, want)
	}
	if _, err := store.List(context.Background(), Query{}); !errors.Is(err, want) {
		t.Fatalf("List error = %v, want %v", err, want)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}
}
