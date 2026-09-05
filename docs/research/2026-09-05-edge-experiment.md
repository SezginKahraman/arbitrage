# Five-minute edge experiment

This experiment evaluates public Binance, Gate, and KuCoin data. It does not use the user's accounts, credentials, balances, or trading endpoints. Five minutes is an evaluation cadence, not a required holding period or evidence of subsecond execution quality.

## Research findings

Candidate selection should distinguish durable carry from the largest current funding quote. The following diagnostic uses seven trailing 24-hour windows ending at the collection time, with constant nominal exposure. It sums published settlement rates for a fixed direction chosen after seeing current data. It is therefore selection-biased and is NOT a backtest or a trading recommendation. Basis PnL, variable position nominal, depth/slippage, and capital return are excluded.

| Coin | Long / short | Seven-day funding-rate difference | Positive daily windows | Illustrative four-fill fees |
|---|---|---:|---:|---:|
| 4 | Binance / Gate | 0.181343% | 2/7 | 0.25% |
| AKE | Binance / Gate | 0.002958% | 1/7 | 0.25% |
| VELVET | Gate / Binance | 0.121544% | 5/7 | 0.25% |
| FF | Binance / Gate | 0.085400% | 2/7 | 0.25% |
| KOMA | KuCoin / Binance | 0.386720% | 5/7 | 0.22% |
| TRADOOR | Gate / KuCoin | 0.562900% | 7/7 | 0.27% |
| BTC | Gate / Binance | 0.061794% | 7/7 | 0.25% |

TRADOOR and KOMA justify forward observation, not a claim of profit. Today's highest normalized funding differences were not necessarily the most persistent. Account-specific fees are unknown; examples use Binance 0.05%, Gate 0.075%, KuCoin 0.06% per taker fill. Current funding extrapolation is never booked as settled revenue.

Public history sources: [Binance market data](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/market-data), [Gate futures API](https://www.gate.com/docs/developers/apiv4/en/futures/), [KuCoin funding history](https://www.kucoin.com/docs-new/rest/futures-trading/funding-fees/get-public-funding-history). Raw source URLs and event timestamps are saved in `2026-09-05-funding-history.json`; calculations are in `2026-09-05-funding-history-screen.json`.

Funding intervals must be read from current metadata, not hardcoded across assets. KuCoin's August 17 mechanism explicitly changes settlement intervals on extreme rates. [Primary announcement](https://www.kucoin.com/announcement/en-kucoin-futures-to-launch-automatic-funding-fee-settlement-interval-adjustment-mechanism-2026-08-17)

## Alternative: dated cash-and-carry

An asynchronous public Binance top-of-book comparison found approximately 0.1482% BTC and 0.1889% ETH gross basis for September 25 contracts, about 19.6 days to expiry. December 25 gross basis was about 1.2303% BTC and 0.8836% ETH over about 110.6 days. These are raw futures-bid/spot-ask differences before fees, depth, settlement and collateral costs, not executable yields. Capital must support both spot purchase and futures margin. The near expiry basis leaves little cost budget in this sample; further assessment requires contract-specific settlement fees and simultaneous depth.

Unlike perpetual pairs, a dated contract has an expiry mechanism; that does not eliminate interim margin risk. Keep this as a separate longer-horizon research lane. Raw observations are in `2026-09-05-dated-basis-snapshot.json`.

## Falsifiable hypotheses

1. Persistent funding differences pay for entry/exit and basis losses over a multi-payment holding period. Reject if total marked-to-exit equity, including open losers, fails to clear costs.
2. Unusually wide spreads revert sufficiently to clear costs. Record all rejected and entered routes; do not assume full convergence or count repeated samples as independent wins.
3. Combining carry with spread selection improves held-out performance over either signal alone. Separate predicted funding, settlement-rate evidence, and estimated cash valuation. A funding prediction cannot be realized PnL.

Five-minute snapshots cannot validate maker queue position, latency arbitrage, intrainterval stops, or actual dual-venue fills. A positive paper result would justify finer-grained testing, not immediate live deployment. The first run is bounded to 24 hours and may be inconclusive.

## Running and observing

The standalone engine is `scripts/edge_paper.py`. The macOS wrapper is `scripts/edge_paper_service.py`.

```sh
python3 scripts/edge_paper_service.py status
python3 scripts/edge_paper_service.py stop
```

The service writes `.local/edge-paper/latest.md`, `latest.json`, `state.json`, `observations.jsonl`, and `trades.jsonl`. Its logs remain in the same directory. Runtime output is ignored by git. Stop removes the launch agent and retains research data. It does not close or alter real exchange positions because it never creates any.

The local machine needs to be awake and online for scheduled observations. Missed intervals during sleep/outages are not reconstructed as successful observations. The run records a deadline and must not renew the 24-hour budget merely because the process restarts. State and strategy assumptions must be checked before intentionally starting a new experiment.

The service does not send chat notifications or messages to others. The report files are the monitoring surface.
