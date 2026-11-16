package worker

import "testing"

// 給 package worker_test 的整合測試用：發送點 manifest、allowlist 與訂閱資訊都在 package worker 的測試檔裡。

type PublishRoute struct{ Pkg, Exchange, Key string }

type NotifyAllowlistEntry struct{ Exchange, Key, ConsumerQueue, Reason string }

func MonolithPublishRoutes(t *testing.T) []PublishRoute {
	t.Helper()
	var out []PublishRoute
	for _, r := range publishRoutes(t) {
		out = append(out, PublishRoute{r.Package, r.Exchange, r.Key})
	}
	return out
}

func NotifyAllowlist() []NotifyAllowlistEntry {
	var out []NotifyAllowlistEntry
	for _, e := range notifyAllowlist {
		out = append(out, NotifyAllowlistEntry{e.exchange, e.key, e.consumerQueue, e.reason})
	}
	return out
}

// StaticHandles 回傳處理 (exchange, key) 的靜態訂閱的 queue。
func StaticHandles(exchange, key string) (string, bool) {
	s, ok := staticHandles(exchange, key)
	return s.Queue, ok
}

// DynamicQueue 回傳 template 驅動的訂閱對這個 exchange 用的 queue。
func DynamicQueue(exchange string) (string, bool) {
	q, ok := notificationQueueByExchange[exchange]
	return q, ok
}

// TopologyBound 回傳 topology.yaml 裡綁到 (exchange, key) 的 queue 名稱。
func TopologyBound(t *testing.T, exchange, key string) []string {
	t.Helper()
	return boundQueues(loadTopology(t), exchange, key)
}
