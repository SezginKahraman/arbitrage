package main

import (
	"math"
	"sort"
	"time"
)

const (
	quoteFreshnessWindow = 15 * time.Second
	minimumGrossSpread   = 0.05
)

type Quote struct {
	Symbol    string  `json:"symbol"`
	Source    string  `json:"source"`
	BestBid   float64 `json:"best_bid"`
	BestAsk   float64 `json:"best_ask"`
	Timestamp int64   `json:"timestamp"`
}

func validQuote(symbol string, quote Quote, now time.Time) bool {
	if quote.Symbol != symbol || quote.Source == "" {
		return false
	}
	if math.IsNaN(quote.BestBid) || math.IsNaN(quote.BestAsk) || math.IsInf(quote.BestBid, 0) || math.IsInf(quote.BestAsk, 0) {
		return false
	}
	if quote.BestBid <= 0 || quote.BestAsk <= 0 || quote.BestBid > quote.BestAsk {
		return false
	}
	age := now.Sub(time.UnixMilli(quote.Timestamp))
	return age >= 0 && age <= quoteFreshnessWindow
}

func FindBestOpportunity(symbol string, quotes map[string]Quote) (ArbitrageOpportunity, bool) {
	return FindBestOpportunityAt(symbol, quotes, time.Now())
}

func FindBestOpportunityAt(symbol string, quotes map[string]Quote, now time.Time) (ArbitrageOpportunity, bool) {
	opportunities := FindOpportunitiesAt(symbol, quotes, now)
	if len(opportunities) == 0 {
		return ArbitrageOpportunity{}, false
	}
	return opportunities[0], true
}

func FindOpportunitiesAt(symbol string, quotes map[string]Quote, now time.Time) []ArbitrageOpportunity {
	opportunities := make([]ArbitrageOpportunity, 0)

	for buySource, buyQuote := range quotes {
		if !validQuote(symbol, buyQuote, now) {
			continue
		}
		for sellSource, sellQuote := range quotes {
			if buySource == sellSource || !validQuote(symbol, sellQuote, now) {
				continue
			}

			spreadPct := ((sellQuote.BestBid - buyQuote.BestAsk) / buyQuote.BestAsk) * 100
			if spreadPct <= minimumGrossSpread {
				continue
			}

			opportunities = append(opportunities, ArbitrageOpportunity{
				Symbol:     symbol,
				BuySource:  buySource,
				SellSource: sellSource,
				BuyPrice:   buyQuote.BestAsk,
				SellPrice:  sellQuote.BestBid,
				ProfitPct:  spreadPct,
				Timestamp:  min(buyQuote.Timestamp, sellQuote.Timestamp),
			})
		}
	}

	sort.Slice(opportunities, func(left, right int) bool {
		if opportunities[left].ProfitPct != opportunities[right].ProfitPct {
			return opportunities[left].ProfitPct > opportunities[right].ProfitPct
		}
		if opportunities[left].BuySource != opportunities[right].BuySource {
			return opportunities[left].BuySource < opportunities[right].BuySource
		}
		return opportunities[left].SellSource < opportunities[right].SellSource
	})
	return opportunities
}
