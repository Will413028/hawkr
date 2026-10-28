package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// IDOR (B-198 batch A): a seller must not be able to read an arbitrary buyer's
// PII (phone / nickname / avatar) unless that buyer has an order relationship
// with the seller. CRM customer records are created from order.completed
// events, so "has an order with this seller" is the canonical relationship.

const (
	idorSellerID  = int64(99)
	idorBuyerID   = int64(2688657200)
	idorBuyerPh   = "0912345678"
	idorBuyerNick = "Alice"
)

func setupCrmPIIRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeSellerAuth())
	v1.GET("/customers/lookup", h.LookupBuyer)
	v1.GET("/customers/:buyerId/meta", h.GetBuyerMeta)
	return r
}

// orderRelMock returns an Order client whose GetBuyerOrdersForSeller reports
// `total` orders between the buyer and seller (the relationship signal).
func orderRelMock(total string) *grpcclient.OrderClient {
	return grpcclient.NewOrderClientForTest(&mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			return &orderpb.GetBuyerOrdersForSellerResponse{Code: 0, Total: total, Page: "1", PageSize: "1"}, nil
		},
	})
}

func buyerMetaMock() *grpcclient.BuyerClient {
	return grpcclient.NewBuyerClientForTest(&mockBuyerServiceClient{
		BatchGetBuyerMetaByBuyerIDsFn: func(ctx context.Context, in *buyerpb.BatchGetBuyerMetaByBuyerIDsRequest, opts ...grpc.CallOption) (*buyerpb.BatchGetBuyerMetaByBuyerIDsResponse, error) {
			return &buyerpb.BatchGetBuyerMetaByBuyerIDsResponse{
				Code:   0,
				Buyers: map[int64]*buyerpb.BuyerMeta{idorBuyerID: {BuyerId: idorBuyerID, Nickname: idorBuyerNick, AvatarKey: "https://oss/a.jpg"}},
			}, nil
		},
		GetBuyerProfileByBuyerIDFn: func(ctx context.Context, in *buyerpb.GetBuyerProfileByBuyerIDRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error) {
			return &buyerpb.BuyerProfileResponse{Code: 0, Data: &buyerpb.BuyerInfo{Phone: idorBuyerPh}}, nil
		},
	})
}

func accountByPhoneMock() *grpcclient.AccountClient {
	return grpcclient.NewAccountClientForTest(&mockAccountServiceClient{
		GetAccountByPhoneFn: func(ctx context.Context, in *accountpb.GetAccountByPhoneRequest, opts ...grpc.CallOption) (*accountpb.AccountInfoResponse, error) {
			return &accountpb.AccountInfoResponse{Data: &accountpb.AccountInfo{AccountId: idorBuyerID, Phone: idorBuyerPh}}, nil
		},
	})
}

func idorRequest(t *testing.T, h *Holder, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := setupCrmPIIRouter(h)
	req, _ := http.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---- LookupBuyer ----

func TestLookupBuyer_ByPhone_NoRelationship_DeniesPII(t *testing.T) {
	h := &Holder{Account: accountByPhoneMock(), Buyer: buyerMetaMock(), Order: orderRelMock("0"), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/lookup?phone=0912345678")

	assertHTTPStatus(t, w, http.StatusNotFound)
	if body := w.Body.String(); strings.Contains(body, idorBuyerNick) {
		t.Fatalf("unrelated buyer PII leaked via phone lookup: %s", body)
	}
}

// ---- GetBuyerMeta ----

func TestGetBuyerMeta_NoRelationship_Denies(t *testing.T) {
	h := &Holder{Buyer: buyerMetaMock(), Order: orderRelMock("0"), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/2688657200/meta")

	assertHTTPStatus(t, w, http.StatusNotFound)
	if body := w.Body.String(); strings.Contains(body, idorBuyerNick) {
		t.Fatalf("unrelated buyer nickname leaked: %s", body)
	}
}

func TestGetBuyerMeta_WithRelationship_Returns(t *testing.T) {
	h := &Holder{Buyer: buyerMetaMock(), Order: orderRelMock("1"), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/2688657200/meta")

	assertHTTPStatus(t, w, http.StatusOK)
	if body := w.Body.String(); !strings.Contains(body, idorBuyerNick) {
		t.Fatalf("related buyer nickname should be returned: %s", body)
	}
}

func TestLookupBuyer_ByBuyerID_NoDoubleResolve(t *testing.T) {
	h := &Holder{
		Buyer:   buyerMetaMock(), // fixture keyed on idorBuyerID (2688657200)
		Order:   orderRelMock("3"),
		Store:   grandfatherResolverForTest(),
		BuyerID: mustNotResolveBuyer(t),
	}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/lookup?buyer_id=2688657200")

	assertHTTPStatus(t, w, http.StatusOK)
	if body := w.Body.String(); !strings.Contains(body, idorBuyerNick) {
		t.Fatalf("buyer_id-keyed lookup should return the buyer's PII: %s", body)
	}
	if body := w.Body.String(); !strings.Contains(body, `"uid":"2688657200"`) {
		t.Fatalf("uid should echo the buyer_id verbatim (no resolution): %s", body)
	}
}

func TestLookupBuyer_PhoneAndBuyerID_MutuallyExclusive(t *testing.T) {
	h := &Holder{
		Account: accountByPhoneMock(),
		Buyer:   buyerMetaMock(),
		Order:   orderRelMock("3"),
		Store:   grandfatherResolverForTest(),
		BuyerID: mustNotResolveBuyer(t),
	}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/lookup?buyer_id=2688657200&phone=0912345678")

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestLookupBuyer_InvalidBuyerID(t *testing.T) {
	h := &Holder{
		Buyer:   buyerMetaMock(),
		Order:   orderRelMock("3"),
		Store:   grandfatherResolverForTest(),
		BuyerID: mustNotResolveBuyer(t),
	}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/lookup?buyer_id=0")

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if body := w.Body.String(); !strings.Contains(body, "buyer_id") {
		t.Fatalf("error message should name buyer_id, not a generic/legacy field name: %s", body)
	}
}

// TestGetBuyerMeta_ClosedStint_Returns404_IsPIIBoundary 釘住一個**預期行為**，
// 不是待修的 bug：對已關閉 stint 的 buyer_id 打 /meta 一律 404。
//
// 推導（2026-07-27 對 buyer 模組 schema/SQL 逐條核對）：
//   - GetBuyersByBuyerIDs（buyers.sql:14-15）不濾 is_current，但濾
//     `deleted_at IS NULL`
//   - RetireCurrentStint（:58-59）是全 repo 唯一把 is_current 設 false 的地方，
//     它不動 deleted_at
//   - 但 CHECK chk_buyers_active_is_current（schema.sql:21）是
//     `status = 'closed' OR is_current`——要 is_current=false，status 必須先是
//     'closed'
//   - 而唯一能設 status='closed' 的 UpdateBuyerStatus（:67-71）的 SQL 是
//     `deleted_at = CASE WHEN status='closed' THEN now ELSE deleted_at END`，
//     必然同時蓋上 deleted_at
//
// 所以「非現任 stint」與「deleted_at 已設」是同一件事的兩種說法，沒有例外
// 路徑：已關閉 stint 的 meta 一定查不到。這是刻意的 PII 邊界（帳號關閉後不再
// 供應個資，PII cleanup cron 用的也是同一個 deleted_at），賣家自己的交易歷史
// （stats / orders / coupons / tags）則仍看得到——那四張表直接吃
// (store_id, buyer_id)，不經過 buyers 表。
//
// ⚠️ 不要為了讓這支「過」而去放寬 buyer 模組的 deleted_at 過濾。
//
// ⚠️ 2026-08-29（#2830）之後有第二條路，本邊界**未被破壞**：
// `BatchGetBuyerDisplayByBuyerIDs` 對已關閉 stint 查得出來，但只回**常數**
// 墓碑字串「已刪除帳號」＋空頭像，真名與其他 PII 一律不出 buyer 模組。
// 「查得到」與「供應個資」是兩件事 —— 本測試守的是後者。
func TestGetBuyerMeta_ClosedStint_Returns404_IsPIIBoundary(t *testing.T) {
	h := &Holder{
		Buyer:   buyerMetaMock(), // 只認 idorBuyerID；其餘視同已關閉 stint
		Order:   orderRelMock("1"),
		Store:   grandfatherResolverForTest(),
		BuyerID: buyerGrandfatherResolverForTest(),
	}
	// 2619321233 不在 buyer fixture 內 = 已關閉 stint（deleted_at 已設）
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/2619321233/meta")

	assertHTTPStatus(t, w, http.StatusNotFound)
	if body := w.Body.String(); strings.Contains(body, idorBuyerNick) {
		t.Fatalf("closed stint must not leak buyer PII: %s", body)
	}
}

func TestGetBuyerMeta_UsesPathBuyerIDVerbatim(t *testing.T) {
	h := &Holder{
		Buyer: buyerMetaMock(), // fixture 只認 idorBuyerID 這把 key
		Order: orderRelMock("1"),
		Store: grandfatherResolverForTest(),
		// resolver 會把任何輸入翻成 55555——若 handler 還在解析，buyer fixture
		// 就會 miss 而回 404，這支測試就抓得到。
		BuyerID: buyerResolverForTest(55555),
	}
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/2688657200/meta")

	assertHTTPStatus(t, w, http.StatusOK)
	if body := w.Body.String(); !strings.Contains(body, idorBuyerNick) {
		t.Fatalf("path buyer_id must be used verbatim (no resolution): %s", body)
	}
	if body := w.Body.String(); !strings.Contains(body, `"buyerId":"2688657200"`) {
		t.Fatalf("response buyerId must echo the path buyer_id, not a resolved one: %s", body)
	}
}
