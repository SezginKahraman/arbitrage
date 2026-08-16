package main

import (
	"reflect"
	"testing"
)

func TestMarketDiscoveryParsersNormalizeActiveMarkets(t *testing.T) {
	tests := []struct {
		name    string
		parse   func([]byte) ([]string, error)
		payload string
		want    []string
	}{
		{
			name: "binance spot", parse: parseBinanceSpotMarkets,
			payload: `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","quoteAsset":"USDT","isSpotTradingAllowed":true},{"symbol":"OLDUSDT","status":"BREAK","quoteAsset":"USDT","isSpotTradingAllowed":true}]}`,
			want:    []string{"BTCUSDT"},
		},
		{
			name: "binance futures", parse: parseBinanceFuturesMarkets,
			payload: `{"symbols":[{"symbol":"SOLUSDT","status":"TRADING","quoteAsset":"USDT","contractType":"PERPETUAL"},{"symbol":"SOLUSDT_250101","status":"TRADING","quoteAsset":"USDT","contractType":"CURRENT_QUARTER"}]}`,
			want:    []string{"SOLUSDT"},
		},
		{
			name: "gate spot", parse: parseGateSpotMarkets,
			payload: `[{"id":"COTI_USDT","quote":"USDT","trade_status":"tradable"},{"id":"OLD_USDT","quote":"USDT","trade_status":"untradable"}]`,
			want:    []string{"COTIUSDT"},
		},
		{
			name: "bybit", parse: parseBybitMarkets,
			payload: `{"retCode":0,"result":{"list":[{"symbol":"XRPUSDT","quoteCoin":"USDT","status":"Trading"},{"symbol":"OLDUSDT","quoteCoin":"USDT","status":"Settled"}]}}`,
			want:    []string{"XRPUSDT"},
		},
		{
			name: "gate futures", parse: parseGateFuturesMarkets,
			payload: `[{"name":"ETH_USDT","type":"direct","in_delisting":false},{"name":"OLD_USDT","type":"direct","in_delisting":true}]`,
			want:    []string{"ETHUSDT"},
		},
		{
			name: "kucoin spot", parse: parseKuCoinSpotMarkets,
			payload: `{"code":"200000","data":[{"symbol":"SOL-USDT","quoteCurrency":"USDT","enableTrading":true},{"symbol":"OLD-USDT","quoteCurrency":"USDT","enableTrading":false}]}`,
			want:    []string{"SOLUSDT"},
		},
		{
			name: "kucoin futures", parse: parseKuCoinFuturesMarkets,
			payload: `{"code":"200000","data":[{"symbol":"XBTUSDTM","quoteCurrency":"USDT","status":"Open"},{"symbol":"ETHUSDTM","quoteCurrency":"USDT","status":"Closed"}]}`,
			want:    []string{"BTCUSDT"},
		},
		{
			name: "okx futures", parse: parseOKXFuturesMarkets,
			payload: `{"code":"0","data":[{"instId":"LINK-USDT-SWAP","settleCcy":"USDT","state":"live","ctType":"linear"},{"instId":"BTC-USD-SWAP","settleCcy":"BTC","state":"live","ctType":"inverse"}]}`,
			want:    []string{"LINKUSDT"},
		},
		{
			name: "kraken", parse: parseKrakenFuturesMarkets,
			payload: `{"result":"success","instruments":[{"symbol":"PF_XBTUSD","tradeable":true},{"symbol":"FI_ETHUSD_250101","tradeable":true}]}`,
			want:    []string{"BTCUSDT"},
		},
		{
			name: "hyperliquid", parse: parseHyperliquidMarkets,
			payload: `{"universe":[{"name":"BTC"},{"name":"PURR","isDelisted":true}]}`,
			want:    []string{"BTCUSDT"},
		},
		{
			name: "paradex", parse: parseParadexMarkets,
			payload: `{"results":[{"symbol":"BTC-USD-PERP","base_currency":"BTC","asset_kind":"PERP","status":"ACTIVE"},{"symbol":"BTC-USD-25DEC26-85-P","base_currency":"BTC","asset_kind":"OPTION","status":"ACTIVE"}]}`,
			want:    []string{"BTCUSDT"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.parse([]byte(test.payload))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("symbols = %v, want %v", got, test.want)
			}
		})
	}
}
