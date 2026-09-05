import { ArrowLeftRight, ChartNoAxesCombined, Columns2, Rows3, WalletCards } from 'lucide-react';

import type { ComparisonMode, DashboardLayout } from '../../app/types';
import { SOURCES, sourceMatchesComparisonMode } from '../../lib/sources';

interface MarketControlsProps {
  comparisonMode: ComparisonMode;
  dashboardLayout: DashboardLayout;
  enabledSources: Record<string, boolean>;
  onComparisonModeChange: (mode: ComparisonMode) => void;
  onDashboardLayoutChange: (layout: DashboardLayout) => void;
  onSourceToggle: (source: string) => void;
}

const modes: Array<{
  key: ComparisonMode;
  label: string;
  aria: string;
  icon: typeof WalletCards;
}> = [
  { key: 'spot', label: 'Spot', aria: 'Compare spot markets', icon: WalletCards },
  { key: 'futures', label: 'Futures', aria: 'Compare futures markets', icon: ChartNoAxesCombined },
  { key: 'mixed', label: 'Spot ↔ Futures', aria: 'Compare spot and futures markets', icon: ArrowLeftRight },
];

export function MarketControls({
  comparisonMode,
  dashboardLayout,
  enabledSources,
  onComparisonModeChange,
  onDashboardLayoutChange,
  onSourceToggle,
}: MarketControlsProps) {
  const visibleSources = SOURCES.filter((source) => sourceMatchesComparisonMode(source.key, comparisonMode));

  return (
    <section
      aria-label="Market comparison controls"
      className="rounded-xl border border-terminal-line bg-terminal-panel/90 p-3 shadow-[0_1px_2px_rgba(15,56,45,0.04)]"
    >
      <div className="flex flex-wrap items-center gap-3">
        <div aria-label="Comparison mode" className="flex rounded-lg border border-terminal-line bg-terminal-ink/70 p-1" role="group">
          {modes.map(({ key, label, aria, icon: Icon }) => (
            <button
              aria-label={aria}
              aria-pressed={comparisonMode === key}
              className={`inline-flex items-center gap-2 rounded-md px-3 py-2 text-xs font-medium transition ${comparisonMode === key ? 'bg-signal-mint/10 text-signal-mint shadow-[inset_0_0_0_1px_rgba(8,122,76,0.22)]' : 'text-slate-500 hover:bg-slate-900/[0.04] hover:text-terminal-text'}`}
              key={key}
              onClick={() => onComparisonModeChange(key)}
              type="button"
            >
              <Icon aria-hidden="true" size={15} />
              {label}
            </button>
          ))}
        </div>

        <div className="h-7 w-px bg-terminal-line max-md:hidden" />

        <div aria-label="Visible markets" className="flex min-w-0 flex-1 gap-2 overflow-x-auto py-1" role="group">
          {visibleSources.map((source) => {
            const enabled = enabledSources[source.key] !== false;
            return (
              <button
                aria-label={`${enabled ? 'Disable' : 'Enable'} ${source.label}`}
                aria-pressed={enabled}
                className={`inline-flex shrink-0 items-center gap-2 rounded-full border px-3 py-1.5 text-xs transition ${enabled ? 'border-terminal-line bg-slate-900/[0.035] text-terminal-text' : 'border-transparent bg-black/[0.04] text-slate-500'}`}
                key={source.key}
                onClick={() => onSourceToggle(source.key)}
                type="button"
              >
                <span className={`size-1.5 rounded-full ${enabled ? '' : 'opacity-30'}`} style={{ backgroundColor: source.color }} />
                {source.label}
              </button>
            );
          })}
        </div>

        <div aria-label="Panel layout" className="flex rounded-lg border border-terminal-line bg-terminal-ink/70 p-1" role="group">
          <button
            aria-label="Show table and chart side by side"
            aria-pressed={dashboardLayout === 'split'}
            className={`grid size-8 place-items-center rounded-md ${dashboardLayout === 'split' ? 'bg-slate-900/[0.07] text-terminal-text' : 'text-slate-500 hover:text-terminal-text'}`}
            onClick={() => onDashboardLayoutChange('split')}
            title="Side by side"
            type="button"
          >
            <Columns2 aria-hidden="true" size={16} />
          </button>
          <button
            aria-label="Stack table and chart"
            aria-pressed={dashboardLayout === 'stacked'}
            className={`grid size-8 place-items-center rounded-md ${dashboardLayout === 'stacked' ? 'bg-slate-900/[0.07] text-terminal-text' : 'text-slate-500 hover:text-terminal-text'}`}
            onClick={() => onDashboardLayoutChange('stacked')}
            title="Stacked"
            type="button"
          >
            <Rows3 aria-hidden="true" size={16} />
          </button>
        </div>
      </div>
    </section>
  );
}
