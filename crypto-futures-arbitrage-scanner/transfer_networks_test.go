package main

import (
	"testing"

	"futures-arbitrage-scanner/exchanges"
)

func TestEvaluateTransferRouteReportsOpenNativeAliasAsCheckAndClosedERC20AsBlocked(t *testing.T) {
	snapshots := map[string]networkVenueSnapshot{
		"gate_spot": {
			Source: "gate_spot", Asset: "COTI", Status: networkVenueReady, CheckedAt: 20_000,
			Networks: []exchanges.AssetNetwork{
				{Asset: "COTI", NetworkID: "coti_evm", RawNetworkID: "COTI", Name: "COTI", DepositEnabled: true, WithdrawEnabled: true},
				{Asset: "COTI", NetworkID: "ethereum", RawNetworkID: "ETH", Name: "Ethereum(ERC20)", ContractAddress: "0xddb3422497e61e13543bea06989c0789117555c5", DepositEnabled: true, WithdrawEnabled: false},
			},
		},
		"kucoin_spot": {
			Source: "kucoin_spot", Asset: "COTI", Status: networkVenueReady, CheckedAt: 21_000,
			Networks: []exchanges.AssetNetwork{
				{Asset: "COTI", NetworkID: "coti_evm", RawNetworkID: "cotievm", Name: "COTI", DepositEnabled: true, WithdrawEnabled: true, WithdrawalFee: "150", MinimumWithdrawal: "300"},
				{Asset: "COTI", NetworkID: "ethereum", RawNetworkID: "eth", Name: "ERC20", ContractAddress: "0xddb3422497e61e13543bea06989c0789117555c5", DepositEnabled: true, WithdrawEnabled: false, WithdrawalFee: "150", MinimumWithdrawal: "300"},
			},
		},
	}

	route := evaluateTransferRoute("COTI", "gate_spot", "kucoin_spot", snapshots)
	if route.Status != transferRouteCheck || route.CheckedAt != 20_000 || len(route.Networks) != 2 {
		t.Fatalf("route = %+v", route)
	}
	coti := route.Networks[0]
	if coti.NetworkID != "coti_evm" || coti.Status != transferRouteCheck || !coti.SourceWithdrawEnabled || !coti.DestinationDepositEnabled {
		t.Fatalf("COTI route = %+v", coti)
	}
	ethereum := route.Networks[1]
	if ethereum.NetworkID != "ethereum" || ethereum.Status != transferRouteBlocked || ethereum.Reason != "source withdrawal disabled" {
		t.Fatalf("Ethereum route = %+v", ethereum)
	}
}

func TestEvaluateTransferRouteMarksExactOpenContractAsReady(t *testing.T) {
	contract := "0xddb3422497e61e13543bea06989c0789117555c5"
	snapshots := map[string]networkVenueSnapshot{
		"gate_spot": {Source: "gate_spot", Asset: "COTI", Status: networkVenueReady, Networks: []exchanges.AssetNetwork{{
			Asset: "COTI", NetworkID: "ethereum", RawNetworkID: "ETH", ContractAddress: contract, WithdrawEnabled: true,
		}}},
		"kucoin_spot": {Source: "kucoin_spot", Asset: "COTI", Status: networkVenueReady, Networks: []exchanges.AssetNetwork{{
			Asset: "COTI", NetworkID: "ethereum", RawNetworkID: "eth", ContractAddress: contract, DepositEnabled: true,
		}}},
	}

	route := evaluateTransferRoute("COTI", "gate_spot", "kucoin_spot", snapshots)
	if route.Status != transferRouteReady || len(route.Networks) != 1 || route.Networks[0].Status != transferRouteReady {
		t.Fatalf("route = %+v", route)
	}
}

func TestEvaluateTransferRoutePreservesUnavailableAndNonSpotStates(t *testing.T) {
	unknown := evaluateTransferRoute("COTI", "binance_spot", "gate_spot", map[string]networkVenueSnapshot{
		"binance_spot": {Source: "binance_spot", Asset: "COTI", Status: networkVenueUnavailable, ErrorCode: "credentials_rejected"},
		"gate_spot": {Source: "gate_spot", Asset: "COTI", Status: networkVenueReady, Networks: []exchanges.AssetNetwork{{
			Asset: "COTI", NetworkID: "coti_evm", RawNetworkID: "COTI", DepositEnabled: true, WithdrawEnabled: true,
		}}},
	})
	if unknown.Status != transferRouteUnknown || unknown.Reason != "binance_spot credentials rejected" {
		t.Fatalf("unknown route = %+v", unknown)
	}
	if len(unknown.SourceNetworks) != 0 || len(unknown.DestinationNetworks) != 1 {
		t.Fatalf("available-side networks were lost: %+v", unknown)
	}

	notApplicable := evaluateTransferRoute("COTI", "gate_futures", "kucoin_spot", nil)
	if notApplicable.Status != transferRouteNotApplicable {
		t.Fatalf("non-spot route = %+v", notApplicable)
	}
}
