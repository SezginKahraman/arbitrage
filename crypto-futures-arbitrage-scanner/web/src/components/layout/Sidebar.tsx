import { Activity, Bell, ScanSearch, Settings, Target } from 'lucide-react';

import type { AppPage } from '../../app/navigation';

interface SidebarProps {
  onOpenSettings: () => void;
  activePage?: AppPage;
  onNavigate?: (page: AppPage) => void;
}

const navigation: Array<{ page: AppPage; label: string; aria: string; icon: typeof ScanSearch }> = [
  { page: 'scanner', label: 'Scanner', aria: 'Open scanner', icon: ScanSearch },
  { page: 'opportunities', label: 'Opportunities', aria: 'Open opportunities', icon: Target },
  { page: 'alerts', label: 'Alerts', aria: 'Open alerts', icon: Bell },
];

export function Sidebar({ onOpenSettings, activePage = 'scanner', onNavigate = () => undefined }: SidebarProps) {
  return (
    <aside className="fixed inset-x-0 bottom-0 z-40 flex border-t border-terminal-line bg-terminal-panel/95 backdrop-blur lg:sticky lg:top-0 lg:h-screen lg:self-start lg:flex-col lg:overflow-y-auto lg:border-r lg:border-t-0 lg:bg-terminal-panel/75">
      <div className="hidden h-20 place-items-center border-b border-terminal-line lg:grid">
        <span className="grid size-11 place-items-center rounded-xl border border-signal-mint/60 bg-signal-mint/10 text-signal-mint">
          <Activity aria-hidden="true" size={23} strokeWidth={1.8} />
        </span>
      </div>

      <nav aria-label="Primary" className="flex flex-1 items-stretch justify-around lg:min-h-0 lg:flex-col lg:items-center lg:justify-start lg:gap-3 lg:py-5">
        {navigation.map(({ page, label, aria, icon: Icon }) => {
          const active = activePage === page;
          return (
            <button
              aria-current={active ? 'page' : undefined}
              aria-label={aria}
              className={`flex min-w-0 flex-1 flex-col items-center gap-1 border-t-2 px-2 py-3 transition lg:w-full lg:flex-none lg:gap-2 lg:border-l-2 lg:border-t-0 lg:py-4 ${active ? 'border-signal-mint bg-signal-mint/[0.06] text-signal-mint' : 'border-transparent text-slate-400 hover:bg-white/[0.035] hover:text-terminal-text'}`}
              key={page}
              onClick={() => onNavigate(page)}
              type="button"
            >
              <Icon aria-hidden="true" size={22} />
              <span className="truncate text-[10px] font-medium lg:text-[11px]">{label}</span>
            </button>
          );
        })}
        <button
          aria-label="Open settings"
          className="flex min-w-0 flex-1 flex-col items-center gap-1 border-t-2 border-transparent px-2 py-3 text-slate-400 transition hover:bg-white/[0.035] hover:text-terminal-text lg:mt-auto lg:w-full lg:flex-none lg:gap-2 lg:border-0 lg:py-4"
          onClick={onOpenSettings}
          type="button"
        >
          <Settings aria-hidden="true" size={22} />
          <span className="truncate text-[10px] lg:text-[11px]">Settings</span>
        </button>
      </nav>
    </aside>
  );
}
