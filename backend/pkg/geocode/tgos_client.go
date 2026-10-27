// pkg/geocode/tgos_client.go
package geocode

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"google.golang.org/api/idtoken"
)

// Suggestion is one address autocomplete candidate.
type Suggestion struct {
	FullAddress string
	County      string
	District    string
	Detail      string
}

// ValidationResult is the outcome of validating a full address against TGOS.
// Valid=false means no exact match was found.
type ValidationResult struct {
	FullAddress string
	County      string
	District    string
	Detail      string
	Lat         float64
	Lng         float64
	Valid       bool
}

// TGOSClient calls TGOS (內政部國土測繪中心) QueryAddr through the tgos-proxy
// Cloud Run relay (IAM-protected, fixed Taiwan egress IP — TGOS requires a
// Taiwan-whitelisted caller IP and backend-payuni's primary infra has none).
type TGOSClient struct {
	baseURL    string
	appID      string
	apiKey     string
	httpClient *http.Client
}

// NewTGOSClient builds a TGOSClient authenticated against the IAM-protected
// tgos-proxy using a Google ID token signed from the given service account
// key file. The returned http.Client signs, caches, and auto-refreshes the
// token internally (google.golang.org/api/idtoken) — no manual token
// lifecycle management needed here.
func NewTGOSClient(ctx context.Context, baseURL, appID, apiKey, saKeyFile string) (*TGOSClient, error) {
	httpClient, err := idtoken.NewClient(ctx, baseURL, idtoken.WithAuthCredentialsFile(idtoken.ServiceAccount, saKeyFile))
	if err != nil {
		return nil, fmt.Errorf("tgos idtoken client: %w", err)
	}
	// idtoken.NewClient doesn't set a timeout; a missing one would hang
	// callers indefinitely, same rationale as GoogleClient's. 10s (vs
	// GoogleClient's 5s) because a cold tgos-proxy Cloud Run instance +
	// ID token signing + the actual TGOS round trip measurably exceeded
	// 5s on dev's first request after scale-to-zero (confirmed 2026-07-17).
	httpClient.Timeout = 10 * time.Second
	return &TGOSClient{baseURL: baseURL, appID: appID, apiKey: apiKey, httpClient: httpClient}, nil
}

// tgosEnvelope unwraps the ASMX HTTP-GET response shape:
// <string xmlns="http://tempuri.org/">{...json...}</string>.
type tgosEnvelope struct {
	Value string `xml:",chardata"`
}

type tgosQueryAddrResponse struct {
	Info        []tgosInfo        `json:"Info"`
	AddressList []tgosAddressItem `json:"AddressList"`
}

type tgosInfo struct {
	OutMatchType string `json:"OutMatchType"`
}

type tgosAddressItem struct {
	FullAddr string  `json:"FULL_ADDR"`
	County   string  `json:"COUNTY"`
	Town     string  `json:"TOWN"`
	X        float64 `json:"X"`
	Y        float64 `json:"Y"`
}

func (c *TGOSClient) queryAddr(ctx context.Context, params url.Values) (*tgosQueryAddrResponse, error) {
	params.Set("oAPPId", c.appID)
	params.Set("oAPIKey", c.apiKey)
	params.Set("oSRS", "EPSG:4326")
	params.Set("oResultDataType", "JSON")
	params.Set("oIsSupportPast", "false")
	params.Set("oIsShowCodeBase", "false")
	params.Set("oIsLockCounty", "false")
	params.Set("oIsLockTown", "false")
	params.Set("oIsLockVillage", "false")
	params.Set("oIsLockRoadSection", "false")
	params.Set("oIsLockLane", "false")
	params.Set("oIsLockAlley", "false")
	params.Set("oIsLockArea", "false")
	params.Set("oIsSameNumber_SubNumber", "false")
	params.Set("oCanIgnoreVillage", "true")
	params.Set("oCanIgnoreNeighborhood", "true")

	u := c.baseURL + "/addrws/v40/QueryAddr.asmx/QueryAddr?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		// This client always sends every documented parameter, so TGOS
		// should never legitimately 500 here (that only happens on a
		// missing param, confirmed to come back as a plaintext body — not
		// the XML envelope this code expects below). Any non-200 is a hard
		// error, not something to parse further.
		return nil, fmt.Errorf("tgos proxy returned status %d: %s", resp.StatusCode, body)
	}

	var envelope tgosEnvelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("tgos response envelope: %w (body: %s)", err, body)
	}

	var parsed tgosQueryAddrResponse
	if err := json.Unmarshal([]byte(envelope.Value), &parsed); err != nil {
		return nil, fmt.Errorf("tgos response json: %w (body: %s)", err, envelope.Value)
	}
	return &parsed, nil
}

// Suggest 模糊搜尋，用於地址自動完成，回傳所有候選（不限完全比對）。
func (c *TGOSClient) Suggest(ctx context.Context, county, district, query string) ([]Suggestion, error) {
	params := url.Values{}
	params.Set("oAddress", county+district+query)
	params.Set("oFuzzyType", "2")
	params.Set("oFuzzyBuffer", "0")
	params.Set("oIsOnlyFullMatch", "false")
	params.Set("oReturnMaxCount", "5")

	resp, err := c.queryAddr(ctx, params)
	if err != nil {
		return nil, err
	}

	results := make([]Suggestion, 0, len(resp.AddressList))
	for _, item := range resp.AddressList {
		results = append(results, Suggestion{
			FullAddress: item.FullAddr,
			County:      item.County,
			District:    item.Town,
			Detail:      extractDetail(item.FullAddr, item.County, item.Town),
		})
	}
	return results, nil
}

// Validate 將完整地址轉為座標並判斷有效性。oIsOnlyFullMatch=true 讓 TGOS 自己
// 過濾成只回傳完全比對，模糊比對的地址會直接得到空結果（見
// testdata/tgos_queryaddr_response_validate_nomatch.json 實測驗證），不需要
// 額外檢查 OutMatchType。
func (c *TGOSClient) Validate(ctx context.Context, address string) (*ValidationResult, error) {
	params := url.Values{}
	params.Set("oAddress", address)
	params.Set("oFuzzyType", "0")
	params.Set("oFuzzyBuffer", "0")
	params.Set("oIsOnlyFullMatch", "true")
	params.Set("oReturnMaxCount", "1")

	resp, err := c.queryAddr(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(resp.AddressList) == 0 {
		return &ValidationResult{Valid: false}, nil
	}

	item := resp.AddressList[0]
	return &ValidationResult{
		FullAddress: item.FullAddr,
		County:      item.County,
		District:    item.Town,
		Detail:      extractDetail(item.FullAddr, item.County, item.Town),
		Lat:         item.Y,
		Lng:         item.X,
		Valid:       true,
	}, nil
}
