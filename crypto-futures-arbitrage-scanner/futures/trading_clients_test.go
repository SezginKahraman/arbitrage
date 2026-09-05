package futures

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGateTraderSignsPreflightOpenAndReduceOnlyClose(t *testing.T) {
	const secret = "gate-secret"
	var orderBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("KEY") != "gate-key" || !validGateTestSignature(r, body, secret) {
			t.Fatalf("invalid Gate authentication for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/futures/usdt/accounts":
			_, _ = w.Write([]byte(`{"available":"500","in_dual_mode":false}`))
		case "/api/v4/futures/usdt/positions/COTI_USDT":
			_, _ = w.Write([]byte(`{"contract":"COTI_USDT","size":"0"}`))
		case "/api/v4/futures/usdt/orders":
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			orderBodies = append(orderBodies, payload)
			_, _ = w.Write([]byte(`{"id":123,"contract":"COTI_USDT","size":"12","left":"0","fill_price":"0.0106","status":"finished"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	trader := NewGateTraderWithBaseURL(server.URL, server.Client(), "gate-key", secret, func() time.Time { return time.Unix(1_700_000_000, 0) })
	if err := trader.Preflight(context.Background(), "COTI_USDT", 100); err != nil {
		t.Fatal(err)
	}
	fill, err := trader.PlaceMarket(context.Background(), OrderIntent{Venue: "gate_futures", Contract: "COTI_USDT", Side: OrderBuy, Contracts: 12})
	if err != nil || fill.OrderID != "123" || fill.FilledContracts != 12 {
		t.Fatalf("fill = %+v, err = %v", fill, err)
	}
	_, err = trader.CloseFilled(context.Background(), OrderIntent{Venue: "gate_futures", Contract: "COTI_USDT", Side: OrderSell, Contracts: 12, ReduceOnly: true}, fill)
	if err != nil {
		t.Fatal(err)
	}
	if len(orderBodies) != 2 || orderBodies[0]["size"] != "12" || orderBodies[0]["price"] != "0" || orderBodies[0]["tif"] != "ioc" {
		t.Fatalf("open body = %#v", orderBodies)
	}
	if orderBodies[1]["size"] != "0" || orderBodies[1]["close"] != true || orderBodies[1]["reduce_only"] != true {
		t.Fatalf("close body = %#v", orderBodies[1])
	}
}

func validGateTestSignature(r *http.Request, body []byte, secret string) bool {
	hash := sha512.Sum512(body)
	payload := strings.Join([]string{r.Method, r.URL.Path, r.URL.RawQuery, hex.EncodeToString(hash[:]), r.Header.Get("Timestamp")}, "\n")
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hmac.Equal([]byte(r.Header.Get("SIGN")), []byte(hex.EncodeToString(mac.Sum(nil))))
}

func TestBinanceTraderSignsPreflightOpenAndReduceOnlyClose(t *testing.T) {
	const secret = "binance-secret"
	var orderForms []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-MBX-APIKEY") != "binance-key" {
			t.Fatalf("missing Binance key")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		signature := r.Form.Get("signature")
		r.Form.Del("signature")
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(r.Form.Encode()))
		if !hmac.Equal([]byte(signature), []byte(hex.EncodeToString(mac.Sum(nil)))) {
			t.Fatalf("invalid Binance signature for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/fapi/v1/accountConfig":
			_, _ = w.Write([]byte(`{"canTrade":true,"dualSidePosition":false}`))
		case "/fapi/v2/positionRisk":
			_, _ = w.Write([]byte(`[{"symbol":"COTIUSDT","positionAmt":"0","positionSide":"BOTH"}]`))
		case "/fapi/v3/balance":
			_, _ = w.Write([]byte(`[{"asset":"USDT","availableBalance":"500"}]`))
		case "/fapi/v1/order":
			cloned := make(url.Values, len(r.Form))
			for key, values := range r.Form {
				cloned[key] = append([]string(nil), values...)
			}
			orderForms = append(orderForms, cloned)
			_, _ = w.Write([]byte(`{"orderId":456,"symbol":"COTIUSDT","status":"FILLED","executedQty":"12","avgPrice":"0.0106"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	trader := NewBinanceTraderWithBaseURL(server.URL, server.Client(), "binance-key", secret, func() time.Time { return time.UnixMilli(1_700_000_000_000) })
	if err := trader.Preflight(context.Background(), "COTI_USDT", 100); err != nil {
		t.Fatal(err)
	}
	fill, err := trader.PlaceMarket(context.Background(), OrderIntent{Venue: "binance_futures", Contract: "COTI_USDT", Side: OrderSell, Contracts: 12})
	if err != nil || fill.OrderID != "456" || fill.FilledContracts != 12 {
		t.Fatalf("fill = %+v, err = %v", fill, err)
	}
	_, err = trader.CloseFilled(context.Background(), OrderIntent{Venue: "binance_futures", Contract: "COTI_USDT", Side: OrderBuy, Contracts: 12, ReduceOnly: true}, fill)
	if err != nil {
		t.Fatal(err)
	}
	if len(orderForms) != 2 || orderForms[0].Get("side") != "SELL" || orderForms[0].Get("type") != "MARKET" || orderForms[0].Get("reduceOnly") != "" {
		t.Fatalf("open form = %#v", orderForms)
	}
	if orderForms[1].Get("side") != "BUY" || orderForms[1].Get("reduceOnly") != "true" {
		t.Fatalf("close form = %#v", orderForms[1])
	}
}

func TestBinanceTraderPlacesGTXProbeCancelsItAndVerifiesFlatPosition(t *testing.T) {
	const secret = "binance-secret"
	var placed url.Values
	positionChecks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-MBX-APIKEY") != "binance-key" {
			t.Fatalf("missing Binance key")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		signature := r.Form.Get("signature")
		r.Form.Del("signature")
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(r.Form.Encode()))
		if !hmac.Equal([]byte(signature), []byte(hex.EncodeToString(mac.Sum(nil)))) {
			t.Fatalf("invalid Binance signature for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/accountConfig":
			_, _ = w.Write([]byte(`{"canTrade":true,"dualSidePosition":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/fapi/v2/positionRisk":
			positionChecks++
			_, _ = w.Write([]byte(`[{"symbol":"HUSDT","positionAmt":"0","positionSide":"BOTH"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/fapi/v3/balance":
			_, _ = w.Write([]byte(`[{"asset":"USDT","availableBalance":"50"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/fapi/v1/order":
			placed = make(url.Values, len(r.Form))
			for key, values := range r.Form {
				placed[key] = append([]string(nil), values...)
			}
			_, _ = w.Write([]byte(`{"orderId":456,"symbol":"HUSDT","status":"NEW","executedQty":"0","avgPrice":"0"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/fapi/v1/order":
			_, _ = w.Write([]byte(`{"orderId":456,"symbol":"HUSDT","status":"CANCELED","executedQty":"0","avgPrice":"0"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	trader := NewBinanceTraderWithBaseURL(server.URL, server.Client(), "binance-key", secret, func() time.Time { return time.UnixMilli(1_700_000_000_000) })
	if err := trader.Preflight(context.Background(), "H_USDT", 5.1); err != nil {
		t.Fatal(err)
	}
	result, err := trader.ProbePostOnly(context.Background(), OrderProbePlan{
		Venue: "binance_futures", Contract: "H_USDT", Side: OrderBuy,
		Price: 0.13338, Contracts: 38, Notional: 5.06844,
	})

	if err != nil {
		t.Fatal(err)
	}
	if placed.Get("type") != "LIMIT" || placed.Get("timeInForce") != "GTX" || placed.Get("side") != "BUY" ||
		placed.Get("price") != "0.13338" || placed.Get("quantity") != "38" {
		t.Fatalf("placed form = %#v", placed)
	}
	if result.Status != OrderProbeCancelled || result.OrderID != "456" || !result.CleanupVerified || positionChecks != 2 {
		t.Fatalf("result = %+v, position checks = %d", result, positionChecks)
	}
}

func TestGateTraderPlacesPOCProbeCancelsItAndVerifiesFlatPosition(t *testing.T) {
	const secret = "gate-secret"
	var placed map[string]any
	positionChecks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("KEY") != "gate-key" || !validGateTestSignature(r, body, secret) {
			t.Fatalf("invalid Gate authentication for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/futures/usdt/accounts":
			_, _ = w.Write([]byte(`{"available":"50","in_dual_mode":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/futures/usdt/positions/H_USDT":
			positionChecks++
			_, _ = w.Write([]byte(`{"contract":"H_USDT","size":"0"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/futures/usdt/orders":
			if err := json.Unmarshal(body, &placed); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"id":123,"contract":"H_USDT","size":"1","left":"1","price":"0.13307","status":"open"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v4/futures/usdt/orders/123":
			_, _ = w.Write([]byte(`{"id":123,"contract":"H_USDT","size":"1","left":"1","price":"0.13307","status":"finished","finish_as":"cancelled"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	trader := NewGateTraderWithBaseURL(server.URL, server.Client(), "gate-key", secret, func() time.Time { return time.Unix(1_700_000_000, 0) })
	if err := trader.Preflight(context.Background(), "H_USDT", 1.34); err != nil {
		t.Fatal(err)
	}
	result, err := trader.ProbePostOnly(context.Background(), OrderProbePlan{
		Venue: "gate_futures", Contract: "H_USDT", Side: OrderBuy,
		Price: 0.13307, Contracts: 1, Notional: 1.3307,
	})

	if err != nil {
		t.Fatal(err)
	}
	if placed["tif"] != "poc" || placed["price"] != "0.13307" || placed["size"] != "1" {
		t.Fatalf("placed body = %#v", placed)
	}
	if result.Status != OrderProbeCancelled || result.OrderID != "123" || !result.CleanupVerified || positionChecks != 2 {
		t.Fatalf("result = %+v, position checks = %d", result, positionChecks)
	}
}

func TestGateTraderTreatsPositionNotFoundAsAFlatPreflight(t *testing.T) {
	const secret = "gate-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("KEY") != "gate-key" || !validGateTestSignature(r, body, secret) {
			t.Fatalf("invalid Gate authentication for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/futures/usdt/accounts":
			_, _ = w.Write([]byte(`{"available":"50","in_dual_mode":false}`))
		case "/api/v4/futures/usdt/positions/H_USDT":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"label":"POSITION_NOT_FOUND","message":"Position not found"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	trader := NewGateTraderWithBaseURL(server.URL, server.Client(), "gate-key", secret, func() time.Time { return time.Unix(1_700_000_000, 0) })
	if err := trader.Preflight(context.Background(), "H_USDT", 5); err != nil {
		t.Fatalf("flat Gate account was rejected: %v", err)
	}
}
