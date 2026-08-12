import { CheckCircle2, CircleHelp, ShieldAlert, XCircle } from 'lucide-react';

import type { TransferRouteEvaluation, TransferRouteRequestStatus, VenueAssetNetwork } from '../../app/types';
import { sourceMeta } from '../../lib/sources';
import { transferRoutePresentation } from '../../lib/transfer-route';

interface ExecutionChecksProps {
  route: TransferRouteEvaluation | null;
  requestStatus: TransferRouteRequestStatus;
}

const toneClass = {
  positive: 'text-signal-mint',
  warning: 'text-signal-amber',
  negative: 'text-red-400',
  neutral: 'text-slate-400',
} as const;

function Availability({ enabled, label }: { enabled: boolean; label: string }) {
  const Icon = enabled ? CheckCircle2 : XCircle;
  return (
    <span className={`inline-flex items-center gap-1 ${enabled ? 'text-signal-mint' : 'text-red-400'}`}>
      <Icon aria-hidden="true" size={13} />
      {label} {enabled ? 'open' : 'closed'}
    </span>
  );
}

function NetworkList({ asset, networks, source }: { asset: string; networks: VenueAssetNetwork[]; source: string }) {
  return (
    <article className="rounded-lg border border-terminal-line bg-black/10 p-3">
      <h3 className="font-medium">{sourceMeta(source).label} networks</h3>
      {networks.length === 0 ? (
        <p className="mt-3 text-xs text-slate-500">No network metadata is available.</p>
      ) : (
        <div className="mt-3 space-y-2">
          {networks.map((network) => (
            <div className="rounded-md border border-terminal-line/70 bg-black/10 p-3" key={`${network.networkID}:${network.rawNetworkID}`}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="font-data text-sm">{network.name || network.rawNetworkID}</p>
                <span className="text-[11px] uppercase tracking-wide text-slate-500">{network.rawNetworkID}</span>
              </div>
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs">
                <Availability enabled={network.depositEnabled} label="Deposit" />
                <Availability enabled={network.withdrawEnabled} label="Withdrawal" />
              </div>
              {(network.withdrawalFee || network.minimumWithdrawal) && (
                <div className="mt-2 flex flex-wrap gap-3 text-xs text-slate-400">
                  {network.withdrawalFee && <span>Fee {network.withdrawalFee} {asset}</span>}
                  {network.minimumWithdrawal && <span>Minimum {network.minimumWithdrawal} {asset}</span>}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </article>
  );
}

export function ExecutionChecks({ route, requestStatus }: ExecutionChecksProps) {
  const presentation = transferRoutePresentation(route, requestStatus);
  const commonNetwork = route?.networks.find((network) => network.status === 'ready' || network.status === 'check');
  const showVenueNetworks = Boolean(route && (route.sourceNetworks.length || route.destinationNetworks.length));

  return (
    <section className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-4">
      <div className="mb-4 flex items-center gap-2">
        <ShieldAlert aria-hidden="true" className={toneClass[presentation.tone]} size={19} />
        <h2 className="font-display text-lg font-medium">Execution checks</h2>
      </div>
      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <article className="rounded-lg border border-terminal-line bg-black/10 p-3">
          <CircleHelp aria-hidden="true" className={`mb-3 ${toneClass[presentation.tone]}`} size={18} />
          <p className="text-sm">Transfer route</p>
          <p className={`mt-1 text-sm font-medium ${toneClass[presentation.tone]}`}>{presentation.label}</p>
          <p className="mt-2 text-xs leading-5 text-slate-500">{presentation.detail}</p>
        </article>
        <article className="rounded-lg border border-terminal-line bg-black/10 p-3">
          <CircleHelp aria-hidden="true" className="mb-3 text-signal-amber" size={18} />
          <p className="text-sm">Common network</p>
          <p className="mt-1 text-sm font-medium text-slate-300">{commonNetwork?.name || 'Not verified'}</p>
          <p className="mt-2 text-xs leading-5 text-slate-500">Direction must support source withdrawal and destination deposit.</p>
        </article>
        <article className="rounded-lg border border-terminal-line bg-black/10 p-3">
          <CircleHelp aria-hidden="true" className="mb-3 text-signal-amber" size={18} />
          <p className="text-sm">Trading fees</p>
          <p className="mt-1 text-sm font-medium text-signal-amber">Not configured</p>
          <p className="mt-2 text-xs leading-5 text-slate-500">Gross spread does not include account fees.</p>
        </article>
        <article className="rounded-lg border border-terminal-line bg-black/10 p-3">
          <CircleHelp aria-hidden="true" className="mb-3 text-signal-amber" size={18} />
          <p className="text-sm">Liquidity</p>
          <p className="mt-1 text-sm font-medium text-signal-amber">Unknown</p>
          <p className="mt-2 text-xs leading-5 text-slate-500">Top-of-book size is not normalized yet.</p>
        </article>
      </div>
      {showVenueNetworks && route && (
        <div className="mt-3 grid gap-3 lg:grid-cols-2">
          <NetworkList asset={route.asset} networks={route.sourceNetworks} source={route.source} />
          <NetworkList asset={route.asset} networks={route.destinationNetworks} source={route.destination} />
        </div>
      )}
    </section>
  );
}
