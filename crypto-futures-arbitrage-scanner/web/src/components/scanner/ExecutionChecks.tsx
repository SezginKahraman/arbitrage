import { CircleHelp, ShieldAlert } from 'lucide-react';

const checks = [
  ['Transfer route', 'Unverified', 'Deposit and withdrawal networks are not checked.'],
  ['Common network', 'Unknown', 'No normalized network metadata is available.'],
  ['Trading fees', 'Not configured', 'Gross spread does not include account fees.'],
  ['Liquidity', 'Unknown', 'Top-of-book size is not normalized yet.'],
] as const;

export function ExecutionChecks() {
  return (
    <section className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-4">
      <div className="mb-4 flex items-center gap-2">
        <ShieldAlert aria-hidden="true" className="text-signal-amber" size={19} />
        <h2 className="font-display text-lg font-medium">Execution checks</h2>
      </div>
      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        {checks.map(([label, value, detail]) => (
          <article className="rounded-lg border border-terminal-line bg-black/10 p-3" key={label}>
            <CircleHelp aria-hidden="true" className="mb-3 text-signal-amber" size={18} />
            <p className="text-sm">{label}</p>
            <p className="mt-1 text-sm font-medium text-signal-amber">{label === 'Transfer route' ? 'Transfer route unverified' : value}</p>
            <p className="mt-2 text-xs leading-5 text-slate-500">{detail}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
