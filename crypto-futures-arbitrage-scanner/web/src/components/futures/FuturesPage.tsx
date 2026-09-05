import { Activity, AlertTriangle, BookOpen, CheckCircle2, Gauge, LoaderCircle, Radio, RefreshCw, ShieldCheck, Waves, Zap } from 'lucide-react';
import { useEffect, useMemo, useState, type FormEvent } from 'react';

import type { FuturesAnalyzeInput, FuturesBookLevel, FuturesInterval, FuturesStrategy, NeutralRoute } from '../../app/types';
import { useFuturesWorkspace, type FuturesWorkspace } from '../../hooks/useFuturesWorkspace';
import { formatPrice } from '../../lib/format';
import { FuturesChart } from './FuturesChart';
import { ScenarioCard } from './ScenarioCard';

const preferencesKey = 'arbitrage.futures.preferences.v1';

interface FuturesPreferences {
  contract: string;
  interval: FuturesInterval;
  strategy: FuturesStrategy;
  accountBalance: number;
  riskPercent: number;
  tradeNotional: number;
  minNetSpreadPct: number;
}

const defaultPreferences: FuturesPreferences = {
  contract: '', interval: '15m', strategy: 'trend_pullback', accountBalance: 1_000, riskPercent: 1,
  tradeNotional: 100, minNetSpreadPct: 0.1,
};

function readPreferences(): FuturesPreferences {
  try {
    const value = JSON.parse(localStorage.getItem(preferencesKey) ?? '{}') as Partial<FuturesPreferences>;
    const requestedContract = new URLSearchParams(window.location.search).get('contract') ?? '';
    const linkedContract = /^[A-Z0-9]{1,24}_USDT$/.test(requestedContract) ? requestedContract : '';
    return {
      contract: linkedContract || (typeof value.contract === 'string' ? value.contract : ''),
      interval: value.interval === '1h' || value.interval === '4h' ? value.interval : '15m',
      strategy: value.strategy === 'breakout' || value.strategy === 'mean_reversion' ? value.strategy : 'trend_pullback',
      accountBalance: typeof value.accountBalance === 'number' && value.accountBalance >= 10 ? value.accountBalance : 1_000,
      riskPercent: typeof value.riskPercent === 'number' && value.riskPercent > 0 && value.riskPercent <= 5 ? value.riskPercent : 1,
      tradeNotional: typeof value.tradeNotional === 'number' && value.tradeNotional >= 5 ? value.tradeNotional : 100,
      minNetSpreadPct: typeof value.minNetSpreadPct === 'number' && value.minNetSpreadPct >= 0 ? value.minNetSpreadPct : 0.1,
    };
  } catch {
    return defaultPreferences;
  }
}

export function FuturesPage() {
  const workspace = useFuturesWorkspace();
  return <FuturesPageView workspace={workspace} />;
}

export function FuturesPageView({ workspace }: { workspace: FuturesWorkspace }) {
  const [preferences, setPreferences] = useState<FuturesPreferences>(readPreferences);
  const [direction, setDirection] = useState<'long' | 'short'>('long');

  useEffect(() => {
    if (!workspace.contracts.length) return;
    if (!workspace.contracts.some((item) => item.symbol === preferences.contract)) {
      setPreferences((current) => ({ ...current, contract: workspace.contracts[0].symbol }));
    }
  }, [preferences.contract, workspace.contracts]);

  useEffect(() => {
    localStorage.setItem(preferencesKey, JSON.stringify(preferences));
  }, [preferences]);

  useEffect(() => {
    if (!workspace.analysis) return;
    if (workspace.analysis.long.status === 'no_trade' && workspace.analysis.short.status !== 'no_trade') setDirection('short');
  }, [workspace.analysis]);

  const selectedContract = useMemo(
    () => workspace.contracts.find((item) => item.symbol === preferences.contract),
    [preferences.contract, workspace.contracts],
  );

  function submit(event: FormEvent) {
    event.preventDefault();
    const input: FuturesAnalyzeInput = {
      contract: preferences.contract, interval: preferences.interval, strategy: preferences.strategy,
      accountBalance: preferences.accountBalance, riskPercent: preferences.riskPercent,
      tradeNotional: preferences.tradeNotional, minNetSpreadPct: preferences.minNetSpreadPct,
    };
    void workspace.analyze(input);
  }

  return (
    <div className="space-y-4">
      <header className="flex flex-wrap items-end justify-between gap-4 px-1 pt-1">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">Futures decision lab</h1>
            <span className="rounded-md border border-signal-amber/35 bg-signal-amber/10 px-2 py-1 font-data text-[10px] uppercase tracking-[0.18em] text-signal-amber">Manual live execution</span>
          </div>
          <p className="mt-2 max-w-3xl text-sm text-slate-400">Analyze Gate.io technical setups and Binance ↔ Gate futures convergence routes from live prices, depth, fees, and funding.</p>
        </div>
        <div className="flex items-center gap-2 text-xs text-slate-500"><ShieldCheck aria-hidden="true" size={16} className="text-signal-mint" />Analyze never trades. Only the execution button sends orders.</div>
      </header>

      <form className="rounded-2xl border border-terminal-line bg-terminal-panel/70 p-4" onSubmit={submit}>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[1.3fr_0.7fr_1.05fr_0.72fr_0.62fr_0.72fr_0.72fr_auto] xl:items-end">
          <Field label="Gate contract">
            <select
              aria-label="Gate contract"
              className="control"
              disabled={workspace.contractsStatus !== 'ready'}
              onChange={(event) => setPreferences((current) => ({ ...current, contract: event.target.value }))}
              value={preferences.contract}
            >
              {!preferences.contract && <option value="">Loading contracts…</option>}
              {workspace.contracts.map((item) => <option key={item.symbol} value={item.symbol}>{item.symbol.replace('_', '/')}</option>)}
            </select>
          </Field>
          <Field label="Timeframe">
            <select aria-label="Timeframe" className="control" onChange={(event) => setPreferences((current) => ({ ...current, interval: event.target.value as FuturesInterval }))} value={preferences.interval}>
              <option value="15m">15 minutes</option><option value="1h">1 hour</option><option value="4h">4 hours</option>
            </select>
          </Field>
          <Field label="Strategy">
            <select aria-label="Strategy" className="control" onChange={(event) => setPreferences((current) => ({ ...current, strategy: event.target.value as FuturesStrategy }))} value={preferences.strategy}>
              <option value="trend_pullback">Trend pullback</option><option value="breakout">Range breakout</option><option value="mean_reversion">Mean reversion</option>
            </select>
          </Field>
          <Field label="Account balance">
            <input aria-label="Account balance" className="control font-data" min="10" onChange={(event) => setPreferences((current) => ({ ...current, accountBalance: Number(event.target.value) }))} step="10" type="number" value={preferences.accountBalance} />
          </Field>
          <Field label="Risk %">
            <input aria-label="Risk per setup" className="control font-data" max="5" min="0.1" onChange={(event) => setPreferences((current) => ({ ...current, riskPercent: Number(event.target.value) }))} step="0.05" type="number" value={preferences.riskPercent} />
          </Field>
          <Field label="Trade size USDT">
            <input aria-label="Trade size USDT" className="control font-data" min="5" onChange={(event) => setPreferences((current) => ({ ...current, tradeNotional: Number(event.target.value) }))} step="5" type="number" value={preferences.tradeNotional} />
          </Field>
          <Field label="Min net spread %">
            <input aria-label="Minimum net spread" className="control font-data" min="0" onChange={(event) => setPreferences((current) => ({ ...current, minNetSpreadPct: Number(event.target.value) }))} step="0.01" type="number" value={preferences.minNetSpreadPct} />
          </Field>
          <button
            className="flex h-11 items-center justify-center gap-2 rounded-xl bg-signal-mint px-5 text-sm font-bold text-white transition hover:bg-[#06653f] disabled:cursor-not-allowed disabled:opacity-45"
            disabled={!preferences.contract || workspace.analysisStatus === 'loading'}
            type="submit"
          >
            {workspace.analysisStatus === 'loading' ? <LoaderCircle aria-hidden="true" className="animate-spin" size={17} /> : <Activity aria-hidden="true" size={17} />}
            Analyze market
          </button>
        </div>
        {selectedContract && <div className="mt-3 flex flex-wrap gap-x-6 gap-y-1 border-t border-terminal-line/70 pt-3 font-data text-[10px] text-slate-500">
          <span>Mark ${formatPrice(selectedContract.markPrice)}</span>
          <span>Funding {(selectedContract.fundingRate * 100).toFixed(4)}%</span>
          <span>Max venue leverage {selectedContract.leverageMax.toFixed(0)}×</span>
          <span>Tick {formatPrice(selectedContract.priceIncrement)}</span>
        </div>}
      </form>

      {workspace.error && <div className="rounded-xl border border-[#c2413b]/35 bg-[#c2413b]/10 px-4 py-3 text-sm text-[#a62f2a]" role="alert">{workspace.error}</div>}

      {!workspace.analysis ? (
        <section className="grid min-h-[420px] place-items-center rounded-2xl border border-dashed border-terminal-line bg-terminal-panel/35 px-6 text-center">
          <div><Waves aria-hidden="true" className="mx-auto text-slate-600" size={36} /><h2 className="mt-4 text-lg font-medium">Choose a contract and run an analysis.</h2><p className="mt-2 max-w-lg text-sm leading-6 text-slate-500">Analysis reads live public market data. No order is sent until you explicitly execute a qualified route.</p></div>
        </section>
      ) : <>
        <section className="grid gap-px overflow-hidden rounded-2xl border border-terminal-line bg-terminal-line sm:grid-cols-2 xl:grid-cols-5">
          <MarketMetric icon={Gauge} label="Regime" value={`${capitalize(workspace.analysis.indicators.marketBias)} regime`} />
          <MarketMetric icon={Activity} label="RSI 14" value={workspace.analysis.indicators.rsi14.toFixed(1)} />
          <MarketMetric icon={Waves} label="ATR 14" value={`$${formatPrice(workspace.analysis.indicators.atr14)}`} />
          <MarketMetric icon={BookOpen} label="Mark / index" value={`${formatPrice(workspace.analysis.contract.markPrice)} / ${formatPrice(workspace.analysis.contract.indexPrice)}`} />
          <MarketMetric icon={RefreshCw} label="Funding" value={`${(workspace.analysis.contract.fundingRate * 100).toFixed(4)}%`} />
        </section>

        <NeutralRoutePanel workspace={workspace} />

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(340px,0.75fr)]">
          <FuturesChart analysis={workspace.analysis} direction={direction} liveCandle={workspace.liveSnapshot?.candle ?? null} />
          <aside className="space-y-3 rounded-2xl border border-terminal-line bg-terminal-panel/55 p-3">
            <div className="px-1 py-2"><p className="font-data text-[10px] uppercase tracking-[0.22em] text-slate-500">Decision rail</p><h2 className="mt-1 text-lg font-medium">Two-sided plan</h2></div>
            <ScenarioCard active={direction === 'long'} onSelect={() => setDirection('long')} scenario={workspace.analysis.long} />
            <ScenarioCard active={direction === 'short'} onSelect={() => setDirection('short')} scenario={workspace.analysis.short} />
          </aside>
        </div>

        <LiquidityPanel analysis={workspace.analysis} />
      </>}

    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block"><span className="mb-1.5 block text-[11px] font-medium text-slate-400">{label}</span>{children}</label>;
}

function MarketMetric({ icon: Icon, label, value }: { icon: typeof Activity; label: string; value: string }) {
  return <div className="flex items-center gap-3 bg-terminal-panel px-4 py-3"><Icon aria-hidden="true" className="text-slate-500" size={17} /><div><div className="text-[10px] uppercase tracking-wider text-slate-600">{label}</div><div className="mt-1 font-data text-sm text-terminal-text">{value}</div></div></div>;
}

function LiquidityPanel({ analysis }: { analysis: NonNullable<FuturesWorkspace['analysis']> }) {
  return (
    <section className="grid gap-4 rounded-2xl border border-terminal-line bg-terminal-panel/55 p-4 md:grid-cols-2">
      <LiquiditySide label="Bid liquidity" levels={analysis.liquidity.bidWalls} tone="bid" />
      <LiquiditySide label="Ask liquidity" levels={analysis.liquidity.askWalls} tone="ask" />
    </section>
  );
}

function LiquiditySide({ label, levels, tone }: { label: string; levels: FuturesBookLevel[]; tone: 'bid' | 'ask' }) {
  const maxNotional = Math.max(...levels.map((level) => level.notional), 1);
  return <div><h3 className={`text-sm font-medium ${tone === 'bid' ? 'text-signal-mint' : 'text-[#a62f2a]'}`}>{label}</h3><div className="mt-3 space-y-2">{levels.length ? levels.map((level) => <div className="relative overflow-hidden rounded-lg border border-terminal-line bg-terminal-ink/45 px-3 py-2" key={`${tone}-${level.price}`}><div className={`absolute inset-y-0 left-0 ${tone === 'bid' ? 'bg-signal-mint/[0.07]' : 'bg-[#c2413b]/[0.07]'}`} style={{ width: `${Math.max(5, level.notional / maxNotional * 100)}%` }} /><div className="relative flex justify-between gap-3 font-data text-xs"><span>${formatPrice(level.price)}</span><span className="text-slate-500">${level.notional.toLocaleString('en-US', { maximumFractionDigits: 0 })}</span></div></div>) : <p className="text-xs text-slate-500">No visible wall in the requested depth.</p>}</div></div>;
}

function NeutralRoutePanel({ workspace }: { workspace: FuturesWorkspace }) {
  const route = workspace.liveSnapshot?.route ?? workspace.analysis?.neutralRoute ?? null;
  const executed = workspace.executionResult?.status === 'opened';
  const executable = route?.status === 'executable';
  const statusTone = executable ? 'text-signal-mint' : route?.status === 'watch' ? 'text-signal-amber' : 'text-[#a62f2a]';
  const StatusIcon = executable ? CheckCircle2 : AlertTriangle;

  return (
    <section aria-label="Binance Gate neutral route" className="overflow-hidden rounded-2xl border border-terminal-line bg-terminal-panel/60" role="region">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-terminal-line px-5 py-4">
        <div>
          <div className="flex items-center gap-2"><Radio aria-hidden="true" className={workspace.liveStatus === 'ready' ? 'text-signal-mint' : 'text-slate-600'} size={16} /><p className="font-data text-[10px] uppercase tracking-[0.22em] text-slate-500">Live convergence route</p></div>
          <h2 className="mt-1 text-lg font-medium">Binance Futures ↔ Gate Futures</h2>
        </div>
        <div className={`flex items-center gap-2 text-xs font-medium ${statusTone}`}><StatusIcon aria-hidden="true" size={16} />{route ? routeStatusLabel(route) : 'Waiting for analysis'}</div>
      </header>

      {!route ? <p className="px-5 py-8 text-sm text-slate-500">Analyze the selected contract to compare both futures order books.</p> : <>
        <div className="grid gap-px bg-terminal-line sm:grid-cols-2 xl:grid-cols-7">
          <RouteMetric label="Long cheaper venue" value={venueLabel(route.longVenue)} subvalue={route.longEntry > 0 ? `$${formatPrice(route.longEntry)}` : '—'} />
          <RouteMetric label="Short richer venue" value={venueLabel(route.shortVenue)} subvalue={route.shortEntry > 0 ? `$${formatPrice(route.shortEntry)}` : '—'} />
          <RouteMetric label="Gross spread" value={`${signed(route.grossSpreadPct)}%`} />
          <RouteMetric label="Projected net" value={`${signed(route.netConvergenceSpreadPct)}%`} accent={executable} subvalue="after 4 taker fees" />
          <RouteMetric label="Open + close fees" value={`$${(route.openFees + route.estimatedCloseFees).toFixed(4)}`} />
          <RouteMetric label="Next funding carry" value={`${signed(route.nextFundingCarryPct)}%`} subvalue="shown separately" />
          <RouteMetric label="Index divergence" value={`${route.indexDivergencePct.toFixed(3)}%`} />
        </div>
        <div className="flex flex-col gap-4 px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <div className="font-data text-xs text-slate-400">Matched base {route.baseQuantity.toLocaleString('en-US', { maximumFractionDigits: 8 })} · Gate {route.longVenue === 'gate_futures' ? route.longContracts : route.shortContracts} contracts · Binance {route.longVenue === 'binance_futures' ? route.longContracts : route.shortContracts} quantity</div>
            <ul className="mt-2 space-y-1 text-xs text-slate-500">{route.reasons.map((reason) => <li key={reason}>• {reason}</li>)}</ul>
            <p className="mt-2 flex items-center gap-2 text-xs font-medium text-signal-amber"><AlertTriangle aria-hidden="true" size={14} />This sends two real market orders. A fresh server-side check runs first; exits must currently be managed on both exchanges.</p>
          </div>
          <button
            aria-label="Execute both futures legs"
            className="flex h-11 shrink-0 items-center justify-center gap-2 rounded-xl border border-signal-amber/55 bg-signal-amber/10 px-5 text-sm font-bold text-signal-amber transition hover:bg-signal-amber/15 disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-800/40 disabled:text-slate-600"
            disabled={!executable || workspace.executionStatus === 'loading' || executed}
            onClick={() => void workspace.executeRoute()}
            type="button"
          >
            {workspace.executionStatus === 'loading' ? <LoaderCircle aria-hidden="true" className="animate-spin" size={17} /> : <Zap aria-hidden="true" size={17} />}
            {executed ? 'Position opened' : workspace.executionStatus === 'loading' ? 'Revalidating & executing…' : 'Execute both futures legs'}
          </button>
        </div>
      </>}

      {workspace.executionResult && <div className={`border-t border-terminal-line px-5 py-4 text-xs text-slate-400 ${workspace.executionResult.status === 'exposed' ? 'bg-[#ff6b6b]/[0.06]' : 'bg-signal-mint/[0.045]'}`}>
        <strong className={workspace.executionResult.status === 'exposed' ? 'text-[#a62f2a]' : 'text-signal-mint'}>Execution {workspace.executionResult.status}.</strong>{' '}
        {workspace.executionResult.reasonCode && `${workspace.executionResult.reasonCode.replaceAll('_', ' ')} · `}
        {workspace.executionResult.fills.map((fill) => `${venueLabel(fill.venue)} #${fill.orderId}`).join(' · ')}
        {workspace.executionResult.compensations.length > 0 && ` · Compensated ${workspace.executionResult.compensations.map((fill) => `${venueLabel(fill.venue)} #${fill.orderId}`).join(' + ')}`}
        {workspace.executionResult.compensations.length === 0 && workspace.executionResult.compensation && ` · Compensated ${venueLabel(workspace.executionResult.compensation.venue)} #${workspace.executionResult.compensation.orderId}`}
      </div>}
    </section>
  );
}

function RouteMetric({ label, value, subvalue, accent = false }: { label: string; value: string; subvalue?: string; accent?: boolean }) {
  return <div className="min-w-0 bg-terminal-panel px-4 py-3"><div className="text-[9px] uppercase tracking-wider text-slate-600">{label}</div><div className={`mt-1 truncate font-data text-sm ${accent ? 'text-signal-mint' : 'text-terminal-text'}`}>{value}</div>{subvalue && <div className="mt-1 truncate text-[9px] text-slate-600">{subvalue}</div>}</div>;
}

function routeStatusLabel(route: NeutralRoute) {
  if (route.status === 'executable') return 'Executable now';
  if (route.status === 'watch') return 'Watch — below threshold';
  return route.reasonCode ? `Rejected — ${route.reasonCode.replaceAll('_', ' ')}` : 'Rejected';
}

function venueLabel(value: string) {
  if (value === 'binance_futures') return 'Binance Futures';
  if (value === 'gate_futures') return 'Gate Futures';
  return value || 'Unavailable';
}

function signed(value: number) {
  return `${value >= 0 ? '+' : ''}${value.toFixed(3)}`;
}

function capitalize(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
