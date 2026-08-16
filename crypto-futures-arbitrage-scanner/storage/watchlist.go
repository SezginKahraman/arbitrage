package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func validateWatchlistSymbols(symbols []string) error {
	if len(symbols) == 0 {
		return errors.New("watchlist must contain at least one symbol")
	}
	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" || strings.TrimSpace(symbol) != symbol {
			return errors.New("watchlist symbols must be normalized")
		}
		if _, exists := seen[symbol]; exists {
			return fmt.Errorf("duplicate watchlist symbol %q", symbol)
		}
		seen[symbol] = struct{}{}
	}
	return nil
}

func (s *SQLiteStore) SeedWatchlist(ctx context.Context, defaults []string) error {
	if err := validateWatchlistSymbols(defaults); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin watchlist seed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM watchlist_symbols`).Scan(&count); err != nil {
		return fmt.Errorf("count watchlist: %w", err)
	}
	if count == 0 {
		for position, symbol := range defaults {
			if _, err := tx.ExecContext(ctx, `INSERT INTO watchlist_symbols(symbol, position) VALUES (?, ?)`, symbol, position); err != nil {
				return fmt.Errorf("seed watchlist: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit watchlist seed: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListWatchlist(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT symbol FROM watchlist_symbols ORDER BY position ASC`)
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}
	defer rows.Close()
	result := make([]string, 0, 20)
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("scan watchlist: %w", err)
		}
		result = append(result, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}
	return result, nil
}

func (s *SQLiteStore) ReplaceWatchlist(ctx context.Context, symbols []string) error {
	if err := validateWatchlistSymbols(symbols); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin watchlist update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM watchlist_symbols`); err != nil {
		return fmt.Errorf("clear watchlist: %w", err)
	}
	for position, symbol := range symbols {
		if _, err := tx.ExecContext(ctx, `INSERT INTO watchlist_symbols(symbol, position) VALUES (?, ?)`, symbol, position); err != nil {
			return fmt.Errorf("replace watchlist: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit watchlist update: %w", err)
	}
	return nil
}
