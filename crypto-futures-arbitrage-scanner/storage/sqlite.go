package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

const sqliteJournalSizeLimitBytes int64 = 64 << 20

func OpenSQLite(databasePath string) (*SQLiteStore, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, errors.New("database path is required")
	}
	if databasePath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		fmt.Sprintf(`PRAGMA journal_size_limit = %d`, sqliteJournalSizeLimitBytes),
		`CREATE TABLE IF NOT EXISTS opportunities (
			id INTEGER PRIMARY KEY,
			symbol TEXT NOT NULL,
			buy_source TEXT NOT NULL,
			sell_source TEXT NOT NULL,
			buy_price REAL NOT NULL,
			sell_price REAL NOT NULL,
			first_spread_pct REAL NOT NULL,
			latest_spread_pct REAL NOT NULL,
			peak_spread_pct REAL NOT NULL,
			started_at_ms INTEGER NOT NULL,
			last_seen_at_ms INTEGER NOT NULL,
			ended_at_ms INTEGER
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS opportunities_open_route
			ON opportunities(symbol, buy_source, sell_source)
			WHERE ended_at_ms IS NULL`,
		`CREATE INDEX IF NOT EXISTS opportunities_recent
			ON opportunities(last_seen_at_ms DESC)`,
		`CREATE INDEX IF NOT EXISTS opportunities_symbol_peak
			ON opportunities(symbol, peak_spread_pct DESC)`,
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			symbol TEXT NOT NULL DEFAULT '',
			market_mode TEXT NOT NULL,
			buy_source TEXT NOT NULL DEFAULT '',
			sell_source TEXT NOT NULL DEFAULT '',
			min_spread_pct REAL NOT NULL,
			cooldown_seconds INTEGER NOT NULL,
			enabled INTEGER NOT NULL,
			browser_enabled INTEGER NOT NULL,
			created_at_ms INTEGER NOT NULL,
			updated_at_ms INTEGER NOT NULL,
			last_triggered_at_ms INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS alert_rules_enabled
			ON alert_rules(enabled, symbol, min_spread_pct)`,
		`CREATE TABLE IF NOT EXISTS alert_triggers (
			id INTEGER PRIMARY KEY,
			rule_id INTEGER NOT NULL,
			rule_name TEXT NOT NULL,
			symbol TEXT NOT NULL,
			buy_source TEXT NOT NULL,
			sell_source TEXT NOT NULL,
			buy_price REAL NOT NULL,
			sell_price REAL NOT NULL,
			gross_spread_pct REAL NOT NULL,
			triggered_at_ms INTEGER NOT NULL,
			FOREIGN KEY(rule_id) REFERENCES alert_rules(id)
		)`,
		`CREATE INDEX IF NOT EXISTS alert_triggers_recent
			ON alert_triggers(triggered_at_ms DESC, id DESC)`,
		`CREATE TABLE IF NOT EXISTS watchlist_symbols (
			symbol TEXT PRIMARY KEY,
			position INTEGER NOT NULL UNIQUE,
			created_at_ms INTEGER NOT NULL DEFAULT (unixepoch('subsec') * 1000)
		)`,
		`UPDATE opportunities SET ended_at_ms = last_seen_at_ms WHERE ended_at_ms IS NULL`,
	}

	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	if err := s.checkpointWAL(ctx); err != nil {
		return fmt.Errorf("initialize sqlite: %w", err)
	}
	return nil
}

func checkpointOutcomeError(busy, logFrames, checkpointedFrames int) error {
	if busy != 0 {
		return fmt.Errorf(
			"WAL checkpoint blocked: busy=%d log_frames=%d checkpointed_frames=%d",
			busy,
			logFrames,
			checkpointedFrames,
		)
	}
	return nil
}

func (s *SQLiteStore) checkpointWAL(ctx context.Context) error {
	var busy int
	var logFrames int
	var checkpointedFrames int
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(
		&busy,
		&logFrames,
		&checkpointedFrames,
	); err != nil {
		return fmt.Errorf("checkpoint WAL: %w", err)
	}
	return checkpointOutcomeError(busy, logFrames, checkpointedFrames)
}

func validateObservation(observation Observation) error {
	if observation.Symbol == "" || observation.BuySource == "" || observation.SellSource == "" {
		return errors.New("symbol and route sources are required")
	}
	if observation.BuySource == observation.SellSource {
		return errors.New("buy and sell sources must differ")
	}
	if observation.BuyPrice <= 0 || observation.SellPrice <= 0 || observation.ObservedAtMS <= 0 {
		return errors.New("prices and observation timestamp must be positive")
	}
	if math.IsNaN(observation.GrossSpreadPct) || math.IsInf(observation.GrossSpreadPct, 0) {
		return errors.New("spread must be finite")
	}
	return nil
}

func (s *SQLiteStore) Observe(ctx context.Context, observation Observation) error {
	return s.ObserveBatch(ctx, []Observation{observation})
}

func (s *SQLiteStore) ObserveBatch(ctx context.Context, observations []Observation) error {
	if len(observations) == 0 {
		return nil
	}
	for _, observation := range observations {
		if err := validateObservation(observation); err != nil {
			return err
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin observation batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, observation := range observations {
		if err := observeTx(ctx, tx, observation); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit observation batch: %w", err)
	}
	return nil
}

func observeTx(ctx context.Context, tx *sql.Tx, observation Observation) error {
	var id int64
	var peakSpread float64
	var lastSeen int64
	err := tx.QueryRowContext(
		ctx,
		`SELECT id, peak_spread_pct, last_seen_at_ms
		 FROM opportunities
		 WHERE symbol = ? AND buy_source = ? AND sell_source = ? AND ended_at_ms IS NULL`,
		observation.Symbol,
		observation.BuySource,
		observation.SellSource,
	).Scan(&id, &peakSpread, &lastSeen)

	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO opportunities (
				symbol, buy_source, sell_source, buy_price, sell_price,
				first_spread_pct, latest_spread_pct, peak_spread_pct,
				started_at_ms, last_seen_at_ms, ended_at_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			observation.Symbol,
			observation.BuySource,
			observation.SellSource,
			observation.BuyPrice,
			observation.SellPrice,
			observation.GrossSpreadPct,
			observation.GrossSpreadPct,
			observation.GrossSpreadPct,
			observation.ObservedAtMS,
			observation.ObservedAtMS,
		)
	} else if err == nil {
		peakSpread = math.Max(peakSpread, observation.GrossSpreadPct)
		if observation.ObservedAtMS >= lastSeen {
			_, err = tx.ExecContext(
				ctx,
				`UPDATE opportunities
				 SET buy_price = ?, sell_price = ?, latest_spread_pct = ?, peak_spread_pct = ?, last_seen_at_ms = ?
				 WHERE id = ?`,
				observation.BuyPrice,
				observation.SellPrice,
				observation.GrossSpreadPct,
				peakSpread,
				observation.ObservedAtMS,
				id,
			)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE opportunities SET peak_spread_pct = ? WHERE id = ?`, peakSpread, id)
		}
	}
	if err != nil {
		return fmt.Errorf("observe opportunity: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CloseStale(ctx context.Context, cutoffMS int64) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE opportunities SET ended_at_ms = last_seen_at_ms
		 WHERE ended_at_ms IS NULL AND last_seen_at_ms < ?`,
		cutoffMS,
	)
	if err != nil {
		return fmt.Errorf("close stale opportunities: %w", err)
	}
	return nil
}

func (s *SQLiteStore) List(ctx context.Context, query Query) ([]Opportunity, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	statement := `WITH ranked AS (
		SELECT id, symbol, buy_source, sell_source, buy_price, sell_price,
		first_spread_pct, latest_spread_pct, peak_spread_pct,
		started_at_ms, last_seen_at_ms, ended_at_ms,
		ROW_NUMBER() OVER (
			PARTITION BY symbol, buy_source, sell_source
			ORDER BY last_seen_at_ms DESC, id DESC
		) AS route_rank
		FROM opportunities WHERE peak_spread_pct >= ?`
	args := []any{query.MinSpread}
	if query.Symbol != "" {
		statement += ` AND symbol = ?`
		args = append(args, query.Symbol)
	}
	statement += `)
		SELECT id, symbol, buy_source, sell_source, buy_price, sell_price,
		first_spread_pct, latest_spread_pct, peak_spread_pct,
		started_at_ms, last_seen_at_ms, ended_at_ms
		FROM ranked WHERE route_rank = 1
		ORDER BY last_seen_at_ms DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list opportunities: %w", err)
	}
	defer rows.Close()

	items := make([]Opportunity, 0)
	for rows.Next() {
		var item Opportunity
		var endedAt sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.Symbol,
			&item.BuySource,
			&item.SellSource,
			&item.BuyPrice,
			&item.SellPrice,
			&item.FirstSpreadPct,
			&item.LatestSpreadPct,
			&item.PeakSpreadPct,
			&item.StartedAtMS,
			&item.LastSeenAtMS,
			&endedAt,
		); err != nil {
			return nil, fmt.Errorf("scan opportunity: %w", err)
		}
		if endedAt.Valid {
			value := endedAt.Int64
			item.EndedAtMS = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate opportunities: %w", err)
	}
	return items, nil
}

func (s *SQLiteStore) Prune(ctx context.Context, cutoffMS int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM opportunities WHERE last_seen_at_ms < ?`, cutoffMS)
	if err != nil {
		return fmt.Errorf("prune opportunities: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
