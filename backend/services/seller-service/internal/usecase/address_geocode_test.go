// services/seller-service/internal/usecase/address_geocode_test.go
package usecase

import (
	"context"
	"errors"
	"testing"
)

type fakeGeocodeClient struct {
	suggestResult  []Suggestion
	suggestErr     error
	validateResult *GeocodeValidation
	validateErr    error
}

func (f *fakeGeocodeClient) Suggest(_ context.Context, _, _, _ string) ([]Suggestion, error) {
	return f.suggestResult, f.suggestErr
}

func (f *fakeGeocodeClient) Validate(_ context.Context, _ string) (*GeocodeValidation, error) {
	return f.validateResult, f.validateErr
}

type fakeGeocodeCache struct {
	suggestions map[string][]Suggestion
	validations map[string]*GeocodeValidation
}

func newFakeGeocodeCache() *fakeGeocodeCache {
	return &fakeGeocodeCache{
		suggestions: map[string][]Suggestion{},
		validations: map[string]*GeocodeValidation{},
	}
}

func (c *fakeGeocodeCache) GetSuggestions(_ context.Context, key string) ([]Suggestion, bool) {
	v, ok := c.suggestions[key]
	return v, ok
}

func (c *fakeGeocodeCache) SetSuggestions(_ context.Context, key string, items []Suggestion) {
	c.suggestions[key] = items
}

func (c *fakeGeocodeCache) GetValidation(_ context.Context, key string) (*GeocodeValidation, bool) {
	v, ok := c.validations[key]
	return v, ok
}

func (c *fakeGeocodeCache) SetValidation(_ context.Context, key string, v *GeocodeValidation) {
	c.validations[key] = v
}

func TestAddressGeocodeUseCase_Suggest_CacheMissThenHit(t *testing.T) {
	client := &fakeGeocodeClient{suggestResult: []Suggestion{
		{FullAddress: "桃園市桃園區鎮撫街1號", County: "桃園市", District: "桃園區", Detail: "鎮撫街1號"},
	}}
	cache := newFakeGeocodeCache()
	uc := NewAddressGeocodeUseCase(client, cache)

	items, err := uc.Suggest(context.Background(), "桃園市", "桃園區", "鎮撫", 8)
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 筆，got %d", len(items))
	}

	client.suggestResult = nil
	items2, err := uc.Suggest(context.Background(), "桃園市", "桃園區", "鎮撫", 8)
	if err != nil {
		t.Fatalf("second Suggest failed: %v", err)
	}
	if len(items2) != 1 {
		t.Fatalf("cache hit 期望仍有 1 筆，got %d", len(items2))
	}
}

func TestAddressGeocodeUseCase_Suggest_ClientError(t *testing.T) {
	client := &fakeGeocodeClient{suggestErr: errors.New("places API timeout")}
	cache := newFakeGeocodeCache()
	uc := NewAddressGeocodeUseCase(client, cache)

	if _, err := uc.Suggest(context.Background(), "台北市", "中正區", "中華路", 8); err == nil {
		t.Fatal("期望 error，got nil")
	}
}

func TestAddressGeocodeUseCase_Validate_PreciseMatch(t *testing.T) {
	client := &fakeGeocodeClient{validateResult: &GeocodeValidation{
		FullAddress: "臺北市中正區中華路一段1號", County: "臺北市", District: "中正區",
		Detail: "中華路一段1號", Lat: 25.0452, Lng: 121.5107, Valid: true,
	}}
	cache := newFakeGeocodeCache()
	uc := NewAddressGeocodeUseCase(client, cache)

	result, err := uc.Validate(context.Background(), "臺北市中正區中華路一段1號")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Valid {
		t.Errorf("期望 valid=true")
	}
}

func TestAddressGeocodeUseCase_Validate_NoMatch(t *testing.T) {
	client := &fakeGeocodeClient{validateResult: &GeocodeValidation{Valid: false}}
	cache := newFakeGeocodeCache()
	uc := NewAddressGeocodeUseCase(client, cache)

	result, err := uc.Validate(context.Background(), "不存在的地址")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Valid {
		t.Errorf("期望 valid=false")
	}
}

// issue #1533 regression：臺東縣臺東市知本里育樂路1號是真實存在的偏鄉門牌，
// Google-only 時期 Suggest 回空、Validate 回 valid:false。改用 TGOS 後
// pkg/geocode 層已能正確解析，這裡釘住 usecase 的 cache-aside 邏輯不會把
// provider 回傳的成功結果吃掉/擋掉。
func TestAddressGeocodeUseCase_RuralAddress_Issue1533_Regression(t *testing.T) {
	client := &fakeGeocodeClient{
		suggestResult: []Suggestion{
			{FullAddress: "臺東縣臺東市知本里1鄰育樂路1號", County: "臺東縣", District: "臺東市", Detail: "知本里1鄰育樂路1號"},
		},
		validateResult: &GeocodeValidation{
			FullAddress: "臺東縣臺東市知本里1鄰育樂路1號", County: "臺東縣", District: "臺東市",
			Detail: "知本里1鄰育樂路1號", Lat: 22.704098, Lng: 121.063072, Valid: true,
		},
	}
	cache := newFakeGeocodeCache()
	uc := NewAddressGeocodeUseCase(client, cache)

	sugs, err := uc.Suggest(context.Background(), "臺東縣", "臺東市", "育樂路70號", 8)
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if len(sugs) == 0 {
		t.Fatal("期望至少 1 筆候選，got 0（issue #1533：偏鄉門牌不應回空）")
	}

	result, err := uc.Validate(context.Background(), "臺東縣臺東市知本里育樂路1號")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Valid {
		t.Fatal("期望 valid=true（issue #1533：偏鄉門牌不應判定無效）")
	}
}

// cache key 版本化：確保 provider 切換（issue #1533 的 Google→TGOS）能讓舊
// cache key 直接失效，不需要手動清 24h 負快取。
func TestAddressGeocodeUseCase_CacheKey_Versioned(t *testing.T) {
	if got, want := suggestCacheKey("臺東縣", "臺東市", "育樂路70號"), "addr:sug:"+cacheKeyVersion+":臺東縣:臺東市:育樂路70號"; got != want {
		t.Errorf("suggestCacheKey got %q, want %q", got, want)
	}
	if got, want := validateCacheKey("臺東縣臺東市知本里育樂路1號"), "addr:val:"+cacheKeyVersion+":臺東縣臺東市知本里育樂路1號"; got != want {
		t.Errorf("validateCacheKey got %q, want %q", got, want)
	}
}
