import { ArrowDownRight, ArrowUpRight, CircleAlert } from 'lucide-react';

import type { FuturesScenario } from '../../app/types';
import { formatPrice } from '../../lib/format';

const statusCopy = {
  ready: { label: 'Ready', classes: 'border-signal-mint/30 bg-signal-mint/10 text-signal-mint' },
  watch: { label: 'Watch', classes: 'border-signal-amber/30 bg-signal-amber/10 text-signal-amber' },
  no_trade: { label: 'No trade', classes: 'border-slate-700 bg-slate-800/50 text-slate-400' },
};

interface ScenarioCardProps {
  scenario: FuturesScenario;
  active: boolean;
  onSelect: () => void;
}

export function ScenarioCard({ scenario, active, onSelect }: ScenarioCardProps) {
  const long = scenario.direction === 'long';
  const status = statusCopy[scenario.status];
  const accent = long ? 'text-signal-mint' : 'text-[#a62f2a]';
  const Icon = long ? ArrowUpRight : ArrowDownRight;

  return (
    <section
      aria-label={`${long ? 'Long' : 'Short'} scenario`}
      className={`rounded-xl border p-4 transition ${active ? long ? 'border-signal-mint/50 bg-signal-mint/[0.045]' : 'border-[#ff6b6b]/45 bg-[#ff6b6b]/[0.035]' : 'border-terminal-line bg-terminal-ink/35'}`}
      role="region"
    >
      <button className="flex w-full items-center justify-between text-left" onClick={onSelect} type="button">
        <span className={`flex items-center gap-2 font-data text-sm font-semibold uppercase tracking-[0.14em] ${accent}`}>
          <Icon aria-hidden="true" size={17} />{scenario.direction}
        </span>
        <span className={`rounded-md border px-2 py-1 text-[10px] font-semibold uppercase tracking-wider ${status.classes}`}>
          {status.label}
        </span>
      </button>

      {scenario.entry > 0 ? <>
        <div className="mt-4 grid grid-cols-3 gap-2 font-data">
          <Metric label="Entry" value={`$${formatPrice(scenario.entry)}`} />
          <Metric label="Stop" value={`$${formatPrice(scenario.stop)}`} tone="risk" />
          <Metric label="Target" value={scenario.targets[0] ? `$${formatPrice(scenario.targets[0])}` : '—'} tone="target" />
        </div>
        <div className="mt-3 grid grid-cols-2 gap-2 border-y border-terminal-line/70 py-3">
          <Metric label="Risk / reward" value={`${scenario.riskReward.toFixed(2)}R`} />
          <Metric label="Setup score" value={`${scenario.score}/100`} />
          <Metric label="Risk amount" value={`$${scenario.riskAmount.toFixed(2)}`} />
          <Metric label="Est. round-trip fees" value={`$${scenario.estimatedFees.toFixed(3)}`} />
        </div>
      </> : null}

      <ul className="mt-3 space-y-2 text-xs leading-5 text-slate-400">
        {scenario.reasons.map((reason) => <li className="flex gap-2" key={reason}><CircleAlert aria-hidden="true" className="mt-0.5 shrink-0" size={13} />{reason}</li>)}
      </ul>
    </section>
  );
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: 'risk' | 'target' }) {
  return (
    <div className="min-w-0">
      <div className="text-[9px] uppercase tracking-wider text-slate-600">{label}</div>
      <div className={`mt-1 truncate text-xs ${tone === 'risk' ? 'text-[#a62f2a]' : tone === 'target' ? 'text-[#2563eb]' : 'text-terminal-text'}`}>{value}</div>
    </div>
  );
}
