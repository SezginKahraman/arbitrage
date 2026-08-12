import { ArrowRight, BadgeCheck, CircleDollarSign, CircleX, ShoppingCart, Star, TriangleAlert, WalletCards } from 'lucide-react';

import type { ArbitrageOpportunity, ConnectionStatus, SymbolName, TransferRouteEvaluation, TransferRouteRequestStatus } from '../../app/types';
import { formatPrice } from '../../lib/format';
import { transferRoutePresentation } from '../../lib/transfer-route';
import { SourceMark } from '../shared/SourceMark';
import { StatusBadge } from '../shared/StatusBadge';

interface OpportunityRouteProps {
  opportunity: ArbitrageOpportunity | null;
  connection: ConnectionStatus;
  symbol: SymbolName;
  freshBooks: number;
  totalBooks: number;
  minSpread: number;
  transferRoute: TransferRouteEvaluation | null;
  transferRouteStatus: TransferRouteRequestStatus;
}

function pairLabel(symbol: string): string {
  return symbol.replace('USDT', '/USDT');
}

const emptyCopy: Record<ConnectionStatus, string> = {
  connecting: 'Connecting to live market feeds…',
  reconnecting: 'Market connection interrupted; reconnecting…',
  offline: 'Market connection is offline.',
  live: '',
};

export function OpportunityRoute({
  connection,
  opportunity,
  symbol,
  freshBooks,
  totalBooks,
  minSpread,
  transferRoute,
  transferRouteStatus,
}: OpportunityRouteProps) {
  const liveEmptyCopy = freshBooks < 2
    ? `Waiting for fresh order books — ${freshBooks} / ${totalBooks} available.`
    : `No executable route currently clears the ${minSpread.toFixed(2)}% threshold.`;
  const routeEmptyCopy = connection === 'live' ? liveEmptyCopy : emptyCopy[connection];
  const transferPresentation = transferRoutePresentation(transferRoute, transferRouteStatus);
  const TransferIcon = transferPresentation.tone === 'positive'
    ? BadgeCheck
    : transferPresentation.tone === 'negative' ? CircleX : TriangleAlert;
  const transferToneClass = transferPresentation.tone === 'positive'
    ? 'text-signal-mint'
    : transferPresentation.tone === 'negative' ? 'text-red-400' : transferPresentation.tone === 'neutral' ? 'text-slate-400' : 'text-signal-amber';

  return (
    <section className="overflow-hidden rounded-2xl border border-signal-mint/55 bg-[linear-gradient(115deg,rgba(39,229,140,0.055),rgba(12,23,29,0.88)_34%,rgba(12,23,29,0.98))] shadow-[0_20px_80px_rgba(0,0,0,0.22)]">
      <div className="grid min-h-40 items-center gap-6 p-5 lg:grid-cols-[minmax(220px,1.05fr)_minmax(180px,0.8fr)_auto_minmax(180px,0.8fr)_minmax(190px,0.85fr)] lg:p-6">
        <div className="space-y-4 lg:border-r lg:border-terminal-line lg:pr-6">
          <div className="flex items-center gap-3">
            <Star aria-hidden="true" className="text-signal-mint" size={26} />
            <h2 className="font-display text-3xl font-semibold tracking-tight">{pairLabel(symbol)}</h2>
          </div>
          <StatusBadge tone={opportunity ? 'positive' : 'neutral'}>
            {opportunity ? 'Best observed opportunity' : 'Waiting for a qualifying route'}
          </StatusBadge>
        </div>

        {opportunity ? (
          <>
            <div className="grid grid-cols-[46px_1fr] gap-3">
              <span className="grid size-11 place-items-center rounded-full bg-signal-mint/10 text-signal-mint">
                <ShoppingCart aria-hidden="true" size={21} />
              </span>
              <div>
                <p className="text-[11px] uppercase tracking-[0.16em] text-slate-500">Buy on</p>
                <p className="mt-1 font-medium"><SourceMark source={opportunity.buySource} /></p>
                <p className="mt-2 font-data text-xl">${formatPrice(opportunity.buyPrice)}</p>
              </div>
            </div>

            <div className="flex items-center gap-3 text-signal-mint lg:flex-col">
              <ArrowRight aria-hidden="true" className="hidden lg:block" size={22} />
              <div className="min-w-40 rounded-xl border border-terminal-line bg-black/20 px-5 py-4 text-center">
                <p className="text-[10px] uppercase tracking-[0.16em] text-slate-400">Gross spread</p>
                <p className="mt-1 font-data text-3xl font-medium">+{opportunity.profitPct.toFixed(2)}%</p>
                <p className="mt-2 text-[11px] text-slate-500">Net estimate unavailable</p>
              </div>
            </div>

            <div className="grid grid-cols-[46px_1fr] gap-3">
              <span className="grid size-11 place-items-center rounded-full bg-signal-mint/10 text-signal-mint">
                <WalletCards aria-hidden="true" size={21} />
              </span>
              <div>
                <p className="text-[11px] uppercase tracking-[0.16em] text-slate-500">Sell on</p>
                <p className="mt-1 font-medium"><SourceMark source={opportunity.sellSource} /></p>
                <p className="mt-2 font-data text-xl">${formatPrice(opportunity.sellPrice)}</p>
              </div>
            </div>

            <div className={`flex items-center gap-3 border-t border-terminal-line pt-5 lg:border-l lg:border-t-0 lg:pl-6 lg:pt-0 ${transferToneClass}`}>
              <TransferIcon aria-hidden="true" className="shrink-0" size={28} />
              <div>
                <p className="font-medium">{transferPresentation.label}</p>
                <p className="mt-1 text-xs text-slate-500">{transferPresentation.detail}</p>
              </div>
            </div>
          </>
        ) : (
          <div className="flex items-center gap-3 text-slate-400 lg:col-span-4">
            <CircleDollarSign aria-hidden="true" />
            {routeEmptyCopy}
          </div>
        )}
      </div>
    </section>
  );
}
