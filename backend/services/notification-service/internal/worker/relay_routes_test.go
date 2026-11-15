package worker

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/zap"
)

// 單體送出的每個 (exchange, routing key)，消費者這一側要成立：topology.yaml 有 queue 綁到它，而且 notification-service
// 不是接不到就是明白宣告不處理（notifyAllowlist）。單體是另一個 Go module，這裡不能 import 它，所以讀它的原始碼掃描結果
// 寫成的 manifest（掃描規則與失敗條件、重新產生的指令見單體 internal/legacy/rabbitmq/publish_routes_test.go）；
// 這裡不自己掃原始碼。每個 route 有沒有 template 要查 DB，那部分在 template_routes_integration_test.go。
// 欄位正確性不在這裡（那是各發送者的 wire_test）。終點：最後一個舊 Publisher 使用者刪除時刪掉。

const routesManifest = "../../../../internal/legacy/rabbitmq/testdata/routes.json"

type routeRow struct {
	Exchange string `json:"exchange"`
	Key      string `json:"routing_key"`
	Package  string `json:"package"`
}

func publishRoutes(t *testing.T) []routeRow {
	t.Helper()
	raw, err := os.ReadFile(routesManifest)
	if err != nil {
		t.Fatalf("讀不到單體的發送點 manifest %s：%v", routesManifest, err)
	}
	var rows []routeRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("解析 %s：%v", routesManifest, err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s 是空的", routesManifest)
	}
	return rows
}

// topicMatch 是 AMQP topic exchange 的比對：* 一個字，# 零個以上的字（單體 internal/legacy/rabbitmq/publish_routes_test.go 因為是另一個 module 有同一份，改這裡要一併改那裡）。
func topicMatch(pattern, key string) bool {
	var match func(p, k []string) bool
	match = func(p, k []string) bool {
		if len(p) == 0 {
			return len(k) == 0
		}
		if p[0] == "#" {
			for i := 0; i <= len(k); i++ {
				if match(p[1:], k[i:]) {
					return true
				}
			}
			return false
		}
		if len(k) == 0 || (p[0] != "*" && p[0] != k[0]) {
			return false
		}
		return match(p[1:], k[1:])
	}
	return match(strings.Split(pattern, "."), strings.Split(key, "."))
}

// notifyAllowlist 是「單體送得出來、notification-service 卻刻意不做通知」的 route。其餘每個 route 要有 enabled 的 template
// 或是靜態訂閱（staticSubscriptions）。每筆寫由哪個 queue 消費，過期的條目（不再送出、已有靜態訂閱、已有 template）會讓測試失敗。
type notifyAllowlistEntry struct{ exchange, key, consumerQueue, reason string }

var notifyAllowlist = []notifyAllowlistEntry{
	{"hawkr.seller", "seller.profile.changed", "cart.seller.profile.changed", "購物車依店家資料變更刷新，不通知任何人"},
	{"hawkr.seller", "seller.exposure.changed", "product.seller.exposure.changed", "商品依店家曝光狀態更新，不通知任何人"},
	{"hawkr.seller", "seller.suspended", "notification.seller", "mutation probe"},
	{"hawkr.buyer", "buyer.favorite.removed", "product.buyer.favorite.removed", "商品收藏數扣減，取消收藏不通知賣家"},
}

func staticHandles(exchange, key string) (mq.Subscription, bool) {
	for _, s := range staticSubscriptions(nil, zap.NewNop()) {
		if s.Exchange != exchange {
			continue
		}
		for _, k := range s.RoutingKeys {
			if topicMatch(k, key) {
				return s, true
			}
		}
	}
	return mq.Subscription{}, false
}

func TestPublishedRoutesAreBound(t *testing.T) {
	topo := loadTopology(t)
	for _, r := range publishRoutes(t) {
		if len(boundQueues(topo, r.Exchange, r.Key)) == 0 {
			t.Errorf("%s 送出 %s/%s，topology.yaml 沒有任何 queue 綁到它：這個事件送出去沒有人收", r.Package, r.Exchange, r.Key)
		}
	}
}

func boundQueues(topo mqTopology, exchange, key string) []string {
	var out []string
	for _, q := range topo.Queues {
		for _, b := range q.Bindings {
			if b.Exchange == exchange && topicMatch(b.Key, key) {
				out = append(out, q.Name)
				break
			}
		}
	}
	return out
}

func TestStaticSubscriptionsAreBound(t *testing.T) {
	topo := loadTopology(t)
	for _, s := range staticSubscriptions(nil, zap.NewNop()) {
		for _, k := range s.RoutingKeys {
			if !slices.Contains(boundQueues(topo, s.Exchange, k), s.Queue) {
				t.Errorf("靜態訂閱 %s 沒有被 topology.yaml 綁到 %s/%s", s.Queue, s.Exchange, k)
			}
		}
	}
}

func TestNotifyAllowlistIsCurrent(t *testing.T) {
	topo := loadTopology(t)
	emitted := map[string]bool{}
	for _, r := range publishRoutes(t) {
		emitted[r.Exchange+"/"+r.Key] = true
	}
	seen := map[string]bool{}
	for _, e := range notifyAllowlist {
		id := e.exchange + "/" + e.key
		if seen[id] {
			t.Errorf("allowlist 重複：%s", id)
		}
		seen[id] = true
		if e.reason == "" || e.consumerQueue == "" {
			t.Errorf("allowlist %s 缺原因或消費的 queue", id)
		}
		if !emitted[id] {
			t.Errorf("allowlist %s 已不是單體送出的 route：刪掉這筆", id)
		}
		if _, ok := staticHandles(e.exchange, e.key); ok {
			t.Errorf("allowlist %s 已有靜態訂閱處理：刪掉這筆", id)
		}
		if strings.HasPrefix(e.consumerQueue, "notification.") {
			t.Errorf("allowlist %s 的消費者不該是 notification 自己的 queue", id)
		}
		if !slices.Contains(boundQueues(topo, e.exchange, e.key), e.consumerQueue) {
			t.Errorf("allowlist %s 宣稱由 %s 消費，但 topology.yaml 沒有綁", id, e.consumerQueue)
		}
	}
}
