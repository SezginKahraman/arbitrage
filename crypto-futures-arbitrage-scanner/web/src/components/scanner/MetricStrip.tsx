import { Activity, BookOpen, Radio, SlidersHorizontal, TrendingUp } from 'lucide-react';

interface MetricStripProps {
  activeOpportunities: number;
  bestSpread: number | null;
  connectedFeeds: number;
  totalFeeds: number;
  freshBooks: number;
  totalBooks: number;
  minSpread: number;
}

const metricClass =
  'min-w-[220px] flex-1 rounded-xl border border-terminal-line bg-terminal-panel/65 px-5 py-4 shadow-[inset_0_1px_rgba(255,255,255,0.025)]';

export function MetricStrip({ activeOpportunities, bestSpread, connectedFeeds, totalFeeds, freshBooks, totalBooks, minSpread }: MetricStripProps) {
  return (
    <section aria-label="Scanner metrics" className="flex gap-3 overflow-x-auto pb-1">
      <article className={metricClass}>
        <div className="flex items-center justify-between text-sm text-slate-400"><span>Best spread</span><TrendingUp size={20} /></div>
        <p className="mt-2 font-data text-2xl text-signal-mint">{bestSpread === null ? '—' : `+${bestSpread.toFixed(2)}%`}</p>
      </article>
      <article className={metricClass}>
        <div className="flex items-center justify-between text-sm text-slate-400"><span>Active opportunities</span><Activity size={20} /></div>
        <p className="mt-2 font-data text-2xl">{activeOpportunities}</p>
      </article>
      <article className={metricClass}>
        <div className="flex items-center justify-between text-sm text-slate-400"><span>Feeds connected</span><Radio size={20} /></div>
        <p aria-label={`${connectedFeeds} of ${totalFeeds} feeds connected`} className="mt-2 font-data text-2xl text-signal-mint">{connectedFeeds} / {totalFeeds}</p>
      </article>
      <article className={metricClass}>
        <div className="flex items-center justify-between text-sm text-slate-400"><span>Books fresh</span><BookOpen size={20} /></div>
        <p aria-label={`${freshBooks} of ${totalBooks} books fresh`} className="mt-2 font-data text-2xl text-signal-mint">{freshBooks} / {totalBooks}</p>
      </article>
      <article className={metricClass}>
        <div className="flex items-center justify-between text-sm text-slate-400"><span>Minimum spread</span><SlidersHorizontal size={20} /></div>
        <p className="mt-2 font-data text-2xl">{minSpread.toFixed(2)}%</p>
      </article>
    </section>
  );
}
