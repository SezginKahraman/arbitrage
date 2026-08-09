package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServedUIOffersCOTI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	http.FileServer(http.Dir("./static")).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	const option = `<option value="COTIUSDT">COTIUSDT</option>`
	if !strings.Contains(response.Body.String(), option) {
		t.Fatalf("served UI does not contain %s", option)
	}
}

func TestServedUILoadsVersionedPriceFormatterBeforeApp(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	http.FileServer(http.Dir("./static")).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	formatterIndex := strings.Index(body, `<script src="price-format.js?v=8-decimal"></script>`)
	appIndex := strings.Index(body, `<script src="app.js?v=8-decimal"></script>`)
	if formatterIndex == -1 {
		t.Fatal("served UI does not load price-format.js")
	}
	if appIndex == -1 {
		t.Fatal("served UI does not load app.js")
	}
	if formatterIndex > appIndex {
		t.Fatal("served UI loads price-format.js after app.js")
	}
}

func TestStaticHandlerDisablesBrowserCaching(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	response := httptest.NewRecorder()
	newStaticHandler("./static").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /app.js status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}
