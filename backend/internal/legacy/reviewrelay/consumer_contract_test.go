package reviewrelay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/review"
)

const statsConsumer = "../../../services/java/product-service/src/main/java/io/github/will413028/hawkr/product/mq/ReviewStatsConsumer.java"

// Java product 的 ReviewStatsConsumer 是 review.created 與 review.hidden 的消費者：product_id 缺會被當成壞 payload 直接 ACK
// （統計不更新），product_avg_rating、product_review_count 缺或不是 JSON 數字會進 DLQ。這裡先確認 Java 原始碼仍讀這三個 key，
// 再確認兩個事件在「有評價」與「零評價」兩種情況都送出它們，而且型別與值能讓 consumer 的判斷通過。
func TestProductStatsConsumerContract(t *testing.T) {
	src, err := os.ReadFile(filepath.Clean(statsConsumer))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"product_id"`, `"product_avg_rating"`, `"product_review_count"`} {
		if !strings.Contains(string(src), key) {
			t.Fatalf("ReviewStatsConsumer 不再讀 %s：契約變了，更新這個測試與 payload", key)
		}
	}
	check := func(t *testing.T, m map[string]any, wantRating, wantCount float64) {
		t.Helper()
		if id, ok := m["product_id"].(string); !ok || id != product700.String() {
			t.Errorf("product_id = %#v，預期商品的 UUID 字串", m["product_id"])
		}
		if r, ok := m["product_avg_rating"].(float64); !ok || r != wantRating {
			t.Errorf("product_avg_rating = %#v，預期數字 %v", m["product_avg_rating"], wantRating)
		}
		if c, ok := m["product_review_count"].(float64); !ok || c != wantCount {
			t.Errorf("product_review_count = %#v，預期數字 %v", m["product_review_count"], wantCount)
		}
	}
	t.Run("with reviews", func(t *testing.T) {
		w, _ := newFixture()
		check(t, body(t, run(t, w, OnReviewCreated, created())[0]), 4.9, 30)
		check(t, body(t, run(t, w, OnReviewAggregateChanged, review.ReviewAggregateChanged{EventID: eventID, OccurredAt: occurredAt, StoreID: 91, ProductID: product700})[0]), 4.9, 30)
	})
	t.Run("no visible reviews", func(t *testing.T) {
		w, r := newFixture()
		r.product = map[uuid.UUID]review.ProductStats{}
		check(t, body(t, run(t, w, OnReviewAggregateChanged, review.ReviewAggregateChanged{EventID: eventID, OccurredAt: occurredAt, StoreID: 91, ProductID: product700})[0]), 0, 0)
	})
}
