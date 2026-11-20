package reviewrelay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/review"
)

const (
	goldenFile    = "../../../tests/contract/golden/event_payload_keys.json"
	statsConsumer = "../../../services/java/product-service/src/main/java/io/github/will413028/hawkr/product/mq/ReviewStatsConsumer.java"
)

type goldenEntry struct {
	Publisher   string   `json:"publisher"`
	RoutingKey  string   `json:"routing_key"`
	EmittedKeys []string `json:"emitted_keys"`
}

func loadGolden(t *testing.T) map[string]goldenEntry {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(goldenFile))
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]goldenEntry
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

// golden 的 review-service 項目只有 review.created（通知模板讀的 key）：notification-service 的契約測試要求 golden 的每一項都有模板
// 消費它，review.hidden 沒有模板，所以它的消費者契約（Java product）由下面的測試對著 Java 原始碼驗，不放進 golden。
func TestGoldenReviewServiceEntriesAreAllCovered(t *testing.T) {
	var names []string
	for name, e := range loadGolden(t) {
		if e.Publisher == "review-service" {
			names = append(names, name)
		}
	}
	if len(names) != 1 || names[0] != "review.created" {
		t.Fatalf("golden 的 review-service 項目 = %v，relay 只有 review.created 的案例；有新項目要補案例", names)
	}
}

// golden 宣稱 review.created 送出的 key 都要在 payload 裡，而且值不是空的：模板讀到空字串或 0 的通知是壞的（review_id 例外：
// 設計書 §9 第 2 項決定送 0，四端都不讀推播的 reviewId）。
func TestGoldenEmittedKeysAreSubsetOfRelayedPayloadWithValues(t *testing.T) {
	entry := loadGolden(t)["review.created"]
	if entry.RoutingKey != "review.created" || len(entry.EmittedKeys) == 0 {
		t.Fatalf("golden review.created = %+v", entry)
	}
	w, _, _, _ := newFixture()
	got := run(t, w, OnReviewCreated, created())
	if len(got) != 1 {
		t.Fatalf("messages=%d", len(got))
	}
	m := body(t, got[0])
	for _, key := range entry.EmittedKeys {
		v, ok := m[key]
		if !ok {
			t.Errorf("golden 宣稱送出 %q，payload 沒有：%v", key, m)
			continue
		}
		if key == "review_id" {
			continue
		}
		if v == nil || v == "" || v == float64(0) {
			t.Errorf("golden 的 %q 在 payload 是空值 %#v，模板會渲染出壞通知", key, v)
		}
	}
}

// Java product 的 ReviewStatsConsumer 是 review.created 與 review.hidden 的消費者：product_id 缺或 0 會被當成壞 payload 直接 ACK
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
		if id, ok := m["product_id"].(float64); !ok || id != 700 {
			t.Errorf("product_id = %#v，consumer 要非 0 的數字", m["product_id"])
		}
		if r, ok := m["product_avg_rating"].(float64); !ok || r != wantRating {
			t.Errorf("product_avg_rating = %#v，預期數字 %v", m["product_avg_rating"], wantRating)
		}
		if c, ok := m["product_review_count"].(float64); !ok || c != wantCount {
			t.Errorf("product_review_count = %#v，預期數字 %v", m["product_review_count"], wantCount)
		}
	}
	t.Run("with reviews", func(t *testing.T) {
		w, _, _, _ := newFixture()
		check(t, body(t, run(t, w, OnReviewCreated, created())[0]), 4.9, 30)
		check(t, body(t, run(t, w, OnReviewAggregateChanged, review.ReviewAggregateChanged{EventID: eventID, OccurredAt: occurredAt, StoreID: 91, ProductID: 700})[0]), 4.9, 30)
	})
	t.Run("no visible reviews", func(t *testing.T) {
		w, r, _, _ := newFixture()
		r.product = map[int64]review.ProductStats{}
		check(t, body(t, run(t, w, OnReviewAggregateChanged, review.ReviewAggregateChanged{EventID: eventID, OccurredAt: occurredAt, StoreID: 91, ProductID: 700})[0]), 0, 0)
	})
}
