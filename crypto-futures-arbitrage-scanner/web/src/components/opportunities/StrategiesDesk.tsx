import { Activity, ArrowRight, BarChart3, FlaskConical, GitCompareArrows, Waves } from 'lucide-react';

import type { ScannerState } from '../../app/types';
import { buildAdaptiveConvergenceSignals, type FuturesConvergenceCandidate } from '../../lib/opportunity-desks';

interface StrategiesDeskProps {
  candidates: FuturesConvergenceCandidate[];
  history: ScannerState['history'];
}

const strategies = [
  {
    name: 'Trend Pullback',
    family: 'Directional · 15m closed candles',
    description: 'EMA20/EMA50 trend structure, ATR invalidation, RSI confirmation, and visible liquidity targets.',
    icon: BarChart3,
    status: 'Analyze on demand',
  },
  {
    name: 'Breakout',
    family: 'Directional · 15m closed candles',
    description: 'Recent range pressure, volatility-adjusted trigger, structural stop, and minimum risk/reward gate.',
    icon: Activity,
    status: 'Analyze on demand',
  },
  {
    name: 'Mean Reversion',
    family: 'Directional · 15m closed candles',
    description: 'Volatility-band extremes and RSI exhaustion with liquidity-aware targets and explicit invalidation.',
    icon: Waves,
    status: 'Analyze on demand',
  },
] as const;

export function StrategiesDesk({ candidates, history }: StrategiesDeskProps) {
  const signals = buildAdaptiveConvergenceSignals(candidates, history);
  const qualifiedSignals = signals.filter((signal) => signal.status === 'qualified');
  const positiveCandidates = signals.filter((candidate) => candidate.estimatedNetSpreadPct > 0);
  const leader = qualifiedSignals[0] ?? positiveCandidates[0];

  return (
    <section className="space-y-4" aria-labelledby="strategy-registry-title">
      <div className="rounded-2xl border border-terminal-line bg-terminal-panel p-5">
        <p className="font-data text-[10px] uppercase tracking-[0.2em] text-signal-mint">Composable research engines</p>
        <h2 className="mt-1 text-2xl font-semibold" id="strategy-registry-title">Strategy registry</h2>
        <p className="mt-2 max-w-3xl text-sm text-slate-500">
          Every strategy owns its signal cadence, data requirements, risk gates, and execution eligibility. A ranked signal is never treated as an order by itself.
        </p>
      </div>

      <article className="overflow-hidden rounded-2xl border border-signal-mint/30 bg-[linear-gradient(115deg,#ffffff_0%,#f0fdf7_100%)] shadow-[0_14px_40px_rgba(8,122,76,0.08)]">
        <div className="grid gap-6 p-5 lg:grid-cols-[1.1fr_0.9fr]">
          <div>
            <div className="flex items-center gap-3">
              <span className="grid size-11 place-items-center rounded-xl bg-signal-mint text-white"><GitCompareArrows aria-hidden="true" size={20} /></span>
              <div>
                <p className="font-data text-[10px] uppercase tracking-[0.16em] text-signal-mint">Live research · 1 second</p>
                <h3 className="mt-0.5 text-lg font-semibold">Adaptive Cross-Venue Convergence</h3>
              </div>
            </div>
            <p className="mt-4 text-sm leading-6 text-slate-600">
              The H-style model: long the discounted perpetual, short the premium venue, then exit as the spread returns toward its own rolling equilibrium—not toward zero by assumption.
            </p>
            <div className="mt-4 grid gap-2 sm:grid-cols-2">
              {['Executable bid/ask, never last price', 'Open + close fees before ranking', 'Funding and depth required before execution', 'One-hour robust Z-score; half-life gate next'].map((item) => (
                <div className="flex items-center gap-2 rounded-lg border border-emerald-100 bg-white/80 px-3 py-2 text-xs text-slate-600" key={item}>
                  <span className="size-1.5 shrink-0 rounded-full bg-signal-mint" />{item}
                </div>
              ))}
            </div>
          </div>
          <div className="rounded-xl border border-emerald-100 bg-white p-4">
            <div className="flex items-center justify-between">
              <span className="text-xs text-slate-500">Positive screening candidates</span>
              <span className="rounded-full bg-emerald-50 px-2 py-1 font-data text-xs text-signal-mint">{qualifiedSignals.length} qualified</span>
            </div>
            {leader ? (
              <div className="mt-5">
                <p className="font-data text-xl font-semibold">{leader.symbol.replace('USDT', '/USDT')}</p>
                <p className="mt-2 text-xs text-slate-500">Long {leader.longSource.replace('_futures', '')} · Short {leader.shortSource.replace('_futures', '')}</p>
                <p className="mt-5 font-data text-3xl font-semibold text-signal-mint">+{leader.estimatedNetSpreadPct.toFixed(3)}%</p>
                <p className="mt-1 text-[11px] text-slate-400">estimated net convergence screen</p>
                {leader.status === 'insufficient_history' ? (
                  <p className="mt-4 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700">Collecting one-hour paired history · {leader.sampleCount}/720 samples</p>
                ) : (
                  <dl className="mt-4 grid grid-cols-3 gap-2 border-t border-terminal-line pt-3 text-center">
                    <div><dt className="text-[9px] uppercase text-slate-400">Normal</dt><dd className="mt-1 font-data text-xs">{leader.baselineMedianPct?.toFixed(3)}%</dd></div>
                    <div><dt className="text-[9px] uppercase text-slate-400">Deviation</dt><dd className="mt-1 font-data text-xs">{leader.deviationFromNormalPct !== null && leader.deviationFromNormalPct >= 0 ? '+' : ''}{leader.deviationFromNormalPct?.toFixed(3)}%</dd></div>
                    <div><dt className="text-[9px] uppercase text-slate-400">Robust Z</dt><dd className="mt-1 font-data text-xs">{leader.zScore?.toFixed(2)}</dd></div>
                  </dl>
                )}
              </div>
            ) : <p className="mt-8 text-sm text-slate-500">No positive fee-adjusted candidate is fresh right now.</p>}
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-emerald-100 bg-white/70 px-5 py-3">
          <span className="inline-flex items-center gap-2 text-xs text-amber-700"><FlaskConical aria-hidden="true" size={14} />Research state: execution requires historical calibration and live depth validation.</span>
          <a className="inline-flex items-center gap-1 text-xs font-semibold text-signal-mint hover:underline" href="/futures">Open Futures Lab <ArrowRight aria-hidden="true" size={13} /></a>
        </div>
      </article>

      <div className="grid gap-4 lg:grid-cols-3">
        {strategies.map(({ description, family, icon: Icon, name, status }) => (
          <article className="flex min-h-64 flex-col rounded-2xl border border-terminal-line bg-terminal-panel p-5" key={name}>
            <div className="flex items-start justify-between gap-3">
              <span className="grid size-10 place-items-center rounded-xl border border-terminal-line bg-slate-50 text-slate-600"><Icon aria-hidden="true" size={18} /></span>
              <span className="rounded-full bg-slate-100 px-2 py-1 font-data text-[9px] uppercase tracking-wide text-slate-500">{status}</span>
            </div>
            <p className="mt-5 font-data text-[10px] uppercase tracking-[0.13em] text-slate-400">{family}</p>
            <h3 className="mt-1 text-lg font-semibold">{name}</h3>
            <p className="mt-3 flex-1 text-sm leading-6 text-slate-500">{description}</p>
            <a className="mt-5 inline-flex items-center gap-1 text-xs font-semibold text-signal-mint hover:underline" href="/futures">Analyze in Futures Lab <ArrowRight aria-hidden="true" size={13} /></a>
          </article>
        ))}
      </div>
    </section>
  );
}
