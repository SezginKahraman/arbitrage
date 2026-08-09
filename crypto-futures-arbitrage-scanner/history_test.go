package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"futures-arbitrage-scanner/storage"
)

type recordingStore struct {
	observed chan storage.Observation
	batches  chan []storage.Observation
	block    chan struct{}
	closed   chan struct{}
	once     sync.Once
}

func (s *recordingStore) ObserveBatch(ctx context.Context, observations []storage.Observation) error {
	if s.batches != nil {
		copied := append([]storage.Observation(nil), observations...)
		s.batches <- copied
	}
	for _, observation := range observations {
		s.observed <- observation
	}
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *recordingStore) Observe(ctx context.Context, observation storage.Observation) error {
	s.observed <- observation
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func (s *recordingStore) CloseStale(context.Context, int64) error { return nil }
func (s *recordingStore) List(context.Context, storage.Query) ([]storage.Opportunity, error) {
	return nil, nil
}
func (s *recordingStore) Prune(context.Context, int64) error { return nil }
func (s *recordingStore) Health(context.Context) error       { return nil }
func (s *recordingStore) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func opportunityAt(spread float64, timestamp int64) ArbitrageOpportunity {
	return ArbitrageOpportunity{
		Symbol: "COTIUSDT", BuySource: "gate_futures", SellSource: "binance_spot",
		BuyPrice: 0.01131, SellPrice: 0.01140723, ProfitPct: spread, Timestamp: timestamp,
	}
}

func TestOpportunityHistoryPersistsNormalizedObservation(t *testing.T) {
	store := &recordingStore{observed: make(chan storage.Observation, 4), closed: make(chan struct{})}
	history := newOpportunityHistory(store, 4)
	t.Cleanup(func() { _ = history.Close(context.Background()) })

	if !history.Observe(opportunityAt(0.85, 10_000)) {
		t.Fatal("observation was unexpectedly rejected")
	}

	select {
	case got := <-store.observed:
		if got.Symbol != "COTIUSDT" || got.GrossSpreadPct != 0.85 || got.ObservedAtMS != 10_000 {
			t.Fatalf("stored observation = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("observation was not persisted")
	}
}

func TestOpportunityHistoryPersistsMultipleRoutesInOneTimedBatch(t *testing.T) {
	store := &recordingStore{
		observed: make(chan storage.Observation, 8),
		batches:  make(chan []storage.Observation, 2),
		closed:   make(chan struct{}),
	}
	history := newOpportunityHistory(store, 4)
	t.Cleanup(func() { _ = history.Close(context.Background()) })

	first := opportunityAt(0.85, 10_000)
	second := opportunityAt(0.65, 10_001)
	second.BuySource = "gate_spot"
	second.SellSource = "binance_spot"
	if !history.Observe(first) || !history.Observe(second) {
		t.Fatal("observations were unexpectedly rejected")
	}

	select {
	case batch := <-store.batches:
		routes := make(map[string]bool)
		for _, observation := range batch {
			routes[observationKey(observation)] = true
		}
		if len(routes) != 2 {
			t.Fatalf("batch routes = %+v, want both routes", routes)
		}
	case <-time.After(time.Second):
		t.Fatal("timed batch was not persisted")
	}
}

func TestOpportunityHistoryCoalescesPeakWithoutBlockingQuoteProcessing(t *testing.T) {
	block := make(chan struct{})
	store := &recordingStore{observed: make(chan storage.Observation, 8), block: block, closed: make(chan struct{})}
	history := newOpportunityHistory(store, 1)

	if !history.Observe(opportunityAt(0.5, 1_000)) {
		t.Fatal("first observation was rejected")
	}
	select {
	case <-store.observed:
	case <-time.After(time.Second):
		t.Fatal("worker did not start the first write")
	}

	started := time.Now()
	if !history.Observe(opportunityAt(0.9, 2_000)) || !history.Observe(opportunityAt(0.6, 3_000)) {
		t.Fatal("coalesced observations were rejected")
	}
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("coalescing blocked quote processing for %s", elapsed)
	}

	close(block)
	if err := history.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	var spreads []float64
	for len(store.observed) > 0 {
		spreads = append(spreads, (<-store.observed).GrossSpreadPct)
	}
	if len(spreads) != 2 || spreads[0] != 0.9 || spreads[1] != 0.6 {
		t.Fatalf("coalesced spreads after first write = %v, want [0.9 0.6]", spreads)
	}
	select {
	case <-store.closed:
	default:
		t.Fatal("store was not closed")
	}
}

func TestOpportunityHistoryCloseDrainsPendingObservation(t *testing.T) {
	store := &recordingStore{observed: make(chan storage.Observation, 2), closed: make(chan struct{})}
	history := newOpportunityHistory(store, 1)
	if !history.Observe(opportunityAt(0.7, 4_000)) {
		t.Fatal("observation was rejected")
	}
	if err := history.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-store.observed:
		if got.GrossSpreadPct != 0.7 {
			t.Fatalf("drained spread = %v", got.GrossSpreadPct)
		}
	default:
		t.Fatal("pending observation was not drained")
	}
	if history.Observe(opportunityAt(0.8, 5_000)) {
		t.Fatal("closed history accepted an observation")
	}
}

func TestPendingRoutePreservesFirstPeakAndLatestObservations(t *testing.T) {
	pending := pendingRoute{}
	for _, observation := range []storage.Observation{
		observationFrom(opportunityAt(0.5, 1_000)),
		observationFrom(opportunityAt(0.9, 2_000)),
		observationFrom(opportunityAt(0.6, 3_000)),
	} {
		pending = mergePendingRoute(pending, pendingRoute{
			earliest: observation,
			peak:     observation,
			latest:   observation,
		})
	}

	ordered := orderedObservations(pending)
	if len(ordered) != 3 {
		t.Fatalf("ordered observations = %+v", ordered)
	}
	for index, want := range []struct {
		spread    float64
		timestamp int64
	}{{0.5, 1_000}, {0.9, 2_000}, {0.6, 3_000}} {
		if ordered[index].GrossSpreadPct != want.spread || ordered[index].ObservedAtMS != want.timestamp {
			t.Fatalf("observation %d = %+v, want spread=%v timestamp=%d", index, ordered[index], want.spread, want.timestamp)
		}
	}
}

func TestOpportunityHistoryCloseUsesOneBoundedDrainContextAndClosesStore(t *testing.T) {
	store := &recordingStore{
		observed: make(chan storage.Observation, 8),
		block:    make(chan struct{}),
		closed:   make(chan struct{}),
	}
	history := newOpportunityHistory(store, 1)
	if !history.Observe(opportunityAt(0.5, 1_000)) {
		t.Fatal("observation was rejected")
	}
	select {
	case <-store.observed:
	case <-time.After(time.Second):
		t.Fatal("worker did not begin persistence")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := history.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("bounded close took %s", elapsed)
	}
	select {
	case <-store.closed:
	default:
		t.Fatal("store was not closed after bounded drain")
	}
}
