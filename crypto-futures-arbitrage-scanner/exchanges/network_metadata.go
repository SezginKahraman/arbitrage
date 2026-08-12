package exchanges

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type AssetNetwork struct {
	Asset             string `json:"asset"`
	NetworkID         string `json:"network_id"`
	RawNetworkID      string `json:"raw_network_id"`
	Name              string `json:"name"`
	ContractAddress   string `json:"contract_address,omitempty"`
	DepositEnabled    bool   `json:"deposit_enabled"`
	WithdrawEnabled   bool   `json:"withdraw_enabled"`
	WithdrawalFee     string `json:"withdrawal_fee,omitempty"`
	MinimumWithdrawal string `json:"minimum_withdrawal,omitempty"`
	Confirmations     int    `json:"confirmations,omitempty"`
	CheckedAt         int64  `json:"checked_at"`
}

var nonNetworkIDCharacter = regexp.MustCompile(`[^a-z0-9]+`)

func canonicalNetworkID(rawID, name string) string {
	normalized := strings.ToLower(strings.TrimSpace(rawID))
	compact := nonNetworkIDCharacter.ReplaceAllString(normalized, "")
	switch compact {
	case "eth", "ethereum", "erc20":
		return "ethereum"
	case "coti", "cotievm":
		return "coti_evm"
	case "btc", "bitcoin":
		return "bitcoin"
	case "sol", "solana":
		return "solana"
	case "xrp", "ripple":
		return "ripple"
	case "bsc", "bep20", "bnbsmartchain":
		return "bsc"
	case "trx", "tron", "trc20":
		return "tron"
	}
	if compact == "" {
		compact = nonNetworkIDCharacter.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "")
	}
	return compact
}

func normalizeContractAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func parseKuCoinAssetNetworks(payload []byte, asset string, checkedAt time.Time) ([]AssetNetwork, error) {
	var response struct {
		Code string `json:"code"`
		Data struct {
			Currency string `json:"currency"`
			Chains   []struct {
				Name              string `json:"chainName"`
				ID                string `json:"chainId"`
				MinimumWithdrawal string `json:"withdrawalMinSize"`
				WithdrawalFee     string `json:"withdrawalMinFee"`
				WithdrawEnabled   bool   `json:"isWithdrawEnabled"`
				DepositEnabled    bool   `json:"isDepositEnabled"`
				Confirmations     int    `json:"confirms"`
				ContractAddress   string `json:"contractAddress"`
			} `json:"chains"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode KuCoin network metadata: %w", err)
	}
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if response.Code != "200000" || asset == "" || !strings.EqualFold(response.Data.Currency, asset) {
		return nil, fmt.Errorf("invalid KuCoin network metadata response")
	}
	networks := make([]AssetNetwork, 0, len(response.Data.Chains))
	for _, chain := range response.Data.Chains {
		networkID := canonicalNetworkID(chain.ID, chain.Name)
		if networkID == "" {
			continue
		}
		networks = append(networks, AssetNetwork{
			Asset: asset, NetworkID: networkID, RawNetworkID: chain.ID, Name: chain.Name,
			ContractAddress: normalizeContractAddress(chain.ContractAddress),
			DepositEnabled:  chain.DepositEnabled, WithdrawEnabled: chain.WithdrawEnabled,
			WithdrawalFee: chain.WithdrawalFee, MinimumWithdrawal: chain.MinimumWithdrawal,
			Confirmations: chain.Confirmations, CheckedAt: checkedAt.UnixMilli(),
		})
	}
	return networks, nil
}

func parseGateAssetNetworks(payload []byte, asset string, checkedAt time.Time) ([]AssetNetwork, error) {
	var response []struct {
		ID                string `json:"chain"`
		Name              string `json:"name_en"`
		Disabled          int    `json:"is_disabled"`
		DepositDisabled   int    `json:"is_deposit_disabled"`
		WithdrawDisabled  int    `json:"is_withdraw_disabled"`
		ContractAddress   string `json:"contract_address"`
		MinimumWithdrawal string `json:"withdraw_amount_min"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode Gate.io network metadata: %w", err)
	}
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if asset == "" {
		return nil, fmt.Errorf("invalid Gate.io network metadata asset")
	}
	networks := make([]AssetNetwork, 0, len(response))
	for _, chain := range response {
		networkID := canonicalNetworkID(chain.ID, chain.Name)
		if networkID == "" {
			continue
		}
		networks = append(networks, AssetNetwork{
			Asset: asset, NetworkID: networkID, RawNetworkID: chain.ID, Name: chain.Name,
			ContractAddress:   normalizeContractAddress(chain.ContractAddress),
			DepositEnabled:    chain.Disabled == 0 && chain.DepositDisabled == 0,
			WithdrawEnabled:   chain.Disabled == 0 && chain.WithdrawDisabled == 0,
			MinimumWithdrawal: chain.MinimumWithdrawal, CheckedAt: checkedAt.UnixMilli(),
		})
	}
	return networks, nil
}

func parseBinanceAssetNetworks(payload []byte, allowedAssets map[string]struct{}, checkedAt time.Time) (map[string][]AssetNetwork, error) {
	var response []struct {
		Asset    string `json:"coin"`
		Networks []struct {
			ID                string `json:"network"`
			Name              string `json:"name"`
			DepositEnabled    bool   `json:"depositEnable"`
			WithdrawEnabled   bool   `json:"withdrawEnable"`
			WithdrawalFee     string `json:"withdrawFee"`
			MinimumWithdrawal string `json:"withdrawMin"`
			Confirmations     int    `json:"minConfirm"`
			ContractAddress   string `json:"contractAddress"`
		} `json:"networkList"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode Binance network metadata: %w", err)
	}
	assets := make(map[string][]AssetNetwork)
	for _, currency := range response {
		asset := strings.ToUpper(strings.TrimSpace(currency.Asset))
		if _, ok := allowedAssets[asset]; !ok {
			continue
		}
		for _, chain := range currency.Networks {
			networkID := canonicalNetworkID(chain.ID, chain.Name)
			if networkID == "" {
				continue
			}
			assets[asset] = append(assets[asset], AssetNetwork{
				Asset: asset, NetworkID: networkID, RawNetworkID: chain.ID, Name: chain.Name,
				ContractAddress: normalizeContractAddress(chain.ContractAddress),
				DepositEnabled:  chain.DepositEnabled, WithdrawEnabled: chain.WithdrawEnabled,
				WithdrawalFee: chain.WithdrawalFee, MinimumWithdrawal: chain.MinimumWithdrawal,
				Confirmations: chain.Confirmations, CheckedAt: checkedAt.UnixMilli(),
			})
		}
	}
	return assets, nil
}
