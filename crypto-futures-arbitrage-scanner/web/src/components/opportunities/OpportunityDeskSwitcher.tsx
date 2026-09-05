import { Activity, Boxes, Network } from 'lucide-react';

export type OpportunityDesk = 'spot' | 'futures' | 'strategies';

interface OpportunityDeskSwitcherProps {
  active: OpportunityDesk;
  futuresCount: number;
  onChange: (desk: OpportunityDesk) => void;
  spotCount: number;
}

const desks = [
  { key: 'spot', label: 'Spot transfers', cadence: 'network verified', icon: Network },
  { key: 'futures', label: 'Futures convergence', cadence: '1s live scan', icon: Activity },
  { key: 'strategies', label: 'Strategies', cadence: 'modular registry', icon: Boxes },
] as const;

export function OpportunityDeskSwitcher({ active, futuresCount, onChange, spotCount }: OpportunityDeskSwitcherProps) {
  return (
    <nav aria-label="Opportunity desks" className="overflow-hidden rounded-2xl border border-terminal-line bg-terminal-panel shadow-[0_12px_34px_rgba(15,56,45,0.06)]">
      <div className="grid md:grid-cols-3">
        {desks.map(({ cadence, icon: Icon, key, label }) => {
          const count = key === 'spot' ? spotCount : key === 'futures' ? futuresCount : 4;
          const selected = active === key;
          return (
            <button
              aria-label={`Open ${label} desk`}
              aria-pressed={selected}
              className={`group relative flex min-h-20 items-center gap-3 border-b border-terminal-line px-5 text-left transition last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0 ${selected ? 'bg-emerald-50/70' : 'hover:bg-slate-50'}`}
              key={key}
              onClick={() => onChange(key)}
              type="button"
            >
              <span className={`grid size-10 shrink-0 place-items-center rounded-xl border ${selected ? 'border-signal-mint/30 bg-white text-signal-mint' : 'border-terminal-line bg-slate-50 text-slate-500'}`}>
                <Icon aria-hidden="true" size={18} />
              </span>
              <span className="min-w-0 flex-1">
                <span className={`block text-sm font-semibold ${selected ? 'text-terminal-text' : 'text-slate-600'}`}>{label}</span>
                <span className="mt-1 block font-data text-[10px] uppercase tracking-[0.13em] text-slate-400">{cadence}</span>
              </span>
              <span className={`rounded-full px-2 py-1 font-data text-[10px] ${selected ? 'bg-signal-mint text-white' : 'bg-slate-100 text-slate-500'}`}>{count}</span>
              {selected ? <span className="absolute inset-x-0 bottom-0 h-0.5 bg-signal-mint" /> : null}
            </button>
          );
        })}
      </div>
    </nav>
  );
}
