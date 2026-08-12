package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"futures-arbitrage-scanner/exchanges"
)

const (
	networkMetadataRefreshInterval = 5 * time.Minute
	networkMetadataRequestTimeout  = 10 * time.Second
	binanceAPIBaseURL              = "https://api.binance.com"
	gateAPIBaseURL                 = "https://api.gateio.ws"
	kuCoinAPIBaseURL               = "https://api.kucoin.com"
)

var trackedNetworkAssets = []string{"BTC", "ETH", "XRP", "SOL", "COTI"}

type networkSourceFetcher func(context.Context, []string, time.Time) map[string]networkVenueSnapshot

type networkSourceDefinition struct {
	Source string
	Fetch  networkSourceFetcher
}

type networkCatalog struct {
	assets    []string
	sources   []networkSourceDefinition
	mutex     sync.RWMutex
	snapshots map[string]map[string]networkVenueSnapshot
}

func newNetworkCatalog(assets []string, sources []networkSourceDefinition) *networkCatalog {
	catalog := &networkCatalog{
		assets: append([]string(nil), assets...), sources: append([]networkSourceDefinition(nil), sources...),
		snapshots: make(map[string]map[string]networkVenueSnapshot, len(assets)),
	}
	for _, rawAsset := range assets {
		asset := strings.ToUpper(strings.TrimSpace(rawAsset))
		catalog.snapshots[asset] = make(map[string]networkVenueSnapshot, len(sources))
		for _, source := range sources {
			catalog.snapshots[asset][source.Source] = networkVenueSnapshot{
				Source: source.Source, Asset: asset, Status: networkVenueLoading, Networks: []exchanges.AssetNetwork{},
			}
		}
	}
	return catalog
}

func newProductionNetworkCatalog(binanceAPIKey, binanceSecret string) *networkCatalog {
	client := &http.Client{Timeout: networkMetadataRequestTimeout}
	sources := []networkSourceDefinition{
		{
			Source: sourceBinanceSpot,
			Fetch: func(ctx context.Context, assets []string, checkedAt time.Time) map[string]networkVenueSnapshot {
				result := make(map[string]networkVenueSnapshot, len(assets))
				networks, err := exchanges.FetchBinanceAssetNetworks(
					ctx, client, binanceAPIBaseURL, binanceAPIKey, binanceSecret, assets, checkedAt,
				)
				if err != nil {
					return unavailableNetworkSnapshots(sourceBinanceSpot, assets, exchanges.NetworkMetadataErrorCode(err), checkedAt)
				}
				for _, asset := range assets {
					result[asset] = readyNetworkSnapshot(sourceBinanceSpot, asset, networks[asset], checkedAt)
				}
				return result
			},
		},
		{
			Source: sourceGateSpot,
			Fetch: publicAssetNetworkFetcher(sourceGateSpot, func(ctx context.Context, asset string, checkedAt time.Time) ([]exchanges.AssetNetwork, error) {
				return exchanges.FetchGateAssetNetworks(ctx, client, gateAPIBaseURL, asset, checkedAt)
			}),
		},
		{
			Source: sourceKuCoinSpot,
			Fetch: publicAssetNetworkFetcher(sourceKuCoinSpot, func(ctx context.Context, asset string, checkedAt time.Time) ([]exchanges.AssetNetwork, error) {
				return exchanges.FetchKuCoinAssetNetworks(ctx, client, kuCoinAPIBaseURL, asset, checkedAt)
			}),
		},
	}
	return newNetworkCatalog(trackedNetworkAssets, sources)
}

func publicAssetNetworkFetcher(
	source string,
	fetch func(context.Context, string, time.Time) ([]exchanges.AssetNetwork, error),
) networkSourceFetcher {
	return func(ctx context.Context, assets []string, checkedAt time.Time) map[string]networkVenueSnapshot {
		result := make(map[string]networkVenueSnapshot, len(assets))
		for _, asset := range assets {
			networks, err := fetch(ctx, asset, checkedAt)
			if err != nil {
				result[asset] = networkVenueSnapshot{
					Source: source, Asset: asset, Status: networkVenueUnavailable,
					ErrorCode: exchanges.NetworkMetadataErrorCode(err), CheckedAt: checkedAt.UnixMilli(),
					Networks: []exchanges.AssetNetwork{},
				}
				continue
			}
			result[asset] = readyNetworkSnapshot(source, asset, networks, checkedAt)
		}
		return result
	}
}

func readyNetworkSnapshot(source, asset string, networks []exchanges.AssetNetwork, checkedAt time.Time) networkVenueSnapshot {
	return networkVenueSnapshot{
		Source: source, Asset: asset, Status: networkVenueReady, CheckedAt: checkedAt.UnixMilli(),
		Networks: append([]exchanges.AssetNetwork(nil), networks...),
	}
}

func unavailableNetworkSnapshots(source string, assets []string, errorCode string, checkedAt time.Time) map[string]networkVenueSnapshot {
	result := make(map[string]networkVenueSnapshot, len(assets))
	for _, asset := range assets {
		result[asset] = networkVenueSnapshot{
			Source: source, Asset: asset, Status: networkVenueUnavailable, ErrorCode: errorCode,
			CheckedAt: checkedAt.UnixMilli(), Networks: []exchanges.AssetNetwork{},
		}
	}
	return result
}

func (c *networkCatalog) RefreshAt(ctx context.Context, checkedAt time.Time) {
	type sourceResult struct {
		source    string
		snapshots map[string]networkVenueSnapshot
	}
	results := make(chan sourceResult, len(c.sources))
	var waitGroup sync.WaitGroup
	for _, definition := range c.sources {
		definition := definition
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			results <- sourceResult{source: definition.Source, snapshots: definition.Fetch(ctx, c.assets, checkedAt)}
		}()
	}
	waitGroup.Wait()
	close(results)

	c.mutex.Lock()
	defer c.mutex.Unlock()
	for result := range results {
		for _, asset := range c.assets {
			snapshot, exists := result.snapshots[asset]
			if !exists {
				snapshot = networkVenueSnapshot{
					Source: result.source, Asset: asset, Status: networkVenueUnavailable,
					ErrorCode: "invalid_response", CheckedAt: checkedAt.UnixMilli(), Networks: []exchanges.AssetNetwork{},
				}
			}
			if c.snapshots[asset] == nil {
				c.snapshots[asset] = make(map[string]networkVenueSnapshot)
			}
			c.snapshots[asset][result.source] = cloneNetworkVenueSnapshot(snapshot)
		}
	}
}

func (c *networkCatalog) Run(ctx context.Context) {
	c.RefreshAt(ctx, time.Now())
	ticker := time.NewTicker(networkMetadataRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case checkedAt := <-ticker.C:
			c.RefreshAt(ctx, checkedAt)
		}
	}
}

func (c *networkCatalog) Snapshots(asset string) map[string]networkVenueSnapshot {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	sourceSnapshots := c.snapshots[asset]
	result := make(map[string]networkVenueSnapshot, len(sourceSnapshots))
	for source, snapshot := range sourceSnapshots {
		result[source] = cloneNetworkVenueSnapshot(snapshot)
	}
	return result
}

func cloneNetworkVenueSnapshot(snapshot networkVenueSnapshot) networkVenueSnapshot {
	snapshot.Networks = append([]exchanges.AssetNetwork(nil), snapshot.Networks...)
	return snapshot
}
