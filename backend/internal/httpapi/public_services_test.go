package httpapi

import (
	"context"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/discovery"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/discovery/v1alpha1/discoveryv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	discoverymod "github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type publicModule struct{}

func (publicModule) PublicStoreByCode(context.Context, int64) (seller.PublicStore, error) {
	return seller.PublicStore{Store: seller.Store{Profile: seller.Profile{StoreName: "範例小農"}, StoreID: 11, StoreCode: 2600000001}}, nil
}

func (publicModule) ListPublicStores(context.Context, seller.ListPublicStoresInput) (seller.PublicStorePage, error) {
	return seller.PublicStorePage{Items: []seller.PublicStore{{Store: seller.Store{Profile: seller.Profile{StoreName: "範例小農"}, StoreID: 11, StoreCode: 2600000001}}}}, nil
}

func (publicModule) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	name := "範例小農"
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		out[i] = seller.StoreCard{StoreID: id, StoreCode: 2600000000 + id, Name: &name, Status: seller.CardOpen}
	}
	return out, nil
}

func (publicModule) RecordProductView(context.Context, int64) error { return nil }

type publicFollowers struct{}

func (publicFollowers) SubscriberCounts(_ context.Context, ids []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(ids))
	for _, id := range ids {
		out[id] = 1
	}
	return out, nil
}

type publicRatings struct{}

func (publicRatings) StoreRatingStats(context.Context, []int64) (map[int64]httpseller.RatingStats, error) {
	return map[int64]httpseller.RatingStats{}, nil
}

type publicRanking struct{}

func (publicRanking) RecommendedSellers(_ context.Context, limit int) (discoverymod.Recommendation, error) {
	return discoverymod.Recommendation{Sellers: []discoverymod.RankedSeller{{StoreID: int64(limit)}}}, nil
}

func publicEntry(t *testing.T) http.Handler {
	t.Helper()
	stats, err := httpseller.NewStatsSource(publicFollowers{}, publicRatings{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	sellerPublic, err := httpseller.NewPublic(publicModule{}, stats, rpc.Images{}, characterize.WebOrigin(t))
	if err != nil {
		t.Fatal(err)
	}
	discoveryPublic, err := discovery.NewPublic(publicRanking{}, publicModule{}, publicFollowers{}, stats, rpc.Images{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sellerPath, sellerHandler := sellerv1alpha1connect.NewSellerPublicServiceHandler(sellerPublic)
	discoveryPath, discoveryHandler := discoveryv1alpha1connect.NewDiscoveryPublicServiceHandler(discoveryPublic)
	return entry(t, nil, service{sellerPath, sellerHandler}, service{discoveryPath, discoveryHandler})
}

func connectGET(t *testing.T, procedure, message string) *http.Request {
	t.Helper()
	q := url.Values{"connect": {"v1"}, "encoding": {"json"}, "message": {message}}
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, procedure+"?"+q.Encode(), nil)
}

// 店面與店家卡片、推薦排行是沒有副作用的讀取：以 GET 服務，回應是 public, no-cache 加 ETag，沒變時回 304，CDN 與 website 才能
// 快取它們；拿掉 NO_SIDE_EFFECTS 時 connect-go 對 GET 回 405，這個測試就失敗。
func TestPublicReadsAreServedAsCacheableGETs(t *testing.T) {
	h := publicEntry(t)
	for name, c := range map[string]struct{ procedure, message string }{
		"GetStore":                  {sellerv1alpha1connect.SellerPublicServiceGetStoreProcedure, `{"storeCode":"2600000001"}`},
		"ListStores":                {sellerv1alpha1connect.SellerPublicServiceListStoresProcedure, `{}`},
		"BatchGetStores":            {sellerv1alpha1connect.SellerPublicServiceBatchGetStoresProcedure, `{"ids":["1","2"]}`},
		"GetSellerAgreements":       {sellerv1alpha1connect.SellerPublicServiceGetSellerAgreementsProcedure, `{}`},
		"SuggestRecommendedSellers": {discoveryv1alpha1connect.DiscoveryPublicServiceSuggestRecommendedSellersProcedure, `{}`},
	} {
		t.Run(name, func(t *testing.T) {
			rec := do(h, connectGET(t, c.procedure, c.message))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET = %d %s", rec.Code, rec.Body)
			}
			if got := rec.Header().Get("Cache-Control"); got != "public, no-cache" {
				t.Fatalf("Cache-Control = %q", got)
			}
			tag := rec.Header().Get("ETag")
			if tag == "" {
				t.Fatal("沒有 ETag")
			}
			again := connectGET(t, c.procedure, c.message)
			again.Header.Set("If-None-Match", tag)
			if rec := do(h, again); rec.Code != http.StatusNotModified {
				t.Fatalf("帶 If-None-Match 的 GET = %d，預期 304", rec.Code)
			}
		})
	}
}

// 寫入不是 GET：RecordProductView 只收 POST，而且不快取。
func TestRecordProductViewIsAnUncachedPOST(t *testing.T) {
	h := publicEntry(t)
	procedure := sellerv1alpha1connect.SellerPublicServiceRecordProductViewProcedure
	if rec := do(h, connectGET(t, procedure, `{"productId":"1"}`)); rec.Code == http.StatusOK {
		t.Fatalf("GET 呼叫了寫入的方法：%d", rec.Code)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, procedure, strings.NewReader(`{"productId":"1"}`))
	r.Header.Set("Content-Type", "application/json")
	rec := do(h, r)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("POST = %d Cache-Control=%q %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body)
	}
}

// 商品瀏覽的打點是匿名的公開寫入：每個來源 IP 有自己較低的額度（tierProductView），超過回 resource_exhausted；其他公開讀取
// 不受影響。
func TestRecordProductViewIsLimitedPerSourceIP(t *testing.T) {
	h := publicEntry(t)
	procedure := sellerv1alpha1connect.SellerPublicServiceRecordProductViewProcedure
	post := func(ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, procedure, strings.NewReader(`{"productId":"1"}`))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":4000"
		return do(h, r)
	}
	for i := range tierProductView.visitor {
		if rec := post("192.0.2.1"); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次 = %d", i+1, rec.Code)
		}
	}
	rec := post("192.0.2.1")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"code":"resource_exhausted"`) ||
		!strings.Contains(rec.Body.String(), `"type":"google.rpc.RetryInfo"`) {
		t.Fatalf("超過額度 = %d %q", rec.Code, rec.Body)
	}
	if rec := post("192.0.2.2"); rec.Code != http.StatusOK {
		t.Fatalf("另一個 IP 不該被擋：%d", rec.Code)
	}
	if rec := do(h, connectGET(t, sellerv1alpha1connect.SellerPublicServiceGetStoreProcedure, `{"storeCode":"2600000001"}`)); rec.Code != http.StatusOK {
		t.Fatalf("同一個 IP 的店面讀取不該被打點的額度擋住：%d", rec.Code)
	}
	if tierProductView.visitor >= tierPublic.visitor {
		t.Fatalf("打點的額度 %d 應低於一般公開讀取 %d", tierProductView.visitor, tierPublic.visitor)
	}
}

// 統計組裝（評價）與排行是店面頁與推薦排行的必要依賴：沒有它們，組裝時就失敗，不是在請求時才回錯誤。
func TestNewHandlerRequiresDiscoveryAndStats(t *testing.T) {
	deps := Deps{Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}}), Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}}
	withRatings := deps
	withRatings.Stats = characterize.Stats(t)
	if _, err := NewHandler(withRatings); err == nil || !strings.Contains(err.Error(), "Discovery") {
		t.Fatalf("沒有 Discovery：err=%v", err)
	}
	withDiscovery := deps
	withDiscovery.Discovery = &discoverymod.Module{}
	if _, err := NewHandler(withDiscovery); err == nil || !strings.Contains(err.Error(), "Stats") {
		t.Fatalf("沒有 Stats：err=%v", err)
	}
}

// 後台待辦的 handler 需要 ReviewQueue：組裝漏接時啟動就失敗，不是第一個請求才 nil pointer panic。
func TestNewHandlerRequiresReviewQueue(t *testing.T) {
	deps := Deps{
		Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}}), Deletion: &accountdeletion.Module{}, Seller: &seller.Module{},
		Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: struct{ httpseller.DashboardOrders }{}, Discovery: &discoverymod.Module{}, Agent: &agent.Module{},
	}
	if _, err := NewHandler(deps); err == nil || !strings.Contains(err.Error(), "ReviewQueue") {
		t.Fatalf("沒有 ReviewQueue：err=%v", err)
	}
}
