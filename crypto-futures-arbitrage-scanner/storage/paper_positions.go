package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
)

const (
	PaperPositionOpen   = "open"
	PaperPositionClosed = "closed"
)

var (
	ErrPaperPositionNotFound = errors.New("paper position not found")
	ErrPaperPositionClosed   = errors.New("paper position is already closed")
	paperContractPattern     = regexp.MustCompile(`^[A-Z0-9]{1,24}_USDT$`)
)

type PaperPositionInput struct {
	Contract      string  `json:"contract"`
	Direction     string  `json:"direction"`
	Strategy      string  `json:"strategy"`
	Interval      string  `json:"interval"`
	EntryPrice    float64 `json:"entry_price"`
	StopPrice     float64 `json:"stop_price"`
	TargetPrice   float64 `json:"target_price"`
	Contracts     float64 `json:"contracts"`
	BaseQuantity  float64 `json:"base_quantity"`
	Notional      float64 `json:"notional"`
	RiskAmount    float64 `json:"risk_amount"`
	EstimatedFees float64 `json:"estimated_fees"`
	AnalysisAtMS  int64   `json:"analysis_at_ms"`
}

type PaperPosition struct {
	ID int64 `json:"id"`
	PaperPositionInput
	Status      string  `json:"status"`
	OpenedAtMS  int64   `json:"opened_at_ms"`
	ClosedAtMS  *int64  `json:"closed_at_ms"`
	ExitPrice   float64 `json:"exit_price"`
	RealizedPnL float64 `json:"realized_pnl"`
}

type PaperPositionStore interface {
	CreatePaperPosition(context.Context, PaperPositionInput, int64) (PaperPosition, error)
	ListPaperPositions(context.Context, int) ([]PaperPosition, error)
	ClosePaperPosition(context.Context, int64, float64, int64) (PaperPosition, error)
}

func ValidatePaperPositionInput(input PaperPositionInput) error {
	if !paperContractPattern.MatchString(input.Contract) {
		return errors.New("invalid contract")
	}
	if input.Direction != "long" && input.Direction != "short" {
		return errors.New("invalid direction")
	}
	switch input.Strategy {
	case "trend_pullback", "breakout", "mean_reversion":
	default:
		return errors.New("invalid strategy")
	}
	switch input.Interval {
	case "15m", "1h", "4h":
	default:
		return errors.New("invalid interval")
	}
	for _, value := range []float64{
		input.EntryPrice, input.StopPrice, input.TargetPrice, input.Contracts,
		input.BaseQuantity, input.Notional, input.RiskAmount,
	} {
		if !paperFinitePositive(value) {
			return errors.New("paper position values must be positive and finite")
		}
	}
	if input.EstimatedFees < 0 || math.IsNaN(input.EstimatedFees) || math.IsInf(input.EstimatedFees, 0) {
		return errors.New("estimated fees must be finite and non-negative")
	}
	if input.AnalysisAtMS <= 0 {
		return errors.New("analysis timestamp must be positive")
	}
	if input.Direction == "long" && !(input.StopPrice < input.EntryPrice && input.TargetPrice > input.EntryPrice) {
		return errors.New("long price levels are invalid")
	}
	if input.Direction == "short" && !(input.StopPrice > input.EntryPrice && input.TargetPrice < input.EntryPrice) {
		return errors.New("short price levels are invalid")
	}
	return nil
}

func paperFinitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (s *SQLiteStore) CreatePaperPosition(ctx context.Context, input PaperPositionInput, openedAtMS int64) (PaperPosition, error) {
	if err := ValidatePaperPositionInput(input); err != nil {
		return PaperPosition{}, err
	}
	if openedAtMS <= 0 {
		return PaperPosition{}, errors.New("open timestamp must be positive")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO paper_positions (
		contract, direction, strategy, interval, entry_price, stop_price, target_price,
		contracts, base_quantity, notional, risk_amount, estimated_fees, analysis_at_ms,
		status, opened_at_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Contract, input.Direction, input.Strategy, input.Interval,
		input.EntryPrice, input.StopPrice, input.TargetPrice, input.Contracts,
		input.BaseQuantity, input.Notional, input.RiskAmount, input.EstimatedFees,
		input.AnalysisAtMS, PaperPositionOpen, openedAtMS,
	)
	if err != nil {
		return PaperPosition{}, fmt.Errorf("create paper position: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return PaperPosition{}, fmt.Errorf("read paper position ID: %w", err)
	}
	return s.paperPositionByID(ctx, id)
}

type paperPositionScanner interface {
	Scan(...any) error
}

func scanPaperPosition(scanner paperPositionScanner) (PaperPosition, error) {
	var item PaperPosition
	var closedAt sql.NullInt64
	err := scanner.Scan(
		&item.ID, &item.Contract, &item.Direction, &item.Strategy, &item.Interval,
		&item.EntryPrice, &item.StopPrice, &item.TargetPrice, &item.Contracts,
		&item.BaseQuantity, &item.Notional, &item.RiskAmount, &item.EstimatedFees,
		&item.AnalysisAtMS, &item.Status, &item.OpenedAtMS, &closedAt,
		&item.ExitPrice, &item.RealizedPnL,
	)
	if err != nil {
		return PaperPosition{}, err
	}
	if closedAt.Valid {
		item.ClosedAtMS = &closedAt.Int64
	}
	return item, nil
}

const paperPositionColumns = `id, contract, direction, strategy, interval,
	entry_price, stop_price, target_price, contracts, base_quantity, notional,
	risk_amount, estimated_fees, analysis_at_ms, status, opened_at_ms, closed_at_ms,
	exit_price, realized_pnl`

func (s *SQLiteStore) paperPositionByID(ctx context.Context, id int64) (PaperPosition, error) {
	item, err := scanPaperPosition(s.db.QueryRowContext(ctx,
		`SELECT `+paperPositionColumns+` FROM paper_positions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return PaperPosition{}, ErrPaperPositionNotFound
	}
	if err != nil {
		return PaperPosition{}, fmt.Errorf("read paper position: %w", err)
	}
	return item, nil
}

func (s *SQLiteStore) ListPaperPositions(ctx context.Context, limit int) ([]PaperPosition, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+paperPositionColumns+`
		FROM paper_positions ORDER BY opened_at_ms DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list paper positions: %w", err)
	}
	defer rows.Close()
	items := make([]PaperPosition, 0)
	for rows.Next() {
		item, scanErr := scanPaperPosition(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan paper position: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list paper positions: %w", err)
	}
	return items, nil
}

func (s *SQLiteStore) ClosePaperPosition(ctx context.Context, id int64, exitPrice float64, closedAtMS int64) (PaperPosition, error) {
	if id <= 0 || !paperFinitePositive(exitPrice) || closedAtMS <= 0 {
		return PaperPosition{}, errors.New("invalid paper close request")
	}
	item, err := s.paperPositionByID(ctx, id)
	if err != nil {
		return PaperPosition{}, err
	}
	if item.Status != PaperPositionOpen {
		return PaperPosition{}, ErrPaperPositionClosed
	}
	realizedPnL := (exitPrice - item.EntryPrice) * item.BaseQuantity
	if item.Direction == "short" {
		realizedPnL = (item.EntryPrice - exitPrice) * item.BaseQuantity
	}
	realizedPnL -= item.EstimatedFees
	result, err := s.db.ExecContext(ctx, `UPDATE paper_positions
		SET status = ?, closed_at_ms = ?, exit_price = ?, realized_pnl = ?
		WHERE id = ? AND status = ?`,
		PaperPositionClosed, closedAtMS, exitPrice, realizedPnL, id, PaperPositionOpen,
	)
	if err != nil {
		return PaperPosition{}, fmt.Errorf("close paper position: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return PaperPosition{}, fmt.Errorf("close paper position: %w", err)
	}
	if affected == 0 {
		return PaperPosition{}, ErrPaperPositionClosed
	}
	return s.paperPositionByID(ctx, id)
}

type unavailablePaperPositionStore struct {
	err error
}

func NewUnavailablePaperPositions(err error) PaperPositionStore {
	return &unavailablePaperPositionStore{err: err}
}

func (s *unavailablePaperPositionStore) CreatePaperPosition(context.Context, PaperPositionInput, int64) (PaperPosition, error) {
	return PaperPosition{}, s.err
}

func (s *unavailablePaperPositionStore) ListPaperPositions(context.Context, int) ([]PaperPosition, error) {
	return nil, s.err
}

func (s *unavailablePaperPositionStore) ClosePaperPosition(context.Context, int64, float64, int64) (PaperPosition, error) {
	return PaperPosition{}, s.err
}
