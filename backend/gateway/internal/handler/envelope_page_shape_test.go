package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// Page-trap 收斂（10 檔 §2 統一 Envelope）：以下 7 支端點曾把 records/total/page/pageSize
// 掛在 envelope 頂層與 data 並列，前端得先分辨「這支是哪一種形狀」才讀得到資料。
// 收斂後一律 {code,message,data:{records,total,page,pageSize}}，其中 total/page/pageSize
// 為字串（§4 對外數字一律 string）。
//
// 本測試對七支各打一次真實路由，逐支斷言：
//   - envelope 頂層不得再出現任何分頁欄位（pageData 內建的回歸守門）
//   - data.records 是陣列且筆數正確
//   - data.total / data.page / data.pageSize 是字串且值正確
//
// 兩支帶額外欄位的端點（member-cards 明細的 totalSpending/totalRefund、商品評論的 stats）
// 額外斷言那些欄位也在 data 之下、且沒有在收斂過程中被弄丟。
func assertPageEnvelope(t *testing.T, body map[string]interface{}, wantRecords int, wantTotal, wantPage, wantPageSize string) map[string]interface{} {
	t.Helper()
	assertCode(t, body, 0)
	if got := len(pageRecords(t, body)); got != wantRecords {
		t.Errorf("data.records len = %d, want %d", got, wantRecords)
	}
	data := pageData(t, body)
	for _, kv := range []struct{ key, want string }{
		{"total", wantTotal}, {"page", wantPage}, {"pageSize", wantPageSize},
	} {
		if got := data[kv.key]; got != kv.want {
			t.Errorf("data.%s = %v, want %q", kv.key, got, kv.want)
		}
	}
	return data
}

func doGet(t *testing.T, r *gin.Engine, path string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	return parseJSON(t, w.Body)
}

func TestPageEnvelope_ProductReviews(t *testing.T) {
	mock := &reviewMockClient{
		GetProductReviewsFn: func(ctx context.Context, in *reviewpb.GetProductReviewsRequest, opts ...grpc.CallOption) (*reviewpb.ProductReviewsResponse, error) {
			return &reviewpb.ProductReviewsResponse{
				Code:     0,
				Reviews:  []*reviewpb.ReviewInfo{{Id: "1", BuyerId: "2", Rating: 5}},
				Stats:    &reviewpb.ReviewStats{TotalCount: 9, AverageRating: 4.5},
				Total:    9,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}
	// BuyerID / Store 必須有值：#2897 之後 resolveOptionalBuyerID 在
	// loginRole 未定時會跨軸去問兩軸（舊行為是直接回 0、完全不碰 resolver），
	// 少了它們這裡會 nil pointer panic。用「查無 stint」的 resolver 讓本測試
	// 維持原本的匿名語意——它驗的是 envelope 形狀，不是身分解析。
	h := &Holder{
		Review:  grpcclient.NewReviewClientForTest(mock),
		BuyerID: noBuyerResolverForTest(),
		Store:   noStoreResolverForTest(),
	}
	body := doGet(t, setupReviewRouter(h), "/api/v1/public/products/42/reviews")
	data := assertPageEnvelope(t, body, 1, "9", "1", "20")

	stats, ok := data["stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.stats 必須存在; data = %v", data)
	}
	// stats 是自家 DTO（handler.ReviewStatsBlock），不是 proto struct 直出：
	// 欄名 camelCase、且**沒有 omitempty**，所以零評論商品的 0 值必須照樣出現在
	// wire 上（proto 直出時 `json:"total_count,omitempty"` 會把 0 整個吞掉）。
	for _, k := range []string{
		"totalCount", "averageRating", "rating5Count", "rating4Count",
		"rating3Count", "rating2Count", "rating1Count", "withImagesCount",
	} {
		if _, present := stats[k]; !present {
			t.Errorf("data.stats.%s 不得遺失; stats = %v", k, stats)
		}
	}
	for _, stale := range []string{
		"total_count", "average_rating", "rating_5_count", "rating_4_count",
		"rating_3_count", "rating_2_count", "rating_1_count", "with_images_count",
	} {
		if _, leaked := stats[stale]; leaked {
			t.Errorf("data.stats 不得再出現 snake key %q（10 檔 §2）", stale)
		}
	}
	if _, leaked := body["stats"]; leaked {
		t.Error("envelope 頂層不得出現 stats")
	}
}

// ============================================================================
// C8-b：order_review.go 兩支分頁端點的 snake 漂移（已全數收斂）
// ============================================================================
//
// 這兩支原本都是 handler 自己組 gin.H 而非走 response.Page，回應體吐 snake
// `page_size`（連 @Success 註解也照著寫）；其餘分頁端點吐的都是 camelCase
// `pageSize`，前端寫通用的 `data.pageSize` 讀這兩支只會拿到 undefined。
//
//   - GET /app/sellers/me/reviews —— 先行收斂（改走 response.Page）。
//   - GET /app/me/reviews —— 本批一起收斂。它曾被刻意留著，因為
//     hawkr-website `lib/reviews/repo.ts` 的型別宣告與 fallback 寫死讀
//     `d.page_size`，且 web 自己的單元測試 `tests/my-reviews-normalize.test.ts`
//     餵的是寫死 `page_size` 的 fixture 而非真打後端——後端單方面改名時 web 的
//     測試不會轉紅。所以它必須跟 web 同一批切，而這就是那一批。
//
// 兩支現在都走 assertPageEnvelope，並各自斷言舊 snake 名真的消失
// （只斷言新名存在的話，同時吐兩個 key 也會綠）。

func TestPageEnvelope_MyOrderReviews(t *testing.T) {
	mock := &reviewMockClient{
		ListMyOrderReviewsFn: func(ctx context.Context, in *reviewpb.ListMyOrderReviewsRequest, opts ...grpc.CallOption) (*reviewpb.ListMyOrderReviewsResponse, error) {
			return &reviewpb.ListMyOrderReviewsResponse{
				Code:     0,
				Records:  []*reviewpb.OrderReviewInfo{{Id: "1", Rating: 5, Content: "好"}},
				Total:    "1",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		OrderReview: grpcclient.NewOrderReviewClientForTest(mock),
		BuyerID:     buyerGrandfatherResolverForTest(),
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("accountId", int64(42)) })
	r.GET("/api/v1/app/me/reviews", h.ListMyOrderReviews)
	body := doGet(t, r, "/api/v1/app/me/reviews?page=1&size=20")
	assertPageEnvelope(t, body, 1, "1", "1", "20")
	if _, stale := pageData(t, body)["page_size"]; stale {
		t.Error("data.page_size 必須消失（已改走 response.Page）")
	}
}

// 已收斂：走 response.Page，與其餘分頁端點同形（data.pageSize）。
func TestPageEnvelope_SellerOrderReviews(t *testing.T) {
	mock := &reviewMockClient{
		GetSellerOrderReviewsFn: func(ctx context.Context, in *reviewpb.GetSellerOrderReviewsRequest, opts ...grpc.CallOption) (*reviewpb.GetSellerOrderReviewsResponse, error) {
			return &reviewpb.GetSellerOrderReviewsResponse{
				Code:     0,
				Records:  []*reviewpb.OrderReviewInfo{{Id: "1", Rating: 4, Content: "不錯"}},
				Total:    "1",
				Page:     "1",
				PageSize: "20",
			}, nil
		},
	}
	h := &Holder{
		OrderReview: grpcclient.NewOrderReviewClientForTest(mock),
		Store:       grandfatherResolverForTest(),
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("accountId", int64(42)) })
	r.GET("/api/v1/app/sellers/me/reviews", h.GetSellerOrderReviews)
	body := doGet(t, r, "/api/v1/app/sellers/me/reviews?page=1&size=20")
	assertPageEnvelope(t, body, 1, "1", "1", "20")
	// 收斂的重點是舊名真的消失，不是新名存在——只斷言 pageSize 的話，
	// 同時吐兩個 key 也會綠。
	if _, stale := pageData(t, body)["page_size"]; stale {
		t.Error("data.page_size 必須消失（已改走 response.Page）")
	}
}
