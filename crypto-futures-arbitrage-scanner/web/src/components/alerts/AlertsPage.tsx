import { BellRing, Check, Clock3, Plus, Search, ShieldCheck, Volume2, VolumeX } from 'lucide-react';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';

import {
  SYMBOLS,
  type AlertMarketMode,
  type AlertRule,
  type AlertRuleInput,
  type AlertTrigger,
  type ScannerState,
} from '../../app/types';
import { formatPrice } from '../../lib/format';
import { SOURCES, sourceMeta } from '../../lib/sources';
import { type AlertFetcher, useAlerts } from '../../hooks/useAlerts';
import { SourceMark } from '../shared/SourceMark';

interface AlertsPageProps {
  state: ScannerState;
  fetcher?: AlertFetcher;
  now?: number;
}

const executableSources = SOURCES.filter((source) => source.market !== 'oracle');
const marketLabels: Record<AlertMarketMode, string> = {
  all: 'Any route', spot: 'Spot ↔ Spot', mixed: 'Spot ↔ Futures', futures: 'Futures ↔ Futures',
};

const initialDraft: AlertRuleInput = {
  name: '', symbol: 'COTIUSDT', marketMode: 'all', buySource: '', sellSource: '',
  minSpreadPct: 0.5, cooldownSeconds: 300, enabled: true, browserEnabled: true,
};

function inputFromRule(rule: AlertRule): AlertRuleInput {
  const { name, symbol, marketMode, buySource, sellSource, minSpreadPct, cooldownSeconds, enabled, browserEnabled } = rule;
  return { name, symbol, marketMode, buySource, sellSource, minSpreadPct, cooldownSeconds, enabled, browserEnabled };
}

function relativeTime(timestamp: number | null, now: number) {
  if (!timestamp) return 'Never';
  const seconds = Math.max(0, Math.floor((now - timestamp) / 1_000));
  if (seconds < 2) return 'now';
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3_600) return `${Math.floor(seconds / 60)}m ago`;
  return `${Math.floor(seconds / 3_600)}h ago`;
}

function RuleList({ rules, now, onToggle, loading }: { rules: AlertRule[]; now: number; onToggle: (rule: AlertRule) => void; loading: boolean }) {
  const [query, setQuery] = useState('');
  const visible = rules.filter((rule) => `${rule.name} ${rule.symbol}`.toLowerCase().includes(query.trim().toLowerCase()));
  return (
    <section className="overflow-hidden rounded-xl border border-terminal-line bg-terminal-panel/65">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-terminal-line p-4">
        <div>
          <h2 className="font-display text-lg font-semibold">Alert rules</h2>
          <p className="mt-0.5 text-xs text-slate-500">Saved in SQLite and evaluated against live executable routes.</p>
        </div>
        <label className="relative">
          <span className="sr-only">Search alert rules</span>
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-600" size={15} />
          <input aria-label="Search alert rules" className="h-9 rounded-lg border border-terminal-line bg-terminal-ink pl-9 pr-3 text-xs" onChange={(event) => setQuery(event.target.value)} placeholder="Search rules" value={query} />
        </label>
      </header>
      {loading ? <p className="px-5 py-16 text-center text-sm text-slate-500">Loading saved rules…</p> : visible.length ? <ul className="divide-y divide-terminal-line/80">
        {visible.map((rule) => (
          <li className="flex items-center gap-4 p-4" key={rule.id}>
            <span className={`grid size-9 shrink-0 place-items-center rounded-lg border ${rule.enabled ? 'border-signal-mint/25 bg-signal-mint/10 text-signal-mint' : 'border-terminal-line bg-black/10 text-slate-600'}`}>
              {rule.enabled ? <Volume2 size={17} /> : <VolumeX size={17} />}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">{rule.name}</p>
              <p className="mt-1 truncate font-data text-[11px] text-slate-500">
                {rule.symbol ? rule.symbol.replace('USDT', '/USDT') : 'All pairs'} · {marketLabels[rule.marketMode]} · ≥ {rule.minSpreadPct.toFixed(2)}%
              </p>
            </div>
            <div className="hidden text-right font-data text-[11px] text-slate-500 sm:block">
              <p>{rule.cooldownSeconds >= 60 ? `${rule.cooldownSeconds / 60}m` : `${rule.cooldownSeconds}s`} cooldown</p>
              <p className="mt-1">{relativeTime(rule.lastTriggeredAtMS, now)}</p>
            </div>
            <button
              aria-checked={rule.enabled}
              aria-label={`${rule.enabled ? 'Mute' : 'Enable'} ${rule.name}`}
              className={`relative h-6 w-11 rounded-full transition ${rule.enabled ? 'bg-signal-mint' : 'bg-slate-700'}`}
              onClick={() => onToggle(rule)}
              role="switch"
              type="button"
            >
              <span className={`absolute top-1 size-4 rounded-full bg-terminal-ink transition ${rule.enabled ? 'left-6' : 'left-1'}`} />
            </button>
          </li>
        ))}
      </ul> : <p className="px-5 py-16 text-center text-sm text-slate-500">No alert rules yet.</p>}
    </section>
  );
}

function AlertEditor({ onSave }: { onSave: (input: AlertRuleInput) => Promise<void> }) {
  const [draft, setDraft] = useState(initialDraft);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const update = <K extends keyof AlertRuleInput>(field: K, value: AlertRuleInput[K]) => setDraft((current) => ({ ...current, [field]: value }));
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!draft.name.trim()) { setError('Rule name is required.'); return; }
    setSaving(true);
    setError('');
    try {
      await onSave({ ...draft, name: draft.name.trim() });
      setDraft(initialDraft);
    } catch {
      setError('The alert could not be saved. Try again.');
    } finally {
      setSaving(false);
    }
  };
  const fieldClass = 'h-10 w-full rounded-lg border border-terminal-line bg-terminal-ink/80 px-3 text-sm';
  return (
    <section className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-4">
      <header className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h2 className="font-display text-lg font-semibold">Create alert</h2>
          <p className="mt-0.5 text-xs text-slate-500">Matches live gross spread. Fees are not inferred yet.</p>
        </div>
        <span className="rounded-md border border-signal-mint/20 bg-signal-mint/[0.07] px-2 py-1 font-data text-[10px] uppercase text-signal-mint">Browser / in-app</span>
      </header>
      <form className="grid gap-3 sm:grid-cols-2" onSubmit={submit}>
        <label className="sm:col-span-2"><span className="mb-1 block text-xs text-slate-400">Rule name</span><input aria-label="Rule name" className={fieldClass} onChange={(event) => update('name', event.target.value)} placeholder="e.g. COTI spot gap" value={draft.name} /></label>
        <label><span className="mb-1 block text-xs text-slate-400">Pair</span><select aria-label="Alert pair" className={fieldClass} onChange={(event) => update('symbol', event.target.value as AlertRuleInput['symbol'])} value={draft.symbol}><option value="">All pairs</option>{SYMBOLS.map((symbol) => <option key={symbol} value={symbol}>{symbol.replace('USDT', '/USDT')}</option>)}</select></label>
        <label><span className="mb-1 block text-xs text-slate-400">Market type</span><select aria-label="Alert market type" className={fieldClass} onChange={(event) => update('marketMode', event.target.value as AlertMarketMode)} value={draft.marketMode}>{Object.entries(marketLabels).map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select></label>
        <label><span className="mb-1 block text-xs text-slate-400">Buy venue</span><select aria-label="Alert buy venue" className={fieldClass} onChange={(event) => update('buySource', event.target.value)} value={draft.buySource}><option value="">Any exchange</option>{executableSources.map((source) => <option key={source.key} value={source.key}>{source.label}</option>)}</select></label>
        <label><span className="mb-1 block text-xs text-slate-400">Sell venue</span><select aria-label="Alert sell venue" className={fieldClass} onChange={(event) => update('sellSource', event.target.value)} value={draft.sellSource}><option value="">Any exchange</option>{executableSources.map((source) => <option key={source.key} value={source.key}>{source.label}</option>)}</select></label>
        <label><span className="mb-1 block text-xs text-slate-400">Minimum gross spread</span><div className="relative"><input aria-label="Minimum alert spread" className={`${fieldClass} pr-8 font-data`} min="0" onChange={(event) => update('minSpreadPct', Number(event.target.value))} step="0.05" type="number" value={draft.minSpreadPct} /><span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-slate-500">%</span></div></label>
        <label><span className="mb-1 block text-xs text-slate-400">Cooldown</span><select aria-label="Alert cooldown" className={fieldClass} onChange={(event) => update('cooldownSeconds', Number(event.target.value))} value={draft.cooldownSeconds}><option value={60}>1 minute</option><option value={300}>5 minutes</option><option value={900}>15 minutes</option><option value={3600}>1 hour</option></select></label>
        <label className="flex items-center gap-2 text-xs text-slate-300 sm:col-span-2"><input checked={draft.browserEnabled} className="accent-emerald-400" onChange={(event) => update('browserEnabled', event.target.checked)} type="checkbox" /> Browser notification when permission is granted</label>
        {error && <p className="text-xs text-red-400 sm:col-span-2" role="alert">{error}</p>}
        <button className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-signal-mint px-4 text-sm font-semibold text-terminal-ink hover:brightness-110 disabled:opacity-50 sm:col-span-2" disabled={saving} type="submit"><Plus size={16} />{saving ? 'Saving…' : 'Save alert'}</button>
      </form>
    </section>
  );
}

function RecentTriggers({ triggers, now }: { triggers: AlertTrigger[]; now: number }) {
  return (
    <section className="overflow-hidden rounded-xl border border-terminal-line bg-terminal-panel/65 xl:col-span-2">
      <header className="border-b border-terminal-line p-4"><h2 className="font-display text-lg font-semibold">Recent triggers</h2><p className="mt-0.5 text-xs text-slate-500">Persisted rule matches, newest first.</p></header>
      <div className="overflow-x-auto"><table className="w-full min-w-[820px] text-left text-sm"><thead className="text-[10px] uppercase tracking-wide text-slate-600"><tr className="border-b border-terminal-line"><th className="px-4 py-3">Time</th><th className="px-4 py-3">Rule / pair</th><th className="px-4 py-3">Buy</th><th className="px-4 py-3">Sell</th><th className="px-4 py-3 text-right">Gross spread</th><th className="px-4 py-3 text-right">Delivery</th></tr></thead>
        <tbody>{triggers.length ? triggers.map((trigger) => <tr className="border-b border-terminal-line/70 last:border-0" key={trigger.id}><td className="px-4 py-3 font-data text-xs text-slate-500">{relativeTime(trigger.triggeredAtMS, now)}</td><td className="px-4 py-3"><p className="font-medium">{trigger.ruleName}</p><p className="font-data text-[11px] text-slate-500">{trigger.symbol.replace('USDT', '/USDT')}</p></td><td className="px-4 py-3"><SourceMark source={trigger.buySource} /><p className="mt-1 font-data text-[11px] text-slate-500">${formatPrice(trigger.buyPrice)}</p></td><td className="px-4 py-3"><SourceMark source={trigger.sellSource} /><p className="mt-1 font-data text-[11px] text-slate-500">${formatPrice(trigger.sellPrice)}</p></td><td className="px-4 py-3 text-right font-data text-signal-mint">+{trigger.grossSpreadPct.toFixed(2)}%</td><td className="px-4 py-3 text-right text-xs text-signal-mint">In app</td></tr>) : <tr><td className="px-5 py-14 text-center text-slate-500" colSpan={6}>No alert triggers yet.</td></tr>}</tbody>
      </table></div>
    </section>
  );
}

export function AlertsPage({ state, fetcher = fetch, now = Date.now() }: AlertsPageProps) {
  const { rules, triggers, status, createRule, updateRule, retry } = useAlerts(state.alertTriggers, fetcher);
  const [mutationError, setMutationError] = useState('');
  const notified = useRef(new Set<number>());
  const activeCount = rules.filter((rule) => rule.enabled).length;
  const mutedCount = rules.length - activeCount;
  const todayStart = new Date(now); todayStart.setHours(0, 0, 0, 0);
  const todayCount = triggers.filter((trigger) => trigger.triggeredAtMS >= todayStart.getTime()).length;

  useEffect(() => {
    const trigger = state.alertTriggers[0];
    if (!trigger || notified.current.has(trigger.id) || typeof Notification === 'undefined') return;
    notified.current.add(trigger.id);
    const rule = rules.find((item) => item.id === trigger.ruleID);
    if (rule?.browserEnabled && Notification.permission === 'granted') {
      new Notification(`${trigger.symbol.replace('USDT', '/USDT')} +${trigger.grossSpreadPct.toFixed(2)}%`, {
        body: `${sourceMeta(trigger.buySource).label} → ${sourceMeta(trigger.sellSource).label}`,
      });
    }
  }, [rules, state.alertTriggers]);

  const stats = useMemo(() => [
    { label: 'Active rules', value: activeCount, icon: BellRing },
    { label: 'Triggered today', value: todayCount, icon: Check },
    { label: 'Muted', value: mutedCount, icon: VolumeX },
    { label: 'Cooldown engine', value: 'On', icon: Clock3 },
  ], [activeCount, mutedCount, todayCount]);

  const save = async (input: AlertRuleInput) => {
    if (input.browserEnabled && typeof Notification !== 'undefined' && Notification.permission === 'default') {
      await Notification.requestPermission();
    }
    await createRule(input);
  };
  const toggle = async (rule: AlertRule) => {
    setMutationError('');
    try { await updateRule(rule.id, { ...inputFromRule(rule), enabled: !rule.enabled }); }
    catch { setMutationError('Rule status could not be saved.'); }
  };

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-4"><div><p className="font-data text-[11px] uppercase tracking-[0.22em] text-signal-mint">Signal desk</p><h1 className="mt-1 font-display text-3xl font-semibold tracking-tight">Alerts</h1><p className="mt-1 text-sm text-slate-400">Create actionable spread rules and monitor recent triggers.</p></div><div className="flex items-center gap-2 rounded-full border border-signal-mint/20 bg-signal-mint/[0.07] px-3 py-1.5 font-data text-xs text-signal-mint"><ShieldCheck size={14} />{activeCount} active {activeCount === 1 ? 'rule' : 'rules'}</div></header>
      {status === 'degraded' && <div className="flex items-center justify-between rounded-lg border border-red-400/25 bg-red-400/5 px-4 py-3 text-sm text-red-300" role="alert"><span>Alert storage is temporarily unavailable.</span><button className="underline" onClick={retry} type="button">Retry</button></div>}
      {mutationError && <p className="rounded-lg border border-red-400/25 bg-red-400/5 px-4 py-3 text-sm text-red-300" role="alert">{mutationError}</p>}
      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{stats.map(({ label, value, icon: Icon }) => <article className="rounded-xl border border-terminal-line bg-terminal-panel/65 p-4" key={label}><div className="flex items-center justify-between"><p className="text-xs text-slate-500">{label}</p><Icon className="text-slate-600" size={17} /></div><p className="mt-3 font-data text-2xl text-terminal-text">{value}</p></article>)}</section>
      <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(440px,0.95fr)]"><RuleList loading={status === 'loading'} now={now} onToggle={toggle} rules={rules} /><AlertEditor onSave={save} /><RecentTriggers now={now} triggers={triggers} /></div>
    </div>
  );
}
