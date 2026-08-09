import { sourceMeta } from '../../lib/sources';

interface SourceMarkProps {
  source: string;
  compact?: boolean;
}

export function SourceMark({ source, compact = false }: SourceMarkProps) {
  const meta = sourceMeta(source);

  return (
    <span className="inline-flex items-center gap-2">
      <span aria-hidden="true" className="size-2 rounded-full" style={{ backgroundColor: meta.color }} />
      <span>{compact ? meta.shortLabel : meta.label}</span>
    </span>
  );
}
