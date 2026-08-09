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
	sourcePyth               = "pyth"
)

var coreSymbols = []string{"BTCUSDT", "ETHUSDT", "XRPUSDT", "SOLUSDT"}

var cotiSources = map[string]struct{}{
	sourceBinanceFutures: {},
	sourceBybitFutures:   {},
	sourceKrakenFutures:  {},
	sourceGateFutures:    {},
	sourceGateSpot:       {},
	sourceBinanceSpot:    {},
}

func symbolsForSource(source string) []string {
	result := append([]string(nil), coreSymbols...)
	if _, ok := cotiSources[source]; ok {
		result = append(result, "COTIUSDT")
	}
	return result
}
