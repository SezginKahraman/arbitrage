package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildTestUI(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	assets := filepath.Join(directory, "assets")
	if err := os.MkdirAll(assets, 0o750); err != nil {
		t.Fatalf("create assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte(`<div id="root">React scanner</div>`), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index-abc123.js"), []byte(`console.log("scanner")`), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	return directory
}

func TestSPAHandlerServesProductionIndex(t *testing.T) {
	handler := newSPAHandler(buildTestUI(t))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "React scanner") {
		t.Fatal("GET / did not serve the React index")
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("index Cache-Control = %q, want no-store", got)
	}
}

func TestSPAHandlerFallsBackToIndexForClientRoute(t *testing.T) {
	handler := newSPAHandler(buildTestUI(t))
	request := httptest.NewRequest(http.MethodGet, "/opportunities", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "React scanner") {
		t.Fatalf("GET /opportunities did not fall back to React index: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestSPAHandlerCachesFingerprintedAssets(t *testing.T) {
	handler := newSPAHandler(buildTestUI(t))
	request := httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET asset status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control = %q, want immutable cache", got)
	}
}

func TestSPAHandlerDoesNotMaskMissingFiles(t *testing.T) {
	handler := newSPAHandler(buildTestUI(t))
	request := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("GET missing file status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
