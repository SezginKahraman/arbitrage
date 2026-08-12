package exchanges

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const networkMetadataResponseLimit = 8 << 20

type NetworkMetadataError struct {
	Code string
}

func (e *NetworkMetadataError) Error() string {
	return e.Code
}

func NetworkMetadataErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var metadataError *NetworkMetadataError
	if errors.As(err, &metadataError) && metadataError.Code != "" {
		return metadataError.Code
	}
	return "unavailable"
}

func FetchKuCoinAssetNetworks(ctx context.Context, client *http.Client, baseURL, asset string, checkedAt time.Time) ([]AssetNetwork, error) {
	endpoint, err := networkMetadataURL(baseURL, "/api/v3/currencies/"+url.PathEscape(strings.ToUpper(strings.TrimSpace(asset))), nil)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_configuration"}
	}
	payload, err := fetchNetworkMetadata(ctx, client, endpoint, nil)
	if err != nil {
		return nil, err
	}
	networks, err := parseKuCoinAssetNetworks(payload, asset, checkedAt)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_response"}
	}
	return networks, nil
}

func FetchGateAssetNetworks(ctx context.Context, client *http.Client, baseURL, asset string, checkedAt time.Time) ([]AssetNetwork, error) {
	query := url.Values{"currency": {strings.ToUpper(strings.TrimSpace(asset))}}
	endpoint, err := networkMetadataURL(baseURL, "/api/v4/wallet/currency_chains", query)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_configuration"}
	}
	payload, err := fetchNetworkMetadata(ctx, client, endpoint, nil)
	if err != nil {
		return nil, err
	}
	networks, err := parseGateAssetNetworks(payload, asset, checkedAt)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_response"}
	}
	return networks, nil
}

func FetchBinanceAssetNetworks(
	ctx context.Context,
	client *http.Client,
	baseURL, apiKey, secret string,
	assets []string,
	checkedAt time.Time,
) (map[string][]AssetNetwork, error) {
	if apiKey == "" || secret == "" {
		return nil, &NetworkMetadataError{Code: "credentials_missing"}
	}
	timeEndpoint, err := networkMetadataURL(baseURL, "/api/v3/time", nil)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_configuration"}
	}
	timePayload, err := fetchNetworkMetadata(ctx, client, timeEndpoint, nil)
	if err != nil {
		return nil, err
	}
	var serverClock struct {
		ServerTime int64 `json:"serverTime"`
	}
	if json.Unmarshal(timePayload, &serverClock) != nil || serverClock.ServerTime <= 0 {
		return nil, &NetworkMetadataError{Code: "invalid_response"}
	}

	unsigned := url.Values{
		"recvWindow": {"5000"},
		"timestamp":  {fmt.Sprintf("%d", serverClock.ServerTime)},
	}.Encode()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	query := unsigned + "&signature=" + hex.EncodeToString(mac.Sum(nil))
	endpoint, err := networkMetadataURL(baseURL, "/sapi/v1/capital/config/getall", nil)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_configuration"}
	}
	endpoint.RawQuery = query
	payload, err := fetchNetworkMetadata(ctx, client, endpoint, map[string]string{"X-MBX-APIKEY": apiKey})
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		allowed[strings.ToUpper(strings.TrimSpace(asset))] = struct{}{}
	}
	networks, err := parseBinanceAssetNetworks(payload, allowed, checkedAt)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_response"}
	}
	return networks, nil
}

func networkMetadataURL(baseURL, requestPath string, query url.Values) (*url.URL, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("invalid base URL")
	}
	endpoint.Path = path.Join(endpoint.Path, requestPath)
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}
	return endpoint, nil
}

func fetchNetworkMetadata(ctx context.Context, client *http.Client, endpoint *url.URL, headers map[string]string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "invalid_configuration"}
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, &NetworkMetadataError{Code: "unavailable"}
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, networkMetadataResponseLimit+1))
	if readErr != nil || len(body) > networkMetadataResponseLimit {
		return nil, &NetworkMetadataError{Code: "invalid_response"}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, classifyNetworkMetadataHTTPError(response.StatusCode, body)
	}
	return body, nil
}

func classifyNetworkMetadataHTTPError(status int, payload []byte) error {
	var exchangeError struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(payload, &exchangeError)
	switch {
	case exchangeError.Code == -2014 || exchangeError.Code == -2015 || status == http.StatusUnauthorized:
		return &NetworkMetadataError{Code: "credentials_rejected"}
	case exchangeError.Code == -1021:
		return &NetworkMetadataError{Code: "clock_skew"}
	case status == http.StatusForbidden:
		return &NetworkMetadataError{Code: "permission_denied"}
	case status == http.StatusTooManyRequests:
		return &NetworkMetadataError{Code: "rate_limited"}
	default:
		return &NetworkMetadataError{Code: "unavailable"}
	}
}
