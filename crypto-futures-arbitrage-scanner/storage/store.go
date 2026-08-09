package storage

import "context"

type Observation struct {
	Symbol         string
	BuySource      string
	SellSource     string
	BuyPrice       float64
	SellPrice      float64
	GrossSpreadPct float64
	ObservedAtMS   int64
}

type Opportunity struct {
	ID              int64   `json:"id"`
	Symbol          string  `json:"symbol"`
	BuySource       string  `json:"buy_source"`
	SellSource      string  `json:"sell_source"`
	BuyPrice        float64 `json:"buy_price"`
	SellPrice       float64 `json:"sell_price"`
	FirstSpreadPct  float64 `json:"first_spread_pct"`
	LatestSpreadPct float64 `json:"latest_spread_pct"`
	PeakSpreadPct   float64 `json:"peak_spread_pct"`
	StartedAtMS     int64   `json:"started_at_ms"`
	LastSeenAtMS    int64   `json:"last_seen_at_ms"`
	EndedAtMS       *int64  `json:"ended_at_ms"`
}

type Query struct {
	Symbol    string
	MinSpread float64
	Limit     int
}

type OpportunityStore interface {
	Observe(context.Context, Observation) error
	CloseStale(context.Context, int64) error
	List(context.Context, Query) ([]Opportunity, error)
	Prune(context.Context, int64) error
	Health(context.Context) error
	Close() error
}

type unavailableStore struct {
	err error
}

func NewUnavailable(err error) OpportunityStore {
	return &unavailableStore{err: err}
}

func (s *unavailableStore) Observe(context.Context, Observation) error { return s.err }
func (s *unavailableStore) CloseStale(context.Context, int64) error    { return s.err }
func (s *unavailableStore) List(context.Context, Query) ([]Opportunity, error) {
	return nil, s.err
}
func (s *unavailableStore) Prune(context.Context, int64) error { return s.err }
func (s *unavailableStore) Health(context.Context) error       { return s.err }
func (s *unavailableStore) Close() error                       { return nil }
