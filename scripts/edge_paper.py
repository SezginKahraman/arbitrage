#!/usr/bin/env python3
"""Public-data, bounded paper experiment. NEVER sends orders or reads API keys.

python3 scripts/edge_paper.py --once --output-dir /tmp/edge-paper
python3 scripts/edge_paper.py --interval 300 --duration-hours 24 --output-dir /tmp/edge-paper
"""
import argparse
import concurrent.futures
import datetime
from decimal import Decimal, ROUND_DOWN
import fcntl
import itertools
import json
import math
import os
from pathlib import Path
import signal
import time
import urllib.parse
import urllib.request
from collections import Counter

VENUES = ('binance', 'gate', 'kucoin')
STRATEGIES = ('convergence', 'funding', 'hybrid')
BASE = {'binance': 'https://fapi.binance.com', 'gate': 'https://api.gateio.ws/api/v4', 'kucoin': 'https://api-futures.kucoin.com'}
CONTROLS = ['BTC', 'ETH', 'SOL', 'XRP', 'DOGE', 'SUI', 'TRADOOR', 'KOMA']
ASSUMPTIONS = [
    'PUBLIC GET ONLY. Hypothetical paired fills; no actual orders or account access.',
    'Binance taker assumed 0.05%; Gate/KuCoin public contract taker rates, not account-specific.',
    'Each strategy independently starts with 1000 USDT at each venue; nominal is per leg; collateral reserved at 1x.',
    'Entry long ask / short bid depth VWAP; exit long bid / short ask; four fees; additional latency cost 10 bps per pair roundtrip.',
    'Profit target 0.15% of larger entry leg; stop loss 1%; maximum hold 6h convergence / 24h carry.',
    'Funding rates are forecasts at entry. Only published settlement rates enter funding estimates; absent settlement mark uses last sampled book mid.',
    'All funding is modeled paper cashflow, never verified exchange credit. Missing due history disables profit exits and makes equity incomplete.',
    'Receive skew <=3s, response duration <=3s, exchange timestamp age <=3s; missing timestamps rejected. Cross-venue simultaneous execution is not guaranteed.',
    'REST 5-minute samples miss intrainterval paths, liquidation, partial fills, ADL, queue effects, outages and brief opportunities.',
    'Gate enable_decimal contracts are unsupported and excluded (currently includes ETH/SOL/XRP); quantity increments cannot be established from metadata. Numeric multiplier-prefixed symbols excluded, single-character 4 permitted.',
    'Symbols matched by base ticker and contract filters, not definitive economic/index identity. No historical profitability or future edge claim.',
]


def utc(ts=None):
    return datetime.datetime.fromtimestamp(ts or time.time(), datetime.timezone.utc).isoformat()


def get(venue, path, **params):
    url = BASE[venue] + path + ('?' + urllib.parse.urlencode(params) if params else '')
    start = time.time()
    req = urllib.request.Request(url, headers={'User-Agent': 'edge-paper-research/1.0'})
    with urllib.request.urlopen(req, timeout=12) as response:
        raw = json.load(response)
    received = time.time()
    if venue == 'kucoin':
        if str(raw.get('code')) != '200000':
            raise ValueError('KuCoin public API error: ' + str(raw.get('code')))
        raw = raw['data']
    return raw, received, received - start


def parallel(items, fn, workers=6):
    result, errors = {}, {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
        pending = {pool.submit(fn, item): item for item in items}
        for future in concurrent.futures.as_completed(pending):
            item = pending[future]
            try:
                result[item] = future.result()
            except Exception as exc:
                errors[str(item)] = str(exc)
    return result, errors


def normalize(base):
    return 'BTC' if base == 'XBT' else base


def market(symbol, multiplier, step, fee, volume=0, rate=None, interval=None, next_time=None, minimum=0, maximum=None):
    return dict(symbol=symbol, multiplier=float(multiplier), step=str(step), fee=float(fee), volume=float(volume or 0),
                rate=None if rate is None else float(rate), interval=interval, next=next_time,
                minimum=float(minimum), maximum=None if maximum is None else float(maximum))


def discover():
    endpoints = {
        'bi': ('binance', '/fapi/v1/exchangeInfo'), 'bt': ('binance', '/fapi/v1/ticker/24hr'),
        'bp': ('binance', '/fapi/v1/premiumIndex'), 'bf': ('binance', '/fapi/v1/fundingInfo'),
        'gc': ('gate', '/futures/usdt/contracts'), 'gt': ('gate', '/futures/usdt/tickers'),
        'kc': ('kucoin', '/api/v1/contracts/active'),
    }
    data, errors = parallel(list(endpoints), lambda k: get(*endpoints[k])[0])
    if any(k not in data for k in ('bi', 'bt', 'bp', 'gc', 'gt', 'kc')):
        raise RuntimeError('Discovery unavailable: ' + json.dumps(errors))
    out = {v: {} for v in VENUES}
    bt = {x['symbol']: x for x in data['bt']}
    bp = {x['symbol']: x for x in data['bp']}
    bf = {x['symbol']: x for x in data.get('bf', [])}
    for x in data['bi']['symbols']:
        base = normalize(x['baseAsset'])
        if x['status'] != 'TRADING' or x['contractType'] != 'PERPETUAL' or x['quoteAsset'] != 'USDT' or x.get('marginAsset') != 'USDT' or (len(base) > 1 and base[0].isdigit()):
            continue
        if x.get('underlyingType') not in (None, 'COIN'):
            continue
        filters = {f['filterType']: f for f in x['filters']}
        lot = filters.get('MARKET_LOT_SIZE', filters['LOT_SIZE'])
        if float(lot['stepSize']) <= 0:
            lot = filters['LOT_SIZE']
        p = bp.get(x['symbol'], {})
        # Binance fundingInfo lists adjusted contracts only. Do not invent an interval for absent rows.
        interval = bf.get(x['symbol'], {}).get('fundingIntervalHours')
        out['binance'][base] = market(x['symbol'], 1, lot['stepSize'], .0005,
            bt.get(x['symbol'], {}).get('quoteVolume'), p.get('lastFundingRate'),
            float(interval) * 3600 if interval else None, float(p.get('nextFundingTime', 0)) / 1000,
            filters.get('MIN_NOTIONAL', {}).get('notional', 0), lot.get('maxQty'))
        out['binance'][base]['min_qty'] = float(lot['minQty'])
    gt = {x['contract']: x for x in data['gt']}
    for x in data['gc']:
        if not x['name'].endswith('_USDT') or x.get('type') != 'direct' or x.get('in_delisting') or x.get('is_pre_market') or x.get('status', 'trading') != 'trading' or x.get('enable_decimal'):
            continue
        base = normalize(x['name'][:-5])
        if not base or (len(base) > 1 and base[0].isdigit()) or x.get('contract_type') not in (None, '', 'perpetual'):
            continue
        mult = Decimal(x['quanto_multiplier'])
        out['gate'][base] = market(x['name'], mult, mult, x['taker_fee_rate'], gt.get(x['name'], {}).get('volume_24h_quote', 0),
            x.get('funding_rate'), x.get('funding_interval'), x.get('funding_next_apply'), maximum=float(mult)*float(x.get('market_order_size_max', x['order_size_max'])))
        out['gate'][base]['min_qty'] = float(mult)*float(x['order_size_min'])
    for x in data['kc']:
        if x.get('status') != 'Open' or x.get('quoteCurrency') != 'USDT' or x.get('settleCurrency') != 'USDT' or x.get('isInverse') or x.get('expireDate') or x.get('marketType', 'CRYPTO') != 'CRYPTO' or x.get('marketStage', 'NORMAL') != 'NORMAL':
            continue
        base = normalize(x['baseCurrency'])
        if not base or (len(base) > 1 and base[0].isdigit()) or float(x['multiplier']) <= 0:
            continue
        mult = Decimal(str(x['multiplier']))
        out['kucoin'][base] = market(x['symbol'], mult, mult * Decimal(str(x['lotSize'])), x['takerFeeRate'], x.get('turnoverOf24h'),
            x.get('fundingFeeRate'), float(x.get('currentFundingRateGranularity') or x.get('fundingRateGranularity') or 0) / 1000,
            float(x.get('nextFundingRateDateTime') or 0) / 1000, maximum=float(mult)*float(x['marketMaxOrderQty']))
        out['kucoin'][base]['min_qty'] = float(mult)*float(x['lotSize'])
    errors['unsupported_gate_decimal_contracts'] = [x['name'] for x in data['gc'] if x.get('enable_decimal')]
    return out, errors


def funding_known(m, now):
    return m.get('rate') is not None and bool(m.get('interval')) and m['interval'] > 0 and bool(m.get('next')) and now < m['next'] <= now + m['interval'] + 120


def universe(markets, limit, now):
    common = set.intersection(*(set(markets[v]) for v in VENUES))
    liquid = [c for c in common if min(markets[v][c]['volume'] for v in VENUES) >= 500000]
    def funding_score(c):
        rates = [markets[v][c]['rate'] / markets[v][c]['interval'] for v in VENUES if funding_known(markets[v][c], now)]
        return max(rates) - min(rates) if len(rates) >= 2 else 0
    # Reserve half the non-control slots for high funding dispersion and half for weakest-venue liquidity.
    selected = [c for c in CONTROLS if c in common][:limit]
    slots = max(0, limit-len(selected))
    for c in sorted(liquid, key=funding_score, reverse=True):
        if c not in selected and slots and len(selected) < limit - slots//2:
            selected.append(c)
    for c in sorted(liquid, key=lambda c: min(markets[v][c]['volume'] for v in VENUES), reverse=True):
        if c not in selected and len(selected) < limit:
            selected.append(c)
    return selected, len(common)


def fetch_book(venue, m):
    if venue == 'binance':
        raw, received, elapsed = get(venue, '/fapi/v1/depth', symbol=m['symbol'], limit=100)
        timestamp = float(raw.get('T', raw.get('E', 0))) / 1000
    elif venue == 'gate':
        raw, received, elapsed = get(venue, '/futures/usdt/order_book', contract=m['symbol'], limit=100)
        timestamp = float(raw.get('update', 0))
    else:
        raw, received, elapsed = get(venue, '/api/v1/level2/depth100', symbol=m['symbol'])
        timestamp = float(raw.get('ts', 0)) / 1e9
    sides = {}
    for side in ('bids', 'asks'):
        levels = [(float(x['p']), float(x['s'])) if isinstance(x, dict) else (float(x[0]), float(x[1])) for x in raw[side]]
        sides[side] = sorted([(p, q * m['multiplier']) for p, q in levels if p > 0 and q > 0], reverse=side == 'bids')
    return dict(**sides, received=received, elapsed=elapsed, timestamp=timestamp)


def fresh(a, b, now=None):
    for x in (a, b):
        if not x.get('timestamp'):
            return False, 'missing_exchange_timestamp'
        if x['elapsed'] > 3 or not -1 <= x['received'] - x['timestamp'] <= 3 or (now is not None and now - x['timestamp'] > 3):
            return False, 'stale_book'
        if not x['bids'] or not x['asks'] or x['bids'][0][0] >= x['asks'][0][0]:
            return False, 'invalid_book'
    if abs(a['received'] - b['received']) > 3 or abs(a['timestamp'] - b['timestamp']) > 3:
        return False, 'cross_venue_skew'
    return True, None


def vwap(levels, quantity):
    remaining, cost = quantity, 0.0
    for price, size in levels:
        fill = min(remaining, size)
        cost += price * fill
        remaining -= fill
        if remaining <= quantity * 1e-12:
            return cost / quantity
    raise ValueError('insufficient_depth')


def common_quantity(quantity, steps):
    ds = [Decimal(str(x)) for x in steps]
    scale = 10 ** max(0, max(-x.as_tuple().exponent for x in ds))
    ints = [int(x * scale) for x in ds]
    if min(ints) <= 0:
        raise ValueError('invalid_step')
    multiple = math.lcm(*ints)
    step = Decimal(multiple) / Decimal(scale)
    return float((Decimal(str(quantity)) / step).to_integral_value(rounding=ROUND_DOWN) * step)


def exit_value(p, a, b):
    q = p['quantity']
    long_exit, short_exit = vwap(a['bids'], q), vwap(b['asks'], q)
    price_pnl = q * (long_exit - p['long_entry'] + p['short_entry'] - short_exit)
    fees = p['entry_fees'] + q * (long_exit * p['long_fee'] + short_exit * p['short_fee'])
    return dict(net=price_pnl - fees - p['latency_cost'] + p.get('funding_estimate', 0), price_pnl=price_pnl,
                fees=fees, long_exit=long_exit, short_exit=short_exit)


def settled_history(venue, symbol, start, end):
    if venue == 'binance':
        raw = get(venue, '/fapi/v1/fundingRate', symbol=symbol, startTime=int(start*1000), endTime=int(end*1000), limit=1000)[0]
        return [(float(x['fundingTime'])/1000, float(x['fundingRate']), float(x.get('markPrice') or 0)) for x in raw]
    if venue == 'gate':
        raw = get(venue, '/futures/usdt/funding_rate', contract=symbol, limit=100)[0]
        return [(float(x['t']), float(x['r']), 0) for x in raw if start < float(x['t']) <= end]
    raw = get(venue, '/api/v1/contract/funding-rates', symbol=symbol, **{'from': int(start*1000), 'to': int(end*1000)})[0]
    return [(float(x['timepoint'])/1000, float(x['fundingRate']), 0) for x in raw]


def accrue(p, markets, books, now):
    pending = []
    for side, sign in (('long', -1), ('short', 1)):
        venue = p[side]
        m = markets[venue].get(p['coin'])
        b = books.get((p['coin'], venue))
        due = p['next_funding'].get(side)
        if not due or due > now:
            if not due:
                pending.append(side + ':unknown_schedule')
            continue
        try:
            events = settled_history(venue, p[side+'_symbol'], p['opened'], now)
            interval = p.get('funding_intervals', {}).get(side, m.get('interval') if m else None)
            expected = []
            if interval and interval > 0:
                expected = [due + i*interval for i in range(int((now-due)//interval)+1)]
            seen_due = bool(expected) and all(any(abs(ts-target) <= 60 for ts,rate,mark in events) for target in expected)
            interval_changed = bool(m and interval and m.get('interval') != interval)
            for ts, rate, mark in events:
                if ts <= p['opened']:
                    continue
                key = side + ':' + str(ts)
                if key in p['funding_events']:
                    continue
                sampled = not bool(mark)
                if not mark:
                    if not b:
                        raise ValueError('no sampled mark')
                    mark = (b['asks'][0][0] + b['bids'][0][0])/2
                cashflow = sign * p['quantity'] * mark * rate
                p['funding_estimate'] += cashflow
                p['funding_events'][key] = dict(time=ts, rate=rate, mark=mark, sampled_mark=sampled, cashflow=cashflow)
            if not seen_due or interval_changed:
                pending.append(side + ':missing_settlement_or_changed_schedule')
            elif m and funding_known(m, now):
                p['next_funding'][side] = m['next']
            else:
                pending.append(side + ':unknown_next_schedule')
        except Exception as exc:
            pending.append(side + ':' + str(exc))
    p['funding_pending'] = pending


def new_state():
    return dict(version=1, last_cycle=None, positions=[], closed=[], capital={s: {v: 1000.0 for v in VENUES} for s in STRATEGIES}, cycles=0)


def load_state(path):
    return json.loads(path.read_text()) if path.exists() else new_state()


def atomic_json(path, data):
    tmp = path.with_suffix(path.suffix+'.tmp')
    with tmp.open('w') as f:
        json.dump(data, f, indent=2, allow_nan=False)
        f.flush()
        os.fsync(f.fileno())
    os.replace(tmp, path)


def append_json(path, data):
    with path.open('a') as f:
        f.write(json.dumps(data, allow_nan=False)+'\n')
        f.flush()
        os.fsync(f.fileno())


def cycle(state, markets, books, now, nominal=100, cycle_id=None, decision_clock=None):
    decision_clock = decision_clock or (lambda: now)
    cycle_id = cycle_id or str(int(now))
    if state['last_cycle'] == cycle_id:
        return dict(duplicate=True, observations=[], trades=[], rejections={})
    rejected, observations, trades = Counter(), [], []
    for p in list(state['positions']):
        a, b = books.get((p['coin'], p['long'])), books.get((p['coin'], p['short']))
        accrue(p, markets, books, decision_clock())
        now = decision_clock()
        if not a or not b or not fresh(a, b, now)[0]:
            p['valuation_stale'] = True
            continue
        try:
            value = exit_value(p, a, b)
        except ValueError:
            p['valuation_stale'] = True
            continue
        p.update(last_value=value, last_valued=now, valuation_stale=False)
        age = now-p['opened']
        reason = 'profit' if value['net'] >= p['target'] and not p['funding_pending'] else 'loss' if value['net'] <= -p['stop'] else 'max_hold' if age >= p['max_hold'] else None
        if reason and p['opened_cycle'] != cycle_id:
            p.update(closed_at=now, reason=reason, pnl=value['net'], accounting_complete=not bool(p['funding_pending']))
            for side in ('long', 'short'):
                venue = p[side]
                # Allocate each leg's price PnL and actual modeled fee to its own venue.
                sign = 1 if side == 'long' else -1
                cash = sign*p['quantity']*(value[side+'_exit']-p[side+'_entry'])
                cash -= p['quantity']*(p[side+'_entry']+value[side+'_exit'])*p[side+'_fee']
                cash -= p['latency_cost']/2
                cash += sum(e['cashflow'] for k,e in p['funding_events'].items() if k.startswith(side+':'))
                state['capital'][p['strategy']][venue] += cash
            state['positions'].remove(p)
            state['closed'].append(p)
            trades.append(dict(event='close', position=p.copy()))
    now = decision_clock()
    coins = sorted(set(c for c, v in books))
    candidates = []
    for coin in coins:
        for long, short in itertools.permutations(VENUES, 2):
            a, b = books.get((coin, long)), books.get((coin, short))
            if not a or not b or coin not in markets[long] or coin not in markets[short]:
                rejected['missing_book_or_contract'] += 1
                continue
            valid, reason = fresh(a, b, now)
            if not valid:
                rejected[reason] += 1
                continue
            lm, sm = markets[long][coin], markets[short][coin]
            q = common_quantity(nominal / max(a['asks'][0][0], b['bids'][0][0]), [lm['step'], sm['step']])
            if q <= 0 or any(q < m.get('min_qty',0) or (m.get('maximum') is not None and q > m['maximum']) for m in (lm,sm)):
                rejected['quantity_limits'] += 1
                continue
            try:
                le, se = vwap(a['asks'], q), vwap(b['bids'], q)
            except ValueError:
                rejected['insufficient_depth'] += 1
                continue
            if any(q*price < m['minimum'] for m,price in ((lm,le),(sm,se))):
                rejected['minimum_notional'] += 1
                continue
            n = q * max(le,se)
            known = funding_known(lm, now) and funding_known(sm, now)
            def events(m):
                return max(0, int((now + 86400 - m['next']) // m['interval']) + 1)
            forecast = q*(se*sm['rate']*events(sm)-le*lm['rate']*events(lm)) if known else None
            fees = q*(le*lm['fee']+se*sm['fee'])
            potential = q*(se-le)-2*fees-.001*n
            try:
                lx, sx = vwap(a['bids'],q), vwap(b['asks'],q)
            except ValueError:
                rejected['insufficient_exit_depth'] += 1
                continue
            immediate_net = q*(lx-le+se-sx)-fees-q*(lx*lm['fee']+sx*sm['fee'])-.001*n
            row = dict(coin=coin, long=long, short=short, quantity=q, long_entry=le, short_entry=se,
                nominal=n, convergence_potential=potential, immediate_exit_net=immediate_net, forecast_funding_24h=forecast, funding_metadata_known=known,
                entry_fees=fees, latency_cost=.001*n, long_fee=lm['fee'], short_fee=sm['fee'])
            observations.append(row.copy())
            candidates.append((potential, row, lm, sm))
    for _, row, lm, sm in sorted(candidates, key=lambda x:x[0], reverse=True):
        now = decision_clock()
        if not fresh(books[(row['coin'],row['long'])], books[(row['coin'],row['short'])], now)[0]:
            rejected['stale_at_entry'] += 1
            continue
        for strategy in STRATEGIES:
            n, forecast = row['nominal'], row['forecast_funding_24h']
            qualifies = row['convergence_potential'] >= .002*n
            if strategy == 'funding':
                qualifies = forecast is not None and forecast + row['immediate_exit_net'] >= .003*n
            elif strategy == 'hybrid':
                qualifies = qualifies and forecast is not None and forecast >= .001*n
            if not qualifies:
                rejected[strategy+':below_threshold_or_unknown_funding'] += 1
                continue
            # One pair per strategy, total strategy capital remains venue-separated.
            if any(p['strategy'] == strategy for p in state['positions']):
                rejected[strategy+':position_limit'] += 1
                continue
            if any(state['capital'][strategy][row[side]] < row['quantity']*row[side+'_entry']*(1+row[side+'_fee']) for side in ('long','short')):
                rejected[strategy+':capital'] += 1
                continue
            p = dict(row, strategy=strategy, id=cycle_id+':'+strategy, opened=now, opened_cycle=cycle_id,
                long_symbol=lm['symbol'], short_symbol=sm['symbol'], next_funding={'long':lm.get('next') or None,'short':sm.get('next') or None},
                funding_intervals={'long':lm.get('interval'),'short':sm.get('interval')}, funding_events={}, funding_estimate=0.0, funding_pending=[], target=.0015*n, stop=.01*n,
                max_hold=21600 if strategy=='convergence' else 86400, valuation_stale=False)
            if not row['funding_metadata_known']:
                p['funding_pending']=['unknown_schedule']
            p['last_value']=exit_value(p, books[(p['coin'],p['long'])], books[(p['coin'],p['short'])])
            p['last_valued']=now
            state['positions'].append(p)
            trades.append(dict(event='open', position=p.copy()))
    state['last_cycle'], state['cycles'] = cycle_id, state['cycles']+1
    return dict(observations=observations, trades=trades, rejections=dict(rejected))


def report(state, result, now, next_tick, errors, selected, count):
    totals = {}
    for strategy in STRATEGIES:
        opened = [p for p in state['positions'] if p['strategy']==strategy]
        closed = [p for p in state['closed'] if p['strategy']==strategy]
        realized = sum(p['pnl'] for p in closed)
        unrealized = sum(p['last_value']['net'] for p in opened)
        reserved = {v:sum(p['quantity']*p[side+'_entry'] for p in opened for side in ('long','short') if p[side]==v) for v in VENUES}
        totals[strategy] = dict(closed=len(closed), open=len(opened), modeled_closed_pnl=realized,
            modeled_open_exit_pnl=unrealized, modeled_equity=3000+realized+unrealized,
            incomplete_funding_positions=sum(bool(p.get('funding_pending')) for p in opened+closed),
            stale_open_valuations=sum(p.get('valuation_stale',False) for p in opened), capital_by_venue=state['capital'][strategy], reserved_by_venue=reserved)
    rows = sorted(result.get('observations', []), key=lambda r:r['convergence_potential'], reverse=True)
    return dict(heartbeat=utc(now), next_tick=utc(next_tick) if next_tick else None, cycles=state['cycles'],
        common_universe=count, sampled_coins=selected, valid_routes=len(rows), qualified_openings=sum(t['event']=='open' for t in result.get('trades',[])),
        rejections=result.get('rejections',{}), errors=errors, strategies=totals, positions=state['positions'],
        best_convergence=rows[:10], best_funding=sorted([r for r in rows if r['forecast_funding_24h'] is not None], key=lambda r:r['forecast_funding_24h'], reverse=True)[:10], assumptions=ASSUMPTIONS)


def render(r):
    lines=['# Public-data paper experiment', '', 'Heartbeat: '+r['heartbeat'], 'Next tick: '+str(r['next_tick']),
        f"Cycles: {r['cycles']}; common contracts: {r['common_universe']}; valid routes: {r['valid_routes']}",
        'Coins: '+', '.join(r['sampled_coins']), '', '| Strategy | Closed / open | Closed modeled PnL | Open exit PnL | Equity | Funding incomplete | Stale |', '|---|---:|---:|---:|---:|---:|---:|']
    for s,x in r['strategies'].items():
        lines.append(f"| {s} | {x['closed']} / {x['open']} | {x['modeled_closed_pnl']:.4f} | {x['modeled_open_exit_pnl']:.4f} | {x['modeled_equity']:.4f} | {x['incomplete_funding_positions']} | {x['stale_open_valuations']} |")
    lines += ['', '## Best observed routes (potential assumes convergence; not executable profit)', '', '| Coin | Long | Short | Potential USDT | Funding forecast / 24h |', '|---|---|---|---:|---:|']
    for x in r['best_convergence']:
        lines.append(f"| {x['coin']} | {x['long']} | {x['short']} | {x['convergence_potential']:.4f} | {x['forecast_funding_24h']} |")
    lines += ['', '## Rejections', '', json.dumps(r['rejections']), '', '## Errors', '', json.dumps(r['errors']), '', '## Assumptions', ''] + ['- '+x for x in ASSUMPTIONS]
    return '\n'.join(lines)+'\n'


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--once',action='store_true')
    parser.add_argument('--interval',type=int,default=300)
    parser.add_argument('--duration-hours',type=float,default=24)
    parser.add_argument('--output-dir',type=Path,required=True)
    parser.add_argument('--nominal',type=float,default=100)
    parser.add_argument('--max-coins',type=int,default=12)
    args=parser.parse_args()
    if args.interval<30 or not 0<args.duration_hours<=168 or not 0<args.nominal<=1000 or not 1<=args.max_coins<=30:
        parser.error('interval >=30; 0<duration<=168h; 0<nominal<=1000; 1<=max-coins<=30 required')
    args.output_dir.mkdir(parents=True,exist_ok=True)
    lock=(args.output_dir/'.lock').open('w')
    try:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    except BlockingIOError:
        parser.error('another runner owns output directory')
    state_path=args.output_dir/'state.json'
    state=load_state(state_path)
    deadline=(time.time()+args.duration_hours*3600) if args.once else state.setdefault('run_deadline',time.time()+args.duration_hours*3600)
    if not args.once:
        atomic_json(state_path,state)
    stopping=False
    def stop(*_):
        nonlocal stopping
        stopping=True
    signal.signal(signal.SIGTERM,stop)
    signal.signal(signal.SIGINT,stop)
    while not stopping and time.time()<deadline:
        started=time.time()
        cycle_id=str(int(started//args.interval))
        next_tick=None if args.once else min(deadline,started+args.interval)
        try:
            markets,errors=discover()
            selected,count=universe(markets,args.max_coins,time.time())
            selected=list(dict.fromkeys(selected+[p['coin'] for p in state['positions']]))
            keys=[(c,v) for c in selected for v in VENUES if c in markets[v]]
            books,book_errors=parallel(keys,lambda k:fetch_book(k[1],markets[k[1]][k[0]]))
            errors.update(book_errors)
            result=cycle(state,markets,books,time.time(),args.nominal,cycle_id,decision_clock=time.time)
            # State is source of truth. Journal is observational; a crash between files can omit an event, never replay a position.
            atomic_json(state_path,state)
            append_json(args.output_dir/'observations.jsonl',dict(cycle_id=cycle_id,time=utc(),books={c+':'+v:b for (c,v),b in books.items()},markets=markets,**result))
            for trade in result['trades']:
                append_json(args.output_dir/'trades.jsonl',dict(cycle_id=cycle_id,**trade))
            r=report(state,result,time.time(),next_tick,errors,selected,count)
        except Exception as exc:
            for p in state['positions']:
                p['valuation_stale']=True
            atomic_json(state_path,state)
            r=report(state,{},time.time(),next_tick,{'cycle':str(exc)},[],0)
            append_json(args.output_dir/'observations.jsonl',dict(cycle_id=cycle_id,time=utc(),error=str(exc)))
        atomic_json(args.output_dir/'latest.json',r)
        tmp=args.output_dir/'latest.md.tmp'
        tmp.write_text(render(r))
        os.replace(tmp,args.output_dir/'latest.md')
        print(json.dumps({k:r[k] for k in ('heartbeat','cycles','valid_routes','qualified_openings','errors')}),flush=True)
        if args.once:
            break
        while not stopping and time.time()<next_tick:
            time.sleep(max(0,min(1,next_tick-time.time())))
    atomic_json(args.output_dir/'run_status.json',dict(stopped_at=utc(),reason='once' if args.once else 'signal' if stopping else 'duration_limit',open_positions=len(state['positions'])))


if __name__=='__main__':
    main()
