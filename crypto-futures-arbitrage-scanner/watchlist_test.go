package main

import (
	"context"
	"reflect"
	"testing"
)

type fakeWatchlistRepository struct {
	symbols []string
	writes  [][]string
	err     error
}

func (f *fakeWatchlistRepository) ListWatchlist(context.Context) ([]string, error) {
	return append([]string(nil), f.symbols...), f.err
}

func (f *fakeWatchlistRepository) ReplaceWatchlist(_ context.Context, symbols []string) error {
	if f.err != nil {
		return f.err
	}
	f.symbols = append([]string(nil), symbols...)
	f.writes = append(f.writes, append([]string(nil), symbols...))
	return nil
}

type fakeMarketReader struct {
	candidates []marketCandidate
	states     []marketSourceState
}

func (f *fakeMarketReader) Candidates() []marketCandidate {
	return append([]marketCandidate(nil), f.candidates...)
}
func (f *fakeMarketReader) SourceStates() []marketSourceState {
	return append([]marketSourceState(nil), f.states...)
}
func (f *fakeMarketReader) Supports(symbol string) bool {
	for _, candidate := range f.candidates {
		if candidate.Symbol == symbol {
			return true
		}
	}
	return false
}

func TestWatchlistServiceValidatesAndPublishesCommittedUpdates(t *testing.T) {
	repository := &fakeWatchlistRepository{symbols: []string{"BTCUSDT"}}
	markets := &fakeMarketReader{candidates: []marketCandidate{
		{Symbol: "BTCUSDT", Sources: []string{"a", "b"}},
		{Symbol: "COTIUSDT", Sources: []string{"a", "b"}},
	}}
	var published []string
	service := newWatchlistService(repository, markets, func(symbols []string) { published = append([]string(nil), symbols...) })

	if err := service.Replace(context.Background(), []string{"cotiusdt", "BTCUSDT"}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !reflect.DeepEqual(repository.symbols, []string{"COTIUSDT", "BTCUSDT"}) || !reflect.DeepEqual(published, repository.symbols) {
		t.Fatalf("repository=%v published=%v", repository.symbols, published)
	}

	for _, invalid := range [][]string{{}, {"BTCUSDT", "BTCUSDT"}, {"NOTREALUSDT"}} {
		if err := service.Replace(context.Background(), invalid); err == nil {
			t.Fatalf("Replace(%v) succeeded", invalid)
		}
	}
	tooMany := make([]string, maxWatchlistSymbols+1)
	for index := range tooMany {
		tooMany[index] = "X" + string(rune('A'+index)) + "USDT"
	}
	if err := service.Replace(context.Background(), tooMany); err == nil {
		t.Fatal("oversized watchlist succeeded")
	}
	if len(repository.writes) != 1 {
		t.Fatalf("writes = %v, want only committed valid update", repository.writes)
	}
}
