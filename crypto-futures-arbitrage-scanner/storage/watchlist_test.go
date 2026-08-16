package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteWatchlistSeedsDefaultsOnceAndPersistsOrderedReplacement(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "scanner.db")
	store, err := OpenSQLite(databasePath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defaults := []string{"BTCUSDT", "ETHUSDT", "XRPUSDT", "SOLUSDT", "COTIUSDT"}
	if err := store.SeedWatchlist(context.Background(), defaults); err != nil {
		t.Fatalf("SeedWatchlist: %v", err)
	}
	if got, err := store.ListWatchlist(context.Background()); err != nil || !reflect.DeepEqual(got, defaults) {
		t.Fatalf("ListWatchlist = %v, %v", got, err)
	}

	replacement := []string{"SOLUSDT", "BTCUSDT", "LINKUSDT"}
	if err := store.ReplaceWatchlist(context.Background(), replacement); err != nil {
		t.Fatalf("ReplaceWatchlist: %v", err)
	}
	if err := store.SeedWatchlist(context.Background(), defaults); err != nil {
		t.Fatalf("second SeedWatchlist: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenSQLite(databasePath)
	if err != nil {
		t.Fatalf("reopen SQLite: %v", err)
	}
	defer reopened.Close()
	if got, err := reopened.ListWatchlist(context.Background()); err != nil || !reflect.DeepEqual(got, replacement) {
		t.Fatalf("reopened ListWatchlist = %v, %v", got, err)
	}
}

func TestSQLiteWatchlistRejectsEmptyAndDuplicateSymbolsAtomically(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "scanner.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer store.Close()
	if err := store.SeedWatchlist(context.Background(), []string{"BTCUSDT"}); err != nil {
		t.Fatal(err)
	}

	for _, invalid := range [][]string{{}, {"ETHUSDT", "ETHUSDT"}} {
		if err := store.ReplaceWatchlist(context.Background(), invalid); err == nil {
			t.Fatalf("ReplaceWatchlist(%v) succeeded", invalid)
		}
	}
	if got, err := store.ListWatchlist(context.Background()); err != nil || !reflect.DeepEqual(got, []string{"BTCUSDT"}) {
		t.Fatalf("watchlist changed after rejected update: %v, %v", got, err)
	}
}
