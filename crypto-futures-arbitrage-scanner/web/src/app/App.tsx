import { ScannerDashboard } from '../components/scanner/ScannerDashboard';
import { useOpportunityHistory } from '../hooks/useOpportunityHistory';
import { usePreferences } from '../hooks/usePreferences';
import { useScannerSocket } from '../hooks/useScannerSocket';

export function App() {
  const [preferences, setPreferences] = usePreferences();
  const scannerState = useScannerSocket();
  const opportunityHistory = useOpportunityHistory({
    symbol: preferences.symbol,
    minSpread: preferences.minSpread,
  });

  return (
    <ScannerDashboard
      history={opportunityHistory}
      onPreferencesChange={setPreferences}
      preferences={preferences}
      state={scannerState}
    />
  );
}
