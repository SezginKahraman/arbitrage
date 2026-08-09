import type { ReactNode } from 'react';

interface StatusBadgeProps {
  children: ReactNode;
  tone?: 'positive' | 'warning' | 'neutral';
}

const toneClasses = {
  positive: 'border-signal-mint/25 bg-signal-mint/10 text-signal-mint',
  warning: 'border-signal-amber/25 bg-signal-amber/10 text-signal-amber',
  neutral: 'border-terminal-line bg-white/[0.035] text-slate-300',
};

export function StatusBadge({ children, tone = 'neutral' }: StatusBadgeProps) {
  return (
    <span className={`inline-flex items-center rounded-md border px-2 py-1 text-xs font-medium ${toneClasses[tone]}`}>
      {children}
    </span>
  );
}
