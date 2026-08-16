package main

const (
	sourceBinanceFutures     = "binance_futures"
	sourceBybitFutures       = "bybit_futures"
	sourceHyperliquidFutures = "hyperliquid_futures"
	sourceKrakenFutures      = "kraken_futures"
	sourceOKXFutures         = "okx_futures"
	sourceGateFutures        = "gate_futures"
	sourceGateSpot           = "gate_spot"
	sourceParadexFutures     = "paradex_futures"
	sourceBinanceSpot        = "binance_spot"
	sourceBybitSpot          = "bybit_spot"
	sourceKuCoinFutures      = "kucoin_futures"
	sourceKuCoinSpot         = "kucoin_spot"
	sourcePyth               = "pyth"
)

var configuredSources = []string{
	sourceBinanceFutures, sourceBybitFutures, sourceHyperliquidFutures, sourceKrakenFutures,
	sourceOKXFutures, sourceGateFutures, sourceKuCoinFutures, sourceParadexFutures,
	sourceBinanceSpot, sourceBybitSpot, sourceGateSpot, sourceKuCoinSpot, sourcePyth,
}
