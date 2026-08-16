package main

import (
	"context"
	"testing"
	"time"

	"futures-arbitrage-scanner/exchanges"
)

func TestNetworkCatalogRefreshKeepsSuccessfulVenuesWhenAnotherVenueFails(t *testing.T) {
	checkedAt := time.UnixMilli(70_000)
	catalog := newNetworkCatalog(
		[]string{"COTI"},
		[]networkSourceDefinition{
			{Source: "binance_spot", Fetch: func(context.Context, []string, time.Time) map[string]networkVenueSnapshot {
				return map[string]networkVenueSnapshot{"COTI": {
					Source: "binance_spot", Asset: "COTI", Status: networkVenueUnavailable,
					ErrorCode: "credentials_rejected", CheckedAt: checkedAt.UnixMilli(),
				}}
			}},
			{Source: "gate_spot", Fetch: func(context.Context, []string, time.Time) map[string]networkVenueSnapshot {
				return map[string]networkVenueSnapshot{"COTI": {
					Source: "gate_spot", Asset: "COTI", Status: networkVenueReady, CheckedAt: checkedAt.UnixMilli(),
					Networks: []exchanges.AssetNetwork{{Asset: "COTI", NetworkID: "coti_evm"}},
				}}
			}},
		},
	)

	catalog.RefreshAt(context.Background(), checkedAt)
	snapshots := catalog.Snapshots("COTI")
	if len(snapshots) != 2 || snapshots["binance_spot"].ErrorCode != "credentials_rejected" ||
		snapshots["gate_spot"].Status != networkVenueReady {
		t.Fatalf("snapshots = %+v", snapshots)
	}
}

func TestNetworkCatalogSnapshotsAreDefensiveCopies(t *testing.T) {
	catalog := newNetworkCatalog([]string{"COTI"}, nil)
	catalog.snapshots["COTI"]["gate_spot"] = networkVenueSnapshot{
		Source: "gate_spot", Asset: "COTI", Status: networkVenueReady,
		Networks: []exchanges.AssetNetwork{{Asset: "COTI", NetworkID: "coti_evm"}},
	}

	first := catalog.Snapshots("COTI")
	first["gate_spot"] = networkVenueSnapshot{Status: networkVenueUnavailable}
	second := catalog.Snapshots("COTI")
	if second["gate_spot"].Status != networkVenueReady || len(second["gate_spot"].Networks) != 1 {
		t.Fatalf("catalog was mutated through snapshot: %+v", second)
	}
}

func TestNetworkCatalogSetAssetsAddsLoadingSnapshotsAndDropsRemovedAssets(t *testing.T) {
	catalog := newNetworkCatalog([]string{"BTC", "COTI"}, []networkSourceDefinition{{Source: sourceGateSpot}})
	catalog.SetAssets([]string{"SOL", "COTI", "SOL"})
	if got := catalog.Assets(); len(got) != 2 || got[0] != "COTI" || got[1] != "SOL" {
		t.Fatalf("assets = %v", got)
	}
	if got := catalog.Snapshots("BTC"); len(got) != 0 {
		t.Fatalf("removed BTC snapshots = %+v", got)
	}
	if got := catalog.Snapshots("SOL")[sourceGateSpot].Status; got != networkVenueLoading {
		t.Fatalf("new SOL status = %q, want loading", got)
	}
}
