// pkg/geocode/tgos_client_test.go
package geocode

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestTGOSClient(t *testing.T, serverURL string) *TGOSClient {
	t.Helper()
	return &TGOSClient{
		baseURL:    serverURL,
		appID:      "test-appid",
		apiKey:     "test-apikey",
		httpClient: http.DefaultClient,
	}
}

func serveFixtureAsASMXEnvelope(t *testing.T, fixturePath string) http.HandlerFunc {
	t.Helper()
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}
	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>%s<string xmlns="http://tempuri.org/">%s</string>`, "\n", body)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(envelope))
	}
}

func TestTGOSClient_Validate_FullMatch(t *testing.T) {
	server := httptest.NewServer(serveFixtureAsASMXEnvelope(t, "testdata/tgos_queryaddr_response.json"))
	defer server.Close()

	c := newTestTGOSClient(t, server.URL)
	result, err := c.Validate(context.Background(), "台北市大安區忠孝東路四段1號")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Valid {
		t.Fatal("expected Valid=true for full match")
	}
	if result.County != "臺北市" {
		t.Errorf("County = %q, want 臺北市", result.County)
	}
	if result.District != "大安區" {
		t.Errorf("District = %q, want 大安區", result.District)
	}
	if result.Lat != 25.041841 || result.Lng != 121.544042 {
		t.Errorf("Lat/Lng = %v/%v, want 25.041841/121.544042", result.Lat, result.Lng)
	}
}

func TestTGOSClient_Validate_FuzzyOnlyMatch_ReturnsInvalid(t *testing.T) {
	server := httptest.NewServer(serveFixtureAsASMXEnvelope(t, "testdata/tgos_queryaddr_response_validate_nomatch.json"))
	defer server.Close()

	c := newTestTGOSClient(t, server.URL)
	result, err := c.Validate(context.Background(), "台北市大安區忠孝東路四段999號")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected Valid=false when only a fuzzy match exists (oIsOnlyFullMatch=true filters it server-side)")
	}
}

func TestTGOSClient_Validate_ZeroResult(t *testing.T) {
	server := httptest.NewServer(serveFixtureAsASMXEnvelope(t, "testdata/tgos_queryaddr_response_noresult.json"))
	defer server.Close()

	c := newTestTGOSClient(t, server.URL)
	result, err := c.Validate(context.Background(), "火星市外星區銀河路一段1號")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected Valid=false for zero-result address")
	}
}

func TestTGOSClient_Suggest_FuzzyMatch_ReturnsCandidate(t *testing.T) {
	server := httptest.NewServer(serveFixtureAsASMXEnvelope(t, "testdata/tgos_queryaddr_response_fuzzy.json"))
	defer server.Close()

	c := newTestTGOSClient(t, server.URL)
	results, err := c.Suggest(context.Background(), "台北市", "大安區", "忠孝東路四段999號")
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(results))
	}
	if results[0].County != "臺北市" {
		t.Errorf("County = %q, want 臺北市", results[0].County)
	}
}

func TestTGOSClient_QueryAddr_SendsCredentialsAndFixedSRS(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><string xmlns="http://tempuri.org/">{"Info":[{"OutMatchType":""}],"AddressList":[]}</string>`))
	}))
	defer server.Close()

	c := &TGOSClient{baseURL: server.URL, appID: "my-appid", apiKey: "my-apikey", httpClient: http.DefaultClient}
	if _, err := c.Validate(context.Background(), "任意地址"); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if !strings.Contains(capturedQuery, "oAPPId=my-appid") {
		t.Errorf("query missing oAPPId: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "oAPIKey=my-apikey") {
		t.Errorf("query missing oAPIKey: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "oSRS=EPSG%3A4326") {
		t.Errorf("query missing fixed oSRS=EPSG:4326: %s", capturedQuery)
	}
}
