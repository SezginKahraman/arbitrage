import type { TransferRouteEvaluation, TransferRouteRequestStatus } from '../app/types';
import { sourceMeta } from './sources';

export type TransferRouteTone = 'positive' | 'warning' | 'negative' | 'neutral';

export interface TransferRoutePresentation {
  label: string;
  detail: string;
  tone: TransferRouteTone;
}

export function humanizeTransferReason(reason: string, source?: string, destination?: string): string {
  let value = reason.trim();
  for (const venue of [source, destination]) {
    if (venue) value = value.replaceAll(venue, sourceMeta(venue).label);
  }
  value = value.replaceAll('_', ' ');
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : 'Network metadata unavailable';
}

export function transferRoutePresentation(
  route: TransferRouteEvaluation | null,
  requestStatus: TransferRouteRequestStatus,
): TransferRoutePresentation {
  if (requestStatus === 'loading') {
    return { label: 'Checking transfer route', detail: 'Loading live deposit and withdrawal networks…', tone: 'neutral' };
  }
  if (requestStatus === 'degraded') {
    return { label: 'Transfer route unknown', detail: 'Live network metadata is temporarily unavailable.', tone: 'warning' };
  }
  if (!route) {
    return { label: 'Waiting for a route', detail: 'A buy and sell venue are required for a network check.', tone: 'neutral' };
  }

  const detail = humanizeTransferReason(route.reason, route.source, route.destination);
  switch (route.status) {
    case 'ready':
      return { label: 'Transfer route ready', detail, tone: 'positive' };
    case 'blocked':
      return { label: 'Transfer route blocked', detail, tone: 'negative' };
    case 'check':
      return { label: 'Transfer route check required', detail, tone: 'warning' };
    case 'unknown':
      return { label: 'Transfer route unknown', detail, tone: 'warning' };
    case 'not_applicable':
      return { label: 'Network check not applicable', detail, tone: 'neutral' };
  }
}
