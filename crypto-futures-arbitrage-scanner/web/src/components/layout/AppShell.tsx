import type { ReactNode } from 'react';

import type { AppPage } from '../../app/navigation';
import { Sidebar } from './Sidebar';

interface AppShellProps {
  children: ReactNode;
  topBar: ReactNode;
  onOpenSettings: () => void;
  activePage?: AppPage;
  onNavigate?: (page: AppPage) => void;
}

export function AppShell({ children, topBar, onOpenSettings, activePage, onNavigate }: AppShellProps) {
  return (
    <div className="min-h-screen bg-terminal-ink text-terminal-text lg:grid lg:grid-cols-[92px_minmax(0,1fr)]">
      <Sidebar activePage={activePage} onNavigate={onNavigate} onOpenSettings={onOpenSettings} />
      <div className="min-w-0">
        {topBar}
        <main className="mx-auto max-w-[1800px] space-y-4 p-4 pb-24 md:p-5 md:pb-24 lg:pb-5">{children}</main>
      </div>
    </div>
  );
}
