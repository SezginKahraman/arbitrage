import { useCallback, useEffect, useState } from 'react';

import { AppShell } from '../components/layout/AppShell';
import { TopBar } from '../components/layout/TopBar';
import { AlertsPage } from '../components/alerts/AlertsPage';
import { AllOpportunitiesPage } from '../components/opportunities/AllOpportunitiesPage';
import { ScannerDashboard } from '../components/scanner/ScannerDashboard';
import { SettingsDrawer } from '../components/settings/SettingsDrawer';
import { useOpportunityHistory } from '../hooks/useOpportunityHistory';
import { usePreferences } from '../hooks/usePreferences';
import { useScannerSocket } from '../hooks/useScannerSocket';
import { pageFromPath, pathForPage, type AppPage } from './navigation';

export function App() {
  const [preferences, setPreferences] = usePreferences();
  const scannerState = useScannerSocket();
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
          />
        }
      >
        {page === 'opportunities' ? <AllOpportunitiesPage state={scannerState} /> : <AlertsPage state={scannerState} />}
        <SettingsDrawer
          onClose={() => setSettingsOpen(false)}
          onPreferencesChange={setPreferences}
          open={settingsOpen}
          preferences={preferences}
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
    />
  );
}
