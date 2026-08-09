export type AppPage = 'scanner' | 'opportunities' | 'alerts';

const pathByPage: Record<AppPage, string> = {
  scanner: '/',
  opportunities: '/opportunities',
  alerts: '/alerts',
};

export function pageFromPath(pathname: string): AppPage {
  if (pathname.startsWith('/opportunities')) return 'opportunities';
  if (pathname.startsWith('/alerts')) return 'alerts';
  return 'scanner';
}

export function pathForPage(page: AppPage): string {
  return pathByPage[page];
}
