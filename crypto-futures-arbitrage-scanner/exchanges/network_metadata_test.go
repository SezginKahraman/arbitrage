package exchanges

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestParseKuCoinAssetNetworksPreservesDirectionalAvailabilityAndCosts(t *testing.T) {
	payload := []byte(`{
		"code":"200000",
		"data":{"currency":"COTI","chains":[
			{"chainName":"ERC20","chainId":"eth","withdrawalMinSize":"300","withdrawalMinFee":"150","isWithdrawEnabled":false,"isDepositEnabled":true,"confirms":64,"contractAddress":"0xddb3422497e61e13543bea06989c0789117555c5"},
			{"chainName":"COTI","chainId":"cotievm","withdrawalMinSize":"300","withdrawalMinFee":"150","isWithdrawEnabled":true,"isDepositEnabled":true,"confirms":100,"contractAddress":""}
		]}}
	`)

	networks, err := parseKuCoinAssetNetworks(payload, "COTI", time.UnixMilli(20_000))
	if err != nil {
		t.Fatalf("parseKuCoinAssetNetworks() error = %v", err)
	}
	if len(networks) != 2 {
		t.Fatalf("networks = %+v", networks)
	}
	erc20 := networks[0]
	if erc20.Asset != "COTI" || erc20.NetworkID != "ethereum" || erc20.RawNetworkID != "eth" ||
		!erc20.DepositEnabled || erc20.WithdrawEnabled || erc20.WithdrawalFee != "150" ||
		erc20.MinimumWithdrawal != "300" || erc20.Confirmations != 64 ||
		erc20.ContractAddress != "0xddb3422497e61e13543bea06989c0789117555c5" || erc20.CheckedAt != 20_000 {
		t.Fatalf("ERC20 network = %+v", erc20)
	}
	coti := networks[1]
	if coti.NetworkID != "coti_evm" || coti.RawNetworkID != "cotievm" || !coti.DepositEnabled || !coti.WithdrawEnabled {
		t.Fatalf("COTI network = %+v", coti)
	}
}

func TestParseGateAssetNetworksNormalizesStatusWithoutInventingFees(t *testing.T) {
	payload := []byte(`[
		{"chain":"COTI","name_en":"COTI","is_disabled":0,"is_deposit_disabled":0,"is_withdraw_disabled":0,"contract_address":"","withdraw_amount_min":"0.9827044"},
		{"chain":"ETH","name_en":"Ethereum(ERC20)","is_disabled":0,"is_deposit_disabled":0,"is_withdraw_disabled":1,"contract_address":"0xDDB3422497E61e13543BeA06989C0789117555c5","withdraw_amount_min":"0.9827044"}
	]`)

	networks, err := parseGateAssetNetworks(payload, "COTI", time.UnixMilli(30_000))
	if err != nil {
		t.Fatalf("parseGateAssetNetworks() error = %v", err)
	}
	if len(networks) != 2 {
		t.Fatalf("networks = %+v", networks)
	}
	if networks[0].NetworkID != "coti_evm" || !networks[0].DepositEnabled || !networks[0].WithdrawEnabled ||
		networks[0].MinimumWithdrawal != "0.9827044" || networks[0].WithdrawalFee != "" {
		t.Fatalf("COTI network = %+v", networks[0])
	}
	if networks[1].NetworkID != "ethereum" || !networks[1].DepositEnabled || networks[1].WithdrawEnabled ||
		networks[1].ContractAddress != "0xddb3422497e61e13543bea06989c0789117555c5" {
		t.Fatalf("Ethereum network = %+v", networks[1])
	}
}

func TestParseBinanceAssetNetworksFiltersTrackedAssetsAndNormalizesNetworks(t *testing.T) {
	payload := []byte(`[
		{"coin":"COTI","networkList":[
			{"network":"ETH","name":"Ethereum (ERC20)","depositEnable":true,"withdrawEnable":false,"withdrawFee":"150","withdrawMin":"300","minConfirm":64,"contractAddress":"0xDDB3422497E61e13543BeA06989C0789117555c5"},
			{"network":"COTI","name":"COTI","depositEnable":true,"withdrawEnable":true,"withdrawFee":"10","withdrawMin":"20","minConfirm":100,"contractAddress":""}
		]},
		{"coin":"OTHER","networkList":[{"network":"ETH","depositEnable":true,"withdrawEnable":true}]}
	]`)

	assets, err := parseBinanceAssetNetworks(payload, map[string]struct{}{"COTI": {}}, time.UnixMilli(40_000))
	if err != nil {
		t.Fatalf("parseBinanceAssetNetworks() error = %v", err)
	}
	networks := assets["COTI"]
	if len(assets) != 1 || len(networks) != 2 {
		t.Fatalf("assets = %+v", assets)
	}
	if networks[0].NetworkID != "ethereum" || networks[0].WithdrawEnabled || networks[0].WithdrawalFee != "150" {
		t.Fatalf("Ethereum network = %+v", networks[0])
	}
	if networks[1].NetworkID != "coti_evm" || !networks[1].WithdrawEnabled {
		t.Fatalf("COTI network = %+v", networks[1])
	}
}

func TestNetworkMetadataParsersRejectMalformedExchangePayloads(t *testing.T) {
	if _, err := parseKuCoinAssetNetworks([]byte(`{"code":"400000"}`), "COTI", time.Now()); err == nil {
		t.Fatal("KuCoin error response was accepted")
	}
	if _, err := parseGateAssetNetworks([]byte(`{"label":"error"}`), "COTI", time.Now()); err == nil {
		t.Fatal("Gate error response was accepted")
	}
	if _, err := parseBinanceAssetNetworks([]byte(`{"code":-2015}`), map[string]struct{}{"COTI": {}}, time.Now()); err == nil {
		t.Fatal("Binance error response was accepted")
	}
}

func TestFetchKuCoinAndGateAssetNetworksUsePublicCurrencyRoutes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v3/currencies/COTI":
			_, _ = writer.Write([]byte(`{"code":"200000","data":{"currency":"COTI","chains":[{"chainName":"COTI","chainId":"cotievm","isWithdrawEnabled":true,"isDepositEnabled":true}]}}`))
		case "/api/v4/wallet/currency_chains":
			if request.URL.Query().Get("currency") != "COTI" {
				t.Fatalf("Gate currency query = %q", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`[{"chain":"COTI","name_en":"COTI","is_disabled":0,"is_deposit_disabled":0,"is_withdraw_disabled":0}]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	checkedAt := time.UnixMilli(50_000)
	kuCoin, err := FetchKuCoinAssetNetworks(context.Background(), server.Client(), server.URL, "COTI", checkedAt)
	if err != nil || len(kuCoin) != 1 || kuCoin[0].NetworkID != "coti_evm" {
		t.Fatalf("KuCoin networks = %+v, error = %v", kuCoin, err)
	}
	gate, err := FetchGateAssetNetworks(context.Background(), server.Client(), server.URL, "COTI", checkedAt)
	if err != nil || len(gate) != 1 || gate[0].NetworkID != "coti_evm" {
		t.Fatalf("Gate networks = %+v, error = %v", gate, err)
	}
}

func TestFetchBinanceAssetNetworksUsesServerTimeAndSignsReadOnlyCapitalRoute(t *testing.T) {
	const apiKey = "test-api-key"
	const secret = "test-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v3/time":
			_, _ = writer.Write([]byte(`{"serverTime":123456789}`))
		case "/sapi/v1/capital/config/getall":
			if request.Header.Get("X-MBX-APIKEY") != apiKey {
				t.Fatal("Binance API key header missing")
			}
			query := request.URL.Query()
			signature := query.Get("signature")
			unsigned := url.Values{"recvWindow": {"5000"}, "timestamp": {"123456789"}}.Encode()
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write([]byte(unsigned))
			if signature != hex.EncodeToString(mac.Sum(nil)) {
				t.Fatalf("signature = %q", signature)
			}
			_, _ = writer.Write([]byte(`[{"coin":"COTI","networkList":[{"network":"COTI","name":"COTI","depositEnable":true,"withdrawEnable":true}]}]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	assets, err := FetchBinanceAssetNetworks(
		context.Background(), server.Client(), server.URL, apiKey, secret, []string{"COTI"}, time.UnixMilli(60_000),
	)
	if err != nil || len(assets["COTI"]) != 1 || !assets["COTI"][0].WithdrawEnabled {
		t.Fatalf("assets = %+v, error = %v", assets, err)
	}
}

func TestFetchBinanceAssetNetworksClassifiesCredentialRejectionWithoutLeakingMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/v3/time" {
			_, _ = writer.Write([]byte(`{"serverTime":123456789}`))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"code":-2015,"msg":"raw sensitive exchange detail"}`))
	}))
	defer server.Close()

	_, err := FetchBinanceAssetNetworks(
		context.Background(), server.Client(), server.URL, "key", "secret", []string{"COTI"}, time.Now(),
	)
	if NetworkMetadataErrorCode(err) != "credentials_rejected" || err.Error() != "credentials_rejected" {
		t.Fatalf("error = %q, code = %q", err, NetworkMetadataErrorCode(err))
	}
}
