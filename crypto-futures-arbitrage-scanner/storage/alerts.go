package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	AlertMarketAll     = "all"
	AlertMarketSpot    = "spot"
	AlertMarketMixed   = "mixed"
	AlertMarketFutures = "futures"
)

var ErrAlertRuleNotFound = errors.New("alert rule not found")

type AlertRuleInput struct {
	Name            string  `json:"name"`
	Symbol          string  `json:"symbol"`
	MarketMode      string  `json:"market_mode"`
	BuySource       string  `json:"buy_source"`
	SellSource      string  `json:"sell_source"`
	MinSpreadPct    float64 `json:"min_spread_pct"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Enabled         bool    `json:"enabled"`
	BrowserEnabled  bool    `json:"browser_enabled"`
}

type AlertRule struct {
	ID int64 `json:"id"`
	AlertRuleInput
	CreatedAtMS       int64  `json:"created_at_ms"`
	UpdatedAtMS       int64  `json:"updated_at_ms"`
	LastTriggeredAtMS *int64 `json:"last_triggered_at_ms"`
}

type AlertObservation struct {
	Symbol         string
	BuySource      string
	SellSource     string
	BuyPrice       float64
	SellPrice      float64
	GrossSpreadPct float64
	ObservedAtMS   int64
}

type AlertTrigger struct {
	ID             int64   `json:"id"`
	RuleID         int64   `json:"rule_id"`
	RuleName       string  `json:"rule_name"`
	Symbol         string  `json:"symbol"`
	BuySource      string  `json:"buy_source"`
	SellSource     string  `json:"sell_source"`
	BuyPrice       float64 `json:"buy_price"`
	SellPrice      float64 `json:"sell_price"`
	GrossSpreadPct float64 `json:"gross_spread_pct"`
	TriggeredAtMS  int64   `json:"triggered_at_ms"`
}

type AlertStore interface {
	ListAlertRules(context.Context) ([]AlertRule, error)
	CreateAlertRule(context.Context, AlertRuleInput, int64) (AlertRule, error)
	UpdateAlertRule(context.Context, int64, AlertRuleInput, int64) (AlertRule, error)
	ListAlertTriggers(context.Context, int) ([]AlertTrigger, error)
	EvaluateAlerts(context.Context, AlertObservation) ([]AlertTrigger, error)
}

type unavailableAlertStore struct{ err error }

func NewUnavailableAlerts(err error) AlertStore { return &unavailableAlertStore{err: err} }

func (s *unavailableAlertStore) ListAlertRules(context.Context) ([]AlertRule, error) {
	return nil, s.err
}
func (s *unavailableAlertStore) CreateAlertRule(context.Context, AlertRuleInput, int64) (AlertRule, error) {
	return AlertRule{}, s.err
}
func (s *unavailableAlertStore) UpdateAlertRule(context.Context, int64, AlertRuleInput, int64) (AlertRule, error) {
	return AlertRule{}, s.err
}
func (s *unavailableAlertStore) ListAlertTriggers(context.Context, int) ([]AlertTrigger, error) {
	return nil, s.err
}
func (s *unavailableAlertStore) EvaluateAlerts(context.Context, AlertObservation) ([]AlertTrigger, error) {
	return nil, s.err
}

func normalizeAlertRuleInput(input AlertRuleInput) AlertRuleInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Symbol = strings.ToUpper(strings.TrimSpace(input.Symbol))
	input.MarketMode = strings.ToLower(strings.TrimSpace(input.MarketMode))
	input.BuySource = strings.ToLower(strings.TrimSpace(input.BuySource))
	input.SellSource = strings.ToLower(strings.TrimSpace(input.SellSource))
	return input
}

func ValidateAlertRuleInput(input AlertRuleInput) error {
	if input.Name == "" || len(input.Name) > 100 {
		return errors.New("alert rule name must be 1 to 100 characters")
	}
	switch input.MarketMode {
	case AlertMarketAll, AlertMarketSpot, AlertMarketMixed, AlertMarketFutures:
	default:
		return errors.New("unsupported alert market mode")
	}
	if math.IsNaN(input.MinSpreadPct) || math.IsInf(input.MinSpreadPct, 0) || input.MinSpreadPct < 0 || input.MinSpreadPct > 1000 {
		return errors.New("alert spread must be between 0 and 1000")
	}
	if input.CooldownSeconds < 1 || input.CooldownSeconds > 86_400 {
		return errors.New("alert cooldown must be between 1 and 86400 seconds")
	}
	if input.BuySource != "" && input.BuySource == input.SellSource {
		return errors.New("alert buy and sell sources must differ")
	}
	return nil
}

func scanAlertRule(scanner interface{ Scan(...any) error }) (AlertRule, error) {
	var rule AlertRule
	var enabled int
	var browserEnabled int
	var lastTriggered sql.NullInt64
	err := scanner.Scan(
		&rule.ID, &rule.Name, &rule.Symbol, &rule.MarketMode, &rule.BuySource, &rule.SellSource,
		&rule.MinSpreadPct, &rule.CooldownSeconds, &enabled, &browserEnabled,
		&rule.CreatedAtMS, &rule.UpdatedAtMS, &lastTriggered,
	)
	if err != nil {
		return AlertRule{}, err
	}
	rule.Enabled = enabled != 0
	rule.BrowserEnabled = browserEnabled != 0
	if lastTriggered.Valid {
		value := lastTriggered.Int64
		rule.LastTriggeredAtMS = &value
	}
	return rule, nil
}

const alertRuleColumns = `id, name, symbol, market_mode, buy_source, sell_source,
	min_spread_pct, cooldown_seconds, enabled, browser_enabled,
	created_at_ms, updated_at_ms, last_triggered_at_ms`

func (s *SQLiteStore) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+alertRuleColumns+` FROM alert_rules ORDER BY created_at_ms DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()
	rules := make([]AlertRule, 0)
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}
	return rules, nil
}

func (s *SQLiteStore) CreateAlertRule(ctx context.Context, raw AlertRuleInput, nowMS int64) (AlertRule, error) {
	input := normalizeAlertRuleInput(raw)
	if err := ValidateAlertRuleInput(input); err != nil {
		return AlertRule{}, err
	}
	if nowMS <= 0 {
		return AlertRule{}, errors.New("alert rule timestamp must be positive")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO alert_rules (
		name, symbol, market_mode, buy_source, sell_source, min_spread_pct,
		cooldown_seconds, enabled, browser_enabled, created_at_ms, updated_at_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Name, input.Symbol, input.MarketMode, input.BuySource, input.SellSource,
		input.MinSpreadPct, input.CooldownSeconds, input.Enabled, input.BrowserEnabled, nowMS, nowMS,
	)
	if err != nil {
		return AlertRule{}, fmt.Errorf("create alert rule: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return AlertRule{}, fmt.Errorf("read alert rule id: %w", err)
	}
	return AlertRule{ID: id, AlertRuleInput: input, CreatedAtMS: nowMS, UpdatedAtMS: nowMS}, nil
}

func (s *SQLiteStore) UpdateAlertRule(ctx context.Context, id int64, raw AlertRuleInput, nowMS int64) (AlertRule, error) {
	input := normalizeAlertRuleInput(raw)
	if id <= 0 || nowMS <= 0 {
		return AlertRule{}, errors.New("alert rule id and timestamp must be positive")
	}
	if err := ValidateAlertRuleInput(input); err != nil {
		return AlertRule{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE alert_rules SET
		name = ?, symbol = ?, market_mode = ?, buy_source = ?, sell_source = ?, min_spread_pct = ?,
		cooldown_seconds = ?, enabled = ?, browser_enabled = ?, updated_at_ms = ? WHERE id = ?`,
		input.Name, input.Symbol, input.MarketMode, input.BuySource, input.SellSource, input.MinSpreadPct,
		input.CooldownSeconds, input.Enabled, input.BrowserEnabled, nowMS, id,
	)
	if err != nil {
		return AlertRule{}, fmt.Errorf("update alert rule: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return AlertRule{}, fmt.Errorf("read updated alert rule count: %w", err)
	}
	if count == 0 {
		return AlertRule{}, ErrAlertRuleNotFound
	}
	rule, err := scanAlertRule(s.db.QueryRowContext(ctx, `SELECT `+alertRuleColumns+` FROM alert_rules WHERE id = ?`, id))
	if err != nil {
		return AlertRule{}, fmt.Errorf("read updated alert rule: %w", err)
	}
	return rule, nil
}

func alertObservationMarket(observation AlertObservation) string {
	buySpot := strings.HasSuffix(observation.BuySource, "_spot")
	sellSpot := strings.HasSuffix(observation.SellSource, "_spot")
	buyFutures := strings.HasSuffix(observation.BuySource, "_futures")
	sellFutures := strings.HasSuffix(observation.SellSource, "_futures")
	if buySpot && sellSpot {
		return AlertMarketSpot
	}
	if buyFutures && sellFutures {
		return AlertMarketFutures
	}
	return AlertMarketMixed
}

func validateAlertObservation(observation AlertObservation) error {
	if observation.Symbol == "" || observation.BuySource == "" || observation.SellSource == "" {
		return errors.New("alert observation route is required")
	}
	if observation.BuySource == observation.SellSource || observation.BuyPrice <= 0 || observation.SellPrice <= 0 || observation.ObservedAtMS <= 0 {
		return errors.New("invalid alert observation")
	}
	if math.IsNaN(observation.GrossSpreadPct) || math.IsInf(observation.GrossSpreadPct, 0) {
		return errors.New("alert observation spread must be finite")
	}
	return nil
}

func ruleMatchesObservation(rule AlertRule, observation AlertObservation) bool {
	if !rule.Enabled || (rule.Symbol != "" && rule.Symbol != observation.Symbol) {
		return false
	}
	if rule.MarketMode != AlertMarketAll && rule.MarketMode != alertObservationMarket(observation) {
		return false
	}
	if rule.BuySource != "" && rule.BuySource != observation.BuySource {
		return false
	}
	if rule.SellSource != "" && rule.SellSource != observation.SellSource {
		return false
	}
	if observation.GrossSpreadPct < rule.MinSpreadPct {
		return false
	}
	return rule.LastTriggeredAtMS == nil || observation.ObservedAtMS-*rule.LastTriggeredAtMS >= int64(rule.CooldownSeconds)*1_000
}

func (s *SQLiteStore) EvaluateAlerts(ctx context.Context, observation AlertObservation) ([]AlertTrigger, error) {
	if err := validateAlertObservation(observation); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin alert evaluation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT `+alertRuleColumns+` FROM alert_rules WHERE enabled = 1 AND min_spread_pct <= ?`, observation.GrossSpreadPct)
	if err != nil {
		return nil, fmt.Errorf("select alert rules: %w", err)
	}
	rules := make([]AlertRule, 0)
	for rows.Next() {
		rule, scanErr := scanAlertRule(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan alert evaluation rule: %w", scanErr)
		}
		if ruleMatchesObservation(rule, observation) {
			rules = append(rules, rule)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate alert evaluation rules: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close alert rule rows: %w", err)
	}

	triggers := make([]AlertTrigger, 0, len(rules))
	for _, rule := range rules {
		result, err := tx.ExecContext(ctx, `INSERT INTO alert_triggers (
			rule_id, rule_name, symbol, buy_source, sell_source, buy_price, sell_price, gross_spread_pct, triggered_at_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rule.ID, rule.Name, observation.Symbol, observation.BuySource, observation.SellSource,
			observation.BuyPrice, observation.SellPrice, observation.GrossSpreadPct, observation.ObservedAtMS,
		)
		if err != nil {
			return nil, fmt.Errorf("insert alert trigger: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("read alert trigger id: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE alert_rules SET last_triggered_at_ms = ? WHERE id = ?`, observation.ObservedAtMS, rule.ID); err != nil {
			return nil, fmt.Errorf("update alert cooldown: %w", err)
		}
		triggers = append(triggers, AlertTrigger{
			ID: id, RuleID: rule.ID, RuleName: rule.Name, Symbol: observation.Symbol,
			BuySource: observation.BuySource, SellSource: observation.SellSource,
			BuyPrice: observation.BuyPrice, SellPrice: observation.SellPrice,
			GrossSpreadPct: observation.GrossSpreadPct, TriggeredAtMS: observation.ObservedAtMS,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit alert evaluation: %w", err)
	}
	return triggers, nil
}

func (s *SQLiteStore) ListAlertTriggers(ctx context.Context, limit int) ([]AlertTrigger, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, rule_id, rule_name, symbol, buy_source, sell_source,
		buy_price, sell_price, gross_spread_pct, triggered_at_ms
		FROM alert_triggers ORDER BY triggered_at_ms DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list alert triggers: %w", err)
	}
	defer rows.Close()
	triggers := make([]AlertTrigger, 0)
	for rows.Next() {
		var trigger AlertTrigger
		if err := rows.Scan(&trigger.ID, &trigger.RuleID, &trigger.RuleName, &trigger.Symbol,
			&trigger.BuySource, &trigger.SellSource, &trigger.BuyPrice, &trigger.SellPrice,
			&trigger.GrossSpreadPct, &trigger.TriggeredAtMS); err != nil {
			return nil, fmt.Errorf("scan alert trigger: %w", err)
		}
		triggers = append(triggers, trigger)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert triggers: %w", err)
	}
	return triggers, nil
}
