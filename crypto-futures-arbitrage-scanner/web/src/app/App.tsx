import { useCallback, useEffect, useState } from 'react';

import { AppShell } from '../components/layout/AppShell';
import { TopBar } from '../components/layout/TopBar';
import { AlertsPage } from '../components/alerts/AlertsPage';
import { AllOpportunitiesPage } from '../components/opportunities/AllOpportunitiesPage';
import { ScannerDashboard } from '../components/scanner/ScannerDashboard';
import { SettingsDrawer } from '../components/settings/SettingsDrawer';
import { useMarketCatalog } from '../hooks/useMarketCatalog';
import { useOpportunityHistory } from '../hooks/useOpportunityHistory';
import { usePreferences } from '../hooks/usePreferences';
import { useScannerSocket } from '../hooks/useScannerSocket';
import { useTransferRoutes } from '../hooks/useTransferRoutes';
import { pageFromPath, pathForPage, type AppPage } from './navigation';
import { SYMBOLS } from './types';

export function App() {
  const [preferences, setPreferences] = usePreferences();
  const scannerState = useScannerSocket();
  const marketCatalog = useMarketCatalog();
  const transferRoutes = useTransferRoutes(fetch, 60_000, marketCatalog.watchlist.join(','));
  const activeSymbols = marketCatalog.watchlist.length ? marketCatalog.watchlist : [...SYMBOLS];
  const [page, setPage] = useState<AppPage>(() => pageFromPath(window.location.pathname));
  const [settingsOpen, setSettingsOpen] = useState(false);
  const opportunityHistory = useOpportunityHistory({
    symbol: preferences.symbol,
    minSpread: preferences.minSpread,
  });

  useEffect(() => {
    const handlePopState = () => setPage(pageFromPath(window.location.pathname));
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  useEffect(() => {
    if (
      marketCatalog.status !== 'ready' ||
      !marketCatalog.watchlist.length ||
      marketCatalog.watchlist.includes(preferences.symbol)
    ) return;
    setPreferences((current) => ({ ...current, symbol: marketCatalog.watchlist[0] }));
  }, [marketCatalog.status, marketCatalog.watchlist, preferences.symbol, setPreferences]);

  const navigate = useCallback((nextPage: AppPage) => {
    window.history.pushState({}, '', pathForPage(nextPage));
    setPage(nextPage);
  }, []);

  if (page === 'opportunities' || page === 'alerts') {
    return (
      <AppShell
        activePage={page}
        onNavigate={navigate}
        onOpenSettings={() => setSettingsOpen(true)}
        topBar={
          <TopBar
            connection={scannerState.connection}
            lastUpdatedAt={scannerState.lastUpdatedAt}
            onOpenSettings={() => setSettingsOpen(true)}
            onPreferencesChange={setPreferences}
            preferences={preferences}
            showPairSelector={false}
            symbols={activeSymbols}
          />
        }
      >
        {page === 'opportunities'
          ? <AllOpportunitiesPage marketCatalog={marketCatalog} state={scannerState} transferRoutes={transferRoutes} />
          : <AlertsPage state={scannerState} symbols={activeSymbols} />}
        <SettingsDrawer
          onClose={() => setSettingsOpen(false)}
          onPreferencesChange={setPreferences}
          open={settingsOpen}
          preferences={preferences}
          symbols={activeSymbols}
        />
      </AppShell>
    );
  }

  return (
    <ScannerDashboard
      history={opportunityHistory}
      onPreferencesChange={setPreferences}
      onNavigate={navigate}
      preferences={preferences}
      state={scannerState}
      symbols={activeSymbols}
    />
  );
}
