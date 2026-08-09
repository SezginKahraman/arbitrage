import type { ReactNode } from 'react';

import { Sidebar } from './Sidebar';

interface AppShellProps {
  children: ReactNode;
  topBar: ReactNode;
  onOpenSettings: () => void;
}

export function AppShell({ children, topBar, onOpenSettings }: AppShellProps) {
  return (
    <div className="min-h-screen bg-terminal-ink text-terminal-text lg:grid lg:grid-cols-[92px_minmax(0,1fr)]">
      <Sidebar onOpenSettings={onOpenSettings} />
      <div className="min-w-0">
        {topBar}
        <main className="mx-auto max-w-[1800px] space-y-4 p-4 md:p-5">{children}</main>
      </div>
    </div>
  );
}
