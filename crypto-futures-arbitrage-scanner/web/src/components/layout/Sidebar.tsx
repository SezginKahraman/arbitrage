import { Activity, ScanSearch, Settings } from 'lucide-react';

interface SidebarProps {
  onOpenSettings: () => void;
}

export function Sidebar({ onOpenSettings }: SidebarProps) {
  return (
    <aside className="hidden min-h-screen flex-col border-r border-terminal-line bg-terminal-panel/75 lg:flex">
      <div className="grid h-20 place-items-center border-b border-terminal-line">
        <span className="grid size-11 place-items-center rounded-xl border border-signal-mint/60 bg-signal-mint/10 text-signal-mint">
          <Activity aria-hidden="true" size={23} strokeWidth={1.8} />
        </span>
      </div>

      <nav aria-label="Primary" className="flex flex-1 flex-col items-center gap-3 py-5">
        <div className="flex w-full flex-col items-center gap-2 border-l-2 border-signal-mint bg-signal-mint/[0.06] py-4 text-signal-mint">
          <ScanSearch aria-hidden="true" size={22} />
          <span className="text-[11px] font-medium">Scanner</span>
        </div>
        <button
          aria-label="Open settings"
          className="mt-auto flex w-full flex-col items-center gap-2 py-4 text-slate-400 transition hover:bg-white/[0.035] hover:text-terminal-text"
          onClick={onOpenSettings}
          type="button"
        >
          <Settings aria-hidden="true" size={22} />
          <span className="text-[11px]">Settings</span>
        </button>
      </nav>
    </aside>
  );
}
