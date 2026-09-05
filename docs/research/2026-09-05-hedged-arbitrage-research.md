# Hedged arbitrage research and paper-trial specification

Date: 2026-09-05. Scope: Binance, Gate, KuCoin linear USDT perpetuals, matched underlying quantities. Public read-only research only; no orders, deployment, or continuous paper trading were started.

## Decision

Use BTC/ETH as operational controls, SOL/XRP/DOGE/SUI as the main research cohort, COTI/H as a smaller-size experimental cohort. This is a hypothesis-driven watchlist, not a profitability ranking. Compare spread convergence, funding differential, and their combination independently.

Price volatility alone does not generate hedged profit. For equal base quantity q:

```text
Price PnL = q * [(short entry - long entry) - (short exit - long exit)]
Net exit PnL = price PnL + settled net funding
              - actual entry fees - estimated exit fees
              - execution costs not already included in prices
```

Entry uses long ask VWAP and short bid VWAP; exit uses long bid VWAP and short ask VWAP. Avoid double-counting spread/slippage already included in VWAP. Predicted funding is not settled profit. Match underlying quantity rather than equal USDT amounts; account for contract multipliers and lot rounding. Leverage does not increase profit for unchanged nominal exposure.

## Observations

Five public market endpoints were received between 16:30:38 and 16:30:46 UTC. Active perpetual/USDT filters and BTC/XBT normalization produced 457 common candidates. Other matches use base names only, so economic identity, index, multipliers, and redenominations still require validation. These asynchronous snapshots cannot establish an executable cross-venue spread or historical profitability.

| Coin | Binance 24h turnover, million USDT | Gate | KuCoin | Binance high/low range |
|---|---:|---:|---:|---:|
| BTC | 4085.69 | 1049.79 | 150.16 | 0.66% |
| ETH | 2606.03 | 1128.38 | 89.87 | 0.90% |
| SOL | 885.66 | 181.72 | 37.54 | 2.35% |
| XRP | 597.60 | 44.80 | 47.43 | 2.15% |
| DOGE | 367.72 | 40.85 | 17.11 | 4.99% |
| SUI | 231.33 | 13.70 | 13.60 | 8.05% |
| COTI | 21.57 | 0.98 | 0.87 | 14.85% |
| H | 3.61 | 0.83 | 3.62 | 9.01% |

Volume is exchange-reported and is not guaranteed liquidity. High/low range is not realized volatility. Other high-volume candidates such as ZEC merit screening, but one day's volume burst does not establish durable suitability.

An additional 24 book requests covered eight coins across three venues. For each book, compute buy and sell VWAP for the same base quantity, valued at 100, 500, or 1000 USDT at the midpoint. This is hypothetical same-venue instantaneous buy/sell friction, not a cross-venue profit calculation or a four-fill cost estimate. Fees, lot quantization, latency, queueing, and actual fills are excluded.

| Coin | Maximum sampled same-venue buy/sell friction across venues, 1000 USDT |
|---|---:|
| BTC | 0.00013% |
| ETH | 0.00041% |
| SOL | 0.00969% |
| XRP | 0.01861% |
| DOGE | 0.02083% |
| SUI | 0.05411% |
| COTI | 0.22929% |
| H | 0.40893% |

Small observed BTC/ETH friction does not prove that arbitrage opportunities clear fees. COTI/H exhibit much higher friction despite larger price movement.

## Costs and strategies

Illustrative fees: Binance taker 0.05% (existing project assumption), Gate taker 0.075% (sampled public contract metadata). At approximately 1000 USDT per leg, four fills cost approximately 2.50 USDT, or 0.25% of one leg's notional. These are not verified account-specific rates. A spread narrowing from 0.40% to 0.05%, less 0.25% fees and 0.04% additional execution cost, leaves approximately 0.06%, or 0.60 USDT before funding. Actual PnL must use quantities and fill prices in USDT, not approximate percentage subtraction.

| Strategy | Source of profit | Main limitation / decision |
|---|---|---|
| Perpetual spread convergence | Buy cheap venue, short rich venue; spread narrows | Initial trial; spread can widen or persist |
| Funding differential | Short funding receipt minus long funding payment | Separate trial; basis losses and rate reversals can exceed carry |
| Combined filter | Spread plus net funding | Third trial; conservative forecast, separate attribution |
| Spread bands / z-score | Relative deviation from its own historical baseline | Reversion need not be to zero; regime shifts and overfitting matter |
| Spot long + perpetual short | Positive funding and possible basis convergence | Comparison strategy; spot capital and separate futures margin required |
| Spot short + perpetual long | Negative funding | Borrow availability, interest, recall; outside first trial |
| Spot long + dated future short | Basis to expiry | Different instruments and settlement, longer capital commitment |
| Maker/taker execution | Passive price/fee advantage with active hedge | Later stage; adverse selection and queue/fill uncertainty |
| Maker/maker | Two passive fills | Waiting for both creates unhedged exposure |
| Funding-time sniping / lead-lag | Single payment or delayed venue repricing | Outside first trial; settlement and latency uncertainty |
| Different-coin pairs, grid, martingale | Correlation or directional assumptions | Different risk model; do not mix with matched-asset strategy |

Funding comparisons require a common time horizon and actual payment events. A rate per hour is not directly comparable with one per eight hours. Display normalization can help, but cash flows depend on position holdings at settlement and the nominal at each event. Unknown intervals should prevent qualification. KuCoin documents dynamically changing intervals and settlement assessment taking up to one minute. [KuCoin funding](https://www.kucoin.com/support/26686295987353)

## Execution and risk

- Cross-venue fills are not atomic. Track entry, partial fill, hedged position, exit, and recovery separately. Failed compensation must leave visible residual exposure rather than a false closed state.
- Each venue has separate collateral. A common market move can cancel aggregate price PnL while liquidating one leg. Stress each venue independently and reserve collateral; emergency transfers are not a reliable rescue assumption.
- Require profit target, loss limit, maximum holding time, funding-regime, delisting, stale-data, and outage policies. A positive exit threshold alone can trap capital indefinitely.
- Reconcile positions and orders on restart; prevent duplicate entries; use position-mode-appropriate reducing exit orders. IOC price limits can constrain price but do not eliminate partial fills.
- ADL can reduce a profitable leg and break the hedge. Reconciliation must observe exchange positions, not just bot-submitted orders. [Binance ADL](https://www.binance.com/en-AE/support/faq/detail/360033525471)
- Account for index divergence, stablecoin/custody exposure, API outages, and portfolio rebalancing costs. Report USDT PnL and return on total allocated capital separately from return on one leg's notional.

## Proposed paper experiment — not yet running

1. Eight starting coins, three venue pairs, both directions: 48 directed routes. Validate economic identity, contract multipliers, lot/minimum-order rules, and index compatibility. KuCoin stays public-data/simulation only initially.
2. Collect an initial seven days; extend if too few independent completed opportunities occur. Broaden to two to four weeks and multiple market regimes. Seven days alone is not proof of profitability.
3. Store depth events, sequence information, exchange and receive times, contract changes, funding estimates, and actual funding history. Derive freshness/skew limits from measured latency. Deduplicate opportunities; repeated samples are not independent trades.
4. Compare 100/500/1000 USDT per-leg nominal scenarios with matched base amounts and exchange constraints. These sizes are not assumptions about user capital. Portfolio simulations reserve collateral and initially permit only one paired position at a time.
5. Run separate convergence-only, funding-only, and combined strategies. Funding gets its own holding horizon. Select thresholds on an earlier time segment and freeze them on the later validation segment.
6. Start with conservative taker/taker assumptions. Use subsequent available books after simulated latency; model insufficient depth, partial fills, unwind costs, and gaps. Do not assume maker fill merely because price touched a limit.
7. Close when executable net exit value exceeds the target and buffer, or a predeclared risk/time exit triggers. Candidate buffers of 0.05/0.10/0.20% of one leg's notional are sensitivity scenarios, not proven settings. Stop/time rules must be fixed before validation.
8. Report candidate/entered/rejected events; net PnL; open positions marked at executable exit value; fee/funding/slippage attribution; tail losses; drawdown; median/p95 holding time; unhedged duration; and capital usage. Never exclude losing open positions from performance.
9. Stress fees/slippage, latency, funding reversal, one-venue outage, sudden margin losses, and ADL. Positive average PnL is insufficient without out-of-sample resilience and tested recovery.

## Existing project gaps

- `web/src/lib/opportunity-desks.ts` selects the cheapest ask and highest alternative bid, rather than every funding route. Fees are static screening assumptions and quotes may be up to 15 seconds old. This is not execution-grade evidence.
- `futures/cross_venue.go` accounts for entry depth and matched quantities but assumes a common convergence exit price. The estimated convergence return is not current executable liquidation value. Its separately displayed funding difference does not model payment schedules.
- Binance/Gate entry and failed-entry compensation exist. README and inspected code do not provide continuous net-profit monitoring and paired profit exit. Runtime/account behavior was not tested in this research, and product code was not changed.

## Evidence

- [Binance tickers](https://fapi.binance.com/fapi/v1/ticker/24hr), [contract metadata](https://fapi.binance.com/fapi/v1/exchangeInfo)
- [Gate tickers](https://api.gateio.ws/api/v4/futures/usdt/tickers), [contract metadata](https://api.gateio.ws/api/v4/futures/usdt/contracts), [API specification](https://www.gate.com/docs/developers/apiv4/en/)
- [KuCoin contracts](https://api-futures.kucoin.com/api/v1/contracts/active), [official SDK](https://github.com/Kucoin/kucoin-futures-node-sdk)
- [Binance fee calculation](https://www.binance.com/en/support/faq/detail/360033544231): illustrative rates are not an account-specific schedule.
- `2026-09-05-public-market-snapshot.json`: source URLs, times, raw responses.
- `2026-09-05-candidate-screen.json`: filtered candidates and volume/range transformations.
- `2026-09-05-depth-snapshot.json`: raw books, multipliers, size-specific cost calculations.

Verification scope: JSON readback, complete 8x3 book coverage, and independent recalculation of depth metrics. No historical profitability, actual fills, or continuous paper-trading performance was established.
