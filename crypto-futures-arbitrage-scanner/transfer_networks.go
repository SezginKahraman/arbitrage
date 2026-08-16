package main

import (
	"strings"

	"futures-arbitrage-scanner/exchanges"
)

const (
	networkVenueLoading     = "loading"
	networkVenueReady       = "ready"
	networkVenueUnavailable = "unavailable"

	transferRouteReady         = "ready"
	transferRouteBlocked       = "blocked"
	transferRouteCheck         = "check"
	transferRouteUnknown       = "unknown"
	transferRouteNotApplicable = "not_applicable"
)

type networkVenueSnapshot struct {
	Source    string                   `json:"source"`
	Asset     string                   `json:"asset"`
	Status    string                   `json:"status"`
	ErrorCode string                   `json:"error_code,omitempty"`
	CheckedAt int64                    `json:"checked_at"`
	Networks  []exchanges.AssetNetwork `json:"networks"`
}

type transferNetworkMatch struct {
	NetworkID                 string `json:"network_id"`
	Name                      string `json:"name"`
	Status                    string `json:"status"`
	Reason                    string `json:"reason"`
	SourceWithdrawEnabled     bool   `json:"source_withdraw_enabled"`
	DestinationDepositEnabled bool   `json:"destination_deposit_enabled"`
	WithdrawalFee             string `json:"withdrawal_fee,omitempty"`
	MinimumWithdrawal         string `json:"minimum_withdrawal,omitempty"`
	ContractAddress           string `json:"contract_address,omitempty"`
}

type transferRouteEvaluation struct {
	Asset               string                   `json:"asset"`
	Source              string                   `json:"source"`
	Destination         string                   `json:"destination"`
	Status              string                   `json:"status"`
	Reason              string                   `json:"reason"`
	CheckedAt           int64                    `json:"checked_at"`
	Networks            []transferNetworkMatch   `json:"networks"`
	SourceNetworks      []exchanges.AssetNetwork `json:"source_networks"`
	DestinationNetworks []exchanges.AssetNetwork `json:"destination_networks"`
}

func evaluateTransferRoute(asset, source, destination string, snapshots map[string]networkVenueSnapshot) transferRouteEvaluation {
	result := transferRouteEvaluation{
		Asset: asset, Source: source, Destination: destination, Status: transferRouteUnknown,
		Networks: []transferNetworkMatch{}, SourceNetworks: []exchanges.AssetNetwork{}, DestinationNetworks: []exchanges.AssetNetwork{},
	}
	if !strings.HasSuffix(source, "_spot") || !strings.HasSuffix(destination, "_spot") {
		result.Status = transferRouteNotApplicable
		result.Reason = "network checks apply to spot-to-spot routes"
		return result
	}

	sourceSnapshot, sourceExists := snapshots[source]
	destinationSnapshot, destinationExists := snapshots[destination]
	if sourceExists && sourceSnapshot.Status == networkVenueReady {
		result.SourceNetworks = append([]exchanges.AssetNetwork{}, sourceSnapshot.Networks...)
	}
	if destinationExists && destinationSnapshot.Status == networkVenueReady {
		result.DestinationNetworks = append([]exchanges.AssetNetwork{}, destinationSnapshot.Networks...)
	}
	result.CheckedAt = oldestPositiveTimestamp(sourceSnapshot.CheckedAt, destinationSnapshot.CheckedAt)
	if !sourceExists || sourceSnapshot.Status != networkVenueReady {
		result.Reason = unavailableVenueReason(source, sourceSnapshot, sourceExists)
		return result
	}
	if !destinationExists || destinationSnapshot.Status != networkVenueReady {
		result.Reason = unavailableVenueReason(destination, destinationSnapshot, destinationExists)
		return result
	}
	for _, sourceNetwork := range sourceSnapshot.Networks {
		for _, destinationNetwork := range destinationSnapshot.Networks {
			if sourceNetwork.NetworkID == "" || sourceNetwork.NetworkID != destinationNetwork.NetworkID ||
				!contractsCompatible(sourceNetwork.ContractAddress, destinationNetwork.ContractAddress) {
				continue
			}
			match := transferNetworkMatch{
				NetworkID: sourceNetwork.NetworkID, Name: preferredNetworkName(sourceNetwork, destinationNetwork),
				SourceWithdrawEnabled:     sourceNetwork.WithdrawEnabled,
				DestinationDepositEnabled: destinationNetwork.DepositEnabled,
				WithdrawalFee:             sourceNetwork.WithdrawalFee,
				MinimumWithdrawal:         sourceNetwork.MinimumWithdrawal,
				ContractAddress:           sharedContract(sourceNetwork.ContractAddress, destinationNetwork.ContractAddress),
			}
			switch {
			case !sourceNetwork.WithdrawEnabled:
				match.Status = transferRouteBlocked
				match.Reason = "source withdrawal disabled"
			case !destinationNetwork.DepositEnabled:
				match.Status = transferRouteBlocked
				match.Reason = "destination deposit disabled"
			case networkIdentityVerified(sourceNetwork, destinationNetwork):
				match.Status = transferRouteReady
				match.Reason = "network identity and direction are available"
			default:
				match.Status = transferRouteCheck
				match.Reason = "network alias requires verification"
			}
			result.Networks = append(result.Networks, match)
		}
	}

	result.Status = transferRouteBlocked
	result.Reason = "no common network is available in this direction"
	for _, match := range result.Networks {
		if match.Status == transferRouteReady {
			result.Status = transferRouteReady
			result.Reason = "verified common network available"
			return result
		}
	}
	for _, match := range result.Networks {
		if match.Status == transferRouteCheck {
			result.Status = transferRouteCheck
			result.Reason = "common network requires verification"
			return result
		}
	}
	return result
}

func unavailableVenueReason(source string, snapshot networkVenueSnapshot, exists bool) string {
	if exists && snapshot.ErrorCode != "" {
		return source + " " + strings.ReplaceAll(snapshot.ErrorCode, "_", " ")
	}
	return source + " network metadata unavailable"
}

func contractsCompatible(source, destination string) bool {
	return source == "" || destination == "" || strings.EqualFold(source, destination)
}

func networkIdentityVerified(source, destination exchanges.AssetNetwork) bool {
	if source.ContractAddress != "" && destination.ContractAddress != "" {
		return strings.EqualFold(source.ContractAddress, destination.ContractAddress)
	}
	return source.RawNetworkID != "" && strings.EqualFold(source.RawNetworkID, destination.RawNetworkID)
}

func sharedContract(source, destination string) string {
	if source != "" && destination != "" && strings.EqualFold(source, destination) {
		return strings.ToLower(source)
	}
	return ""
}

func preferredNetworkName(source, destination exchanges.AssetNetwork) string {
	if source.Name != "" {
		return source.Name
	}
	return destination.Name
}

func oldestPositiveTimestamp(left, right int64) int64 {
	if left <= 0 {
		return right
	}
	if right <= 0 || left < right {
		return left
	}
	return right
}
