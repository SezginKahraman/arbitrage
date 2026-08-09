import { ArrowDown, ArrowUp, ChevronDown, ChevronUp, ChevronsUpDown } from 'lucide-react';

import type { ArbitrageOpportunity, OpportunitySortField, UiPreferences } from '../../app/types';
import type { OpportunityHistoryState } from '../../hooks/useOpportunityHistory';
import { formatPrice } from '../../lib/format';
import { SourceMark } from '../shared/SourceMark';

interface OpportunitiesTableProps {
  opportunities: ArbitrageOpportunity[];
  liveCount: number;
  historyStatus: OpportunityHistoryState['status'];
  onRetryHistory: () => void;
  onSort: (field: OpportunitySortField) => void;
  sort: UiPreferences['sort'];
  collapsed: boolean;
  onToggleCollapsed: () => void;
}

const columns: Array<{ field: OpportunitySortField; label: string; aria: string; className: string }> = [
  { field: 'symbol', label: 'Pair', aria: 'pair', className: 'px-5' },
  { field: 'buy_source', label: 'Buy', aria: 'buy source', className: 'px-4' },
  { field: 'sell_source', label: 'Sell', aria: 'sell source', className: 'px-4' },
  { field: 'profit', label: 'Gross spread', aria: 'gross spread', className: 'px-4' },
  { field: 'timestamp', label: 'Updated', aria: 'updated time', className: 'px-5 text-right' },
];

export function OpportunitiesTable({
  historyStatus,
  onRetryHistory,
  onSort,
  opportunities,
  liveCount,
  sort,
  collapsed,
  onToggleCollapsed,
}: OpportunitiesTableProps) {
  return (
    <section className="overflow-hidden rounded-xl border border-terminal-line bg-terminal-panel/65">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-terminal-line px-5 py-4">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="font-display text-lg font-medium">Live opportunities</h2>
            <span className="rounded-full border border-terminal-line bg-black/15 px-2 py-0.5 font-data text-[10px] text-slate-400">
              {liveCount} live
            </span>
          </div>
          {historyStatus === 'degraded' ? (
            <p className="mt-1 text-xs text-signal-amber">Opportunity history is unavailable. Live scanning is unaffected.</p>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          {historyStatus === 'degraded' ? (
            <button
              aria-label="Retry opportunity history"
              className="rounded-md border border-signal-amber/30 px-3 py-1.5 text-xs text-signal-amber transition hover:bg-signal-amber/10"
              onClick={onRetryHistory}
              type="button"
            >
              Retry
            </button>
          ) : historyStatus === 'loading' ? (
            <span className="text-xs text-slate-500">Loading history…</span>
          ) : null}
          <button
            aria-controls="opportunities-table-body"
            aria-expanded={!collapsed}
            aria-label={collapsed ? 'Expand live opportunities' : 'Collapse live opportunities'}
            className="grid size-9 place-items-center rounded-lg border border-terminal-line text-slate-400 transition hover:bg-white/[0.035] hover:text-terminal-text"
            onClick={onToggleCollapsed}
            type="button"
          >
            {collapsed ? <ChevronDown aria-hidden="true" size={17} /> : <ChevronUp aria-hidden="true" size={17} />}
          </button>
        </div>
      </header>
      {!collapsed ? <div className="overflow-x-auto" id="opportunities-table-body">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead className="text-xs text-slate-500">
            <tr className="border-b border-terminal-line">
              {columns.map((column) => (
                <th
                  aria-sort={sort.field === column.field ? (sort.direction === 'asc' ? 'ascending' : 'descending') : 'none'}
                  className={`${column.className} py-3 font-medium`}
                  key={column.field}
                >
                  <button
                    aria-label={`Sort by ${column.aria}`}
                    className={`inline-flex items-center gap-1.5 hover:text-terminal-text ${column.field === 'timestamp' ? 'ml-auto' : ''}`}
                    onClick={() => onSort(column.field)}
                    type="button"
                  >
                    {column.label}
                    {sort.field === column.field ? (
                      sort.direction === 'asc' ? <ArrowUp aria-hidden="true" className="text-signal-mint" size={13} /> : <ArrowDown aria-hidden="true" className="text-signal-mint" size={13} />
                    ) : (
                      <ChevronsUpDown aria-hidden="true" className="opacity-45" size={13} />
                    )}
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {opportunities.length ? (
              opportunities.map((opportunity, index) => (
                <tr
                  className={`border-b border-terminal-line/70 last:border-0 ${index === 0 ? 'bg-signal-mint/[0.045]' : 'hover:bg-white/[0.025]'}`}
                  key={opportunity.id}
                >
                  <td className="px-5 py-4 font-data font-medium">
                    <div className="flex items-center gap-2">
                      {opportunity.symbol.replace('USDT', '/USDT')}
                      {opportunity.historical ? (
                        <span className="rounded border border-terminal-line bg-white/[0.035] px-1.5 py-0.5 font-sans text-[10px] font-medium text-slate-400">
                          History
                        </span>
                      ) : (
                        <span className="rounded border border-signal-mint/20 bg-signal-mint/10 px-1.5 py-0.5 font-sans text-[10px] font-medium text-signal-mint">
                          Live
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-4">
                    <SourceMark source={opportunity.buySource} />
                    <p className="mt-1 font-data text-xs text-slate-400">${formatPrice(opportunity.buyPrice)}</p>
                  </td>
                  <td className="px-4 py-4">
                    <SourceMark source={opportunity.sellSource} />
                    <p className="mt-1 font-data text-xs text-slate-400">${formatPrice(opportunity.sellPrice)}</p>
                  </td>
                  <td className="px-4 py-4 font-data font-medium text-signal-mint">
                    +{opportunity.profitPct.toFixed(2)}%
                    {opportunity.historical && opportunity.peakProfitPct !== undefined ? (
                      <p className="mt-1 font-sans text-[10px] font-normal text-slate-500">
                        Peak {opportunity.peakProfitPct.toFixed(2)}%
                      </p>
                    ) : null}
                  </td>
                  <td className="px-5 py-4 text-right font-data text-xs text-slate-400">
                    {new Date(opportunity.timestamp).toLocaleTimeString([], { hour12: false })}
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-5 py-14 text-center text-slate-500" colSpan={5}>
                  No opportunities match the current filters.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div> : null}
    </section>
  );
}
