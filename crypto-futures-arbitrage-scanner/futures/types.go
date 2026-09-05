package futures

import "context"

const (
	StrategyTrendPullback = "trend_pullback"
	StrategyBreakout      = "breakout"
	StrategyMeanReversion = "mean_reversion"
)

type ScenarioStatus string

const (
	ScenarioReady   ScenarioStatus = "ready"
	ScenarioWatch   ScenarioStatus = "watch"
	ScenarioNoTrade ScenarioStatus = "no_trade"
)

type Service interface {
	Contracts(context.Context) ([]Contract, error)
	Analyze(context.Context, AnalyzeRequest) (Analysis, error)
}

type LiveService interface {
	Live(context.Context, LiveRequest) (LiveSnapshot, error)
}

type AnalyzeRequest struct {
	Contract        string  `json:"contract"`
	Interval        string  `json:"interval"`
	Strategy        string  `json:"strategy"`
	AccountBalance  float64 `json:"account_balance"`
	RiskPercent     float64 `json:"risk_percent"`
	TradeNotional   float64 `json:"trade_notional"`
	MinNetSpreadPct float64 `json:"min_net_spread_pct"`
}

type Contract struct {
	Symbol           string  `json:"symbol"`
	Status           string  `json:"status"`
	QuantoMultiplier float64 `json:"quanto_multiplier"`
	MarkPrice        float64 `json:"mark_price"`
	IndexPrice       float64 `json:"index_price"`
	LastPrice        float64 `json:"last_price"`
	MakerFeeRate     float64 `json:"maker_fee_rate"`
	TakerFeeRate     float64 `json:"taker_fee_rate"`
	OrderSizeMin     float64 `json:"order_size_min"`
	OrderSizeMax     float64 `json:"order_size_max"`
	PriceIncrement   float64 `json:"price_increment"`
	LeverageMin      float64 `json:"leverage_min"`
	LeverageMax      float64 `json:"leverage_max"`
	FundingRate      float64 `json:"funding_rate"`
	FundingInterval  int64   `json:"funding_interval"`
}

type Candle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type BookLevel struct {
	Price    float64 `json:"price"`
	Size     float64 `json:"size"`
	Notional float64 `json:"notional"`
}

type OrderBook struct {
	Bids      []BookLevel `json:"bids"`
	Asks      []BookLevel `json:"asks"`
	UpdatedAt int64       `json:"updated_at"`
}

type Indicators struct {
	EMA20      float64 `json:"ema20"`
	EMA50      float64 `json:"ema50"`
	ATR14      float64 `json:"atr14"`
	RSI14      float64 `json:"rsi14"`
	Mean20     float64 `json:"mean20"`
	UpperBand  float64 `json:"upper_band"`
	LowerBand  float64 `json:"lower_band"`
	SwingHigh  float64 `json:"swing_high"`
	SwingLow   float64 `json:"swing_low"`
	MarketBias string  `json:"market_bias"`
}

type Liquidity struct {
	BidWalls []BookLevel `json:"bid_walls"`
	AskWalls []BookLevel `json:"ask_walls"`
}

type Scenario struct {
	Direction       string         `json:"direction"`
	Status          ScenarioStatus `json:"status"`
	Score           int            `json:"score"`
	Entry           float64        `json:"entry"`
	Stop            float64        `json:"stop"`
	Targets         []float64      `json:"targets"`
	RiskReward      float64        `json:"risk_reward"`
	RiskAmount      float64        `json:"risk_amount"`
	Contracts       float64        `json:"contracts"`
	BaseQuantity    float64        `json:"base_quantity"`
	Notional        float64        `json:"notional"`
	EstimatedFees   float64        `json:"estimated_fees"`
	LiquidationNote string         `json:"liquidation_note"`
	Reasons         []string       `json:"reasons"`
}

type Analysis struct {
	Contract     Contract      `json:"contract"`
	Interval     string        `json:"interval"`
	Strategy     string        `json:"strategy"`
	AnalyzedAt   int64         `json:"analyzed_at"`
	Candles      []Candle      `json:"candles"`
	Indicators   Indicators    `json:"indicators"`
	Liquidity    Liquidity     `json:"liquidity"`
	Long         Scenario      `json:"long"`
	Short        Scenario      `json:"short"`
	NeutralRoute *NeutralRoute `json:"neutral_route,omitempty"`
}

type LiveRequest struct {
	Contract        string  `json:"contract"`
	Interval        string  `json:"interval"`
	TradeNotional   float64 `json:"trade_notional"`
	MinNetSpreadPct float64 `json:"min_net_spread_pct"`
}

type LiveSnapshot struct {
	CapturedAt int64        `json:"captured_at"`
	Candle     Candle       `json:"candle"`
	Gate       VenueMarket  `json:"gate"`
	Binance    VenueMarket  `json:"binance"`
	Route      NeutralRoute `json:"route"`
}
