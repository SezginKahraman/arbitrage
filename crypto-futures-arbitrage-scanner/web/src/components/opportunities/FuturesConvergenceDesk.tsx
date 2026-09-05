import { Activity, ArrowDownLeft, ArrowUpRight, ExternalLink, Gauge, ShieldCheck } from 'lucide-react';
import { useMemo, useState } from 'react';

import { formatPrice } from '../../lib/format';
import type { FuturesConvergenceCandidate } from '../../lib/opportunity-desks';
import { SourceMark } from '../shared/SourceMark';

interface FuturesConvergenceDeskProps {
  candidates: FuturesConvergenceCandidate[];
  now: number;
}

function relativeTime(timestamp: number, now: number) {
  const seconds = Math.max(0, Math.floor((now - timestamp) / 1_000));
  return seconds < 2 ? 'now' : `${seconds}s ago`;
}

export function FuturesConvergenceDesk({ candidates, now }: FuturesConvergenceDeskProps) {
  const [minimumNet, setMinimumNet] = useState('0');
  const [query, setQuery] = useState('');
  const minimum = Number(minimumNet);
  const visible = useMemo(() => candidates.filter((candidate) => {
    if (Number.isFinite(minimum) && candidate.estimatedNetSpreadPct < minimum) return false;
    return !query.trim() || candidate.symbol.toLowerCase().includes(query.trim().toLowerCase());
  }), [candidates, minimum, query]);
  const positive = candidates.filter((candidate) => candidate.estimatedNetSpreadPct > 0).length;
  const analyzable = candidates.filter((candidate) => candidate.support === 'analyze').length;

  return (
    <section className="space-y-4" aria-labelledby="futures-convergence-title">
      <div className="flex flex-wrap items-end justify-between gap-4 rounded-2xl border border-terminal-line bg-terminal-panel p-5">
        <div>
          <p className="font-data text-[10px] uppercase tracking-[0.2em] text-signal-mint">Perpetual spread tape</p>
          <h2 className="mt-1 text-2xl font-semibold" id="futures-convergence-title">Futures convergence</h2>
          <p className="mt-2 max-w-3xl text-sm text-slate-500">
            Long the lowest executable ask and short the highest executable bid across Binance, Gate.io, and KuCoin. Ranked after estimated open and close taker fees.
          </p>
        </div>
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">
          Screening signal only · depth, funding, contract size, and account fees require analysis.
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        {[
          { label: 'Fresh routes', value: candidates.length, icon: Activity },
          { label: 'Positive after estimate', value: positive, icon: Gauge },
          { label: 'Binance ↔ Gate analyzable', value: analyzable, icon: ShieldCheck },
        ].map(({ icon: Icon, label, value }) => (
          <div className="rounded-xl border border-terminal-line bg-terminal-panel p-4" key={label}>
            <div className="flex items-center justify-between text-slate-500"><span className="text-xs">{label}</span><Icon aria-hidden="true" size={16} /></div>
            <p className="mt-2 font-data text-2xl text-terminal-text">{value}</p>
          </div>
        ))}
      </div>

      <div className="grid gap-3 rounded-xl border border-terminal-line bg-terminal-panel p-3 md:grid-cols-[1fr_190px]">
        <input aria-label="Search futures opportunities" className="control" onChange={(event) => setQuery(event.target.value)} placeholder="Search futures pair…" type="search" value={query} />
        <label className="relative">
          <span className="absolute left-3 top-1.5 text-[9px] uppercase tracking-wide text-slate-400">Min est. net</span>
          <input aria-label="Minimum estimated net spread" className="control pt-3 font-data" min="-10" onChange={(event) => setMinimumNet(event.target.value)} step="0.05" type="number" value={minimumNet} />
          <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 font-data text-xs text-slate-400">%</span>
        </label>
      </div>

      <div className="overflow-hidden rounded-2xl border border-terminal-line bg-terminal-panel">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1080px] text-left text-sm">
            <thead className="bg-slate-50 text-[10px] uppercase tracking-[0.13em] text-slate-500">
              <tr className="border-b border-terminal-line">
                <th className="px-4 py-3 font-medium">#</th>
                <th className="px-4 py-3 font-medium">Pair</th>
                <th className="px-4 py-3 font-medium">LONG</th>
                <th className="px-4 py-3 font-medium">SHORT</th>
                <th className="px-4 py-3 font-medium">Gross</th>
                <th className="px-4 py-3 font-medium">4-fill fees</th>
                <th className="px-4 py-3 font-medium">Est. net</th>
                <th className="px-4 py-3 font-medium">Support</th>
                <th className="px-4 py-3 text-right font-medium">Freshness</th>
              </tr>
            </thead>
            <tbody>
              {visible.length ? visible.map((candidate, index) => (
                <tr className="border-b border-terminal-line/80 last:border-0 hover:bg-emerald-50/35" key={candidate.id}>
                  <td className="px-4 py-4 font-data text-xs text-slate-400">{index + 1}</td>
                  <td className="px-4 py-4 font-data font-semibold">{candidate.symbol.replace('USDT', '/USDT')}</td>
                  <td className="px-4 py-4">
                    <div className="mb-1 flex items-center gap-1 font-data text-[9px] font-semibold uppercase tracking-wide text-signal-mint"><ArrowDownLeft aria-hidden="true" size={11} />LONG</div>
                    <SourceMark source={candidate.longSource} />
                    <p className="mt-1 font-data text-xs text-slate-500">ask ${formatPrice(candidate.longEntry)}</p>
                  </td>
                  <td className="px-4 py-4">
                    <div className="mb-1 flex items-center gap-1 font-data text-[9px] font-semibold uppercase tracking-wide text-rose-600"><ArrowUpRight aria-hidden="true" size={11} />SHORT</div>
                    <SourceMark source={candidate.shortSource} />
                    <p className="mt-1 font-data text-xs text-slate-500">bid ${formatPrice(candidate.shortEntry)}</p>
                  </td>
                  <td className="px-4 py-4 font-data">{candidate.grossSpreadPct >= 0 ? '+' : ''}{candidate.grossSpreadPct.toFixed(3)}%</td>
                  <td className="px-4 py-4 font-data text-slate-500">−{candidate.estimatedRoundTripFeePct.toFixed(2)}%</td>
                  <td className={`px-4 py-4 font-data text-base font-semibold ${candidate.estimatedNetSpreadPct > 0 ? 'text-signal-mint' : 'text-rose-600'}`}>
                    {candidate.estimatedNetSpreadPct >= 0 ? '+' : ''}{candidate.estimatedNetSpreadPct.toFixed(3)}%
                  </td>
                  <td className="px-4 py-4">
                    {candidate.support === 'analyze' ? (
                      <a className="inline-flex items-center gap-1 rounded-md border border-signal-mint/25 bg-emerald-50 px-2 py-1 text-[11px] font-medium text-signal-mint hover:border-signal-mint" href={`/futures?contract=${candidate.symbol.replace('USDT', '_USDT')}`}>
                        Analyze supported <ExternalLink aria-hidden="true" size={11} />
                      </a>
                    ) : <span className="rounded-md border border-slate-200 bg-slate-50 px-2 py-1 text-[11px] text-slate-500">Public scan only</span>}
                  </td>
                  <td className="px-4 py-4 text-right font-data text-xs text-slate-400">{relativeTime(candidate.timestamp, now)}</td>
                </tr>
              )) : <tr><td className="px-5 py-16 text-center text-slate-500" colSpan={9}>No fresh futures route matches this filter.</td></tr>}
            </tbody>
          </table>
        </div>
        <footer className="border-t border-terminal-line bg-slate-50/70 px-4 py-3 text-xs text-slate-500">
          Conservative screen uses 0.05% per Binance fill, 0.075% per Gate fill, and 0.06% per KuCoin fill. Actual account and contract rates may differ.
        </footer>
      </div>
    </section>
  );
}
