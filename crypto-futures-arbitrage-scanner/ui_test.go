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

func TestServedUILoadsPriceFormatterBeforeApp(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	http.FileServer(http.Dir("./static")).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	formatterIndex := strings.Index(body, `<script src="price-format.js"></script>`)
	appIndex := strings.Index(body, `<script src="app.js"></script>`)
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
