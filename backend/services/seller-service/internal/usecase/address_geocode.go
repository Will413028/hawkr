// services/seller-service/internal/usecase/address_geocode.go
package usecase

import (
	"context"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// Suggestion is one address autocomplete candidate.
type Suggestion struct {
	FullAddress string
	County      string
	District    string
	Detail      string
}

// GeocodeValidation is the outcome of validating a full address. Valid=false
// means no precise, non-partial match was found upstream.
type GeocodeValidation struct {
	FullAddress string
	County      string
	District    string
	Detail      string
	Lat         float64
	Lng         float64
	Valid       bool
}

// GeocodeClient looks up address suggestions/validation from the configured
// geocoding provider (Google Maps, via pkg/geocode).
type GeocodeClient interface {
	Suggest(ctx context.Context, county, district, query string) ([]Suggestion, error)
	Validate(ctx context.Context, address string) (*GeocodeValidation, error)
}

// GeocodeCache is a cache-aside store for geocode lookups. Implementations
// fail open (Get returns ok=false on miss/unavailable Redis).
type GeocodeCache interface {
	GetSuggestions(ctx context.Context, key string) ([]Suggestion, bool)
	SetSuggestions(ctx context.Context, key string, items []Suggestion)
	GetValidation(ctx context.Context, key string) (*GeocodeValidation, bool)
	SetValidation(ctx context.Context, key string, v *GeocodeValidation)
}

// AddressGeocodeUseCase owns 地址查詢/驗證 for the seller's shop-address entry
// flow: cache-aside over the Google Maps anti-corruption client.
type AddressGeocodeUseCase struct {
	client GeocodeClient
	cache  GeocodeCache
}

func NewAddressGeocodeUseCase(client GeocodeClient, cache GeocodeCache) *AddressGeocodeUseCase {
	return &AddressGeocodeUseCase{client: client, cache: cache}
}

// ErrAddressLookupDisabled 是地址查詢**尚未於本環境開通**時的回應（backend#3329）。
//
// 🔴 fail-closed，不是 fail-open：未開通時回 9700，**不得**回空清單或 Valid=false——
// 那會讓「功能沒開」長得跟「查無此地址」一樣，賣家會以為自己打錯地址而反覆重試。
var ErrAddressLookupDisabled = pkgerrors.NewBusiness(
	bizcode.FeatureAddressLookupDisabled, "地址查詢服務尚未開通，請手動輸入完整地址")

// ErrPayuniMerchantDisabled 是金流商家建檔**尚未於本環境開通**時的回應（backend#3329）。
//
// 定義在 usecase 而不是 client package：它是 PayuniClient 這個**介面契約**的一部分
// （停用實作與未來任何實作都回它），而介面本身就住在這裡。放到 client 會讓 usecase
// 反向 import client，是本專案的依賴方向禁忌。
var ErrPayuniMerchantDisabled = pkgerrors.NewBusiness(
	bizcode.FeaturePayuniMerchantDisabled, "金流商家建檔服務尚未開通，請稍後再試")

// cacheKeyVersion bumps whenever a change upstream of the cache invalidates
// previously-cached results (e.g. GEOCODE_PROVIDER switch, issue #1533's
// Google→TGOS migration) — old keys go dead instead of serving a stale 24h
// negative cache from before the fix.
const cacheKeyVersion = "v3"

func suggestCacheKey(county, district, query string) string {
	return "addr:sug:" + cacheKeyVersion + ":" + county + ":" + district + ":" + query
}

func validateCacheKey(address string) string {
	return "addr:val:" + cacheKeyVersion + ":" + address
}

// Suggest 地址模糊搜尋，回傳最多 limit 筆。cache key 不含 limit，可跨不同 limit 共用同一份快取。
func (u *AddressGeocodeUseCase) Suggest(ctx context.Context, county, district, query string, limit int) ([]Suggestion, error) {
	if u.client == nil {
		return nil, ErrAddressLookupDisabled
	}
	key := suggestCacheKey(county, district, query)
	if cached, ok := u.cache.GetSuggestions(ctx, key); ok {
		return truncateSuggestions(cached, limit), nil
	}
	items, err := u.client.Suggest(ctx, county, district, query)
	if err != nil {
		return nil, err
	}
	u.cache.SetSuggestions(ctx, key, items)
	return truncateSuggestions(items, limit), nil
}

func truncateSuggestions(items []Suggestion, limit int) []Suggestion {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

// Validate 地址嚴格驗證，回傳座標與有效性。
func (u *AddressGeocodeUseCase) Validate(ctx context.Context, address string) (*GeocodeValidation, error) {
	if u.client == nil {
		return nil, ErrAddressLookupDisabled
	}
	key := validateCacheKey(address)
	if cached, ok := u.cache.GetValidation(ctx, key); ok {
		return cached, nil
	}
	result, err := u.client.Validate(ctx, address)
	if err != nil {
		return nil, err
	}
	u.cache.SetValidation(ctx, key, result)
	return result, nil
}
