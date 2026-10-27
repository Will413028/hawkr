package mq

import (
	"os"
	"strings"
	"testing"
)

// broadcastQueueName 每次呼叫必須產生不同名字——runOnce 每次重連都會呼叫它。
// 若退回固定名，cluster 環境下 pod 重連到非 owner 節點時 declare 會撞 404
// NOT_FOUND（non-durable queue 只存在於建立它的節點），該 pod 的 SSE 永久不通。
// 2026-07-26 cutover 到 3-node cluster 後真實發生過，兩個 gateway pod 壞一個。
func TestBroadcastQueueName_UniquePerCall(t *testing.T) {
	const n = 50
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		name := broadcastQueueName()
		if _, dup := seen[name]; dup {
			t.Fatalf("broadcastQueueName 產生重複名稱 %q（第 %d 次呼叫）——"+
				"每次重連必須是新 queue，否則會撞到殘留在其他節點的同名 queue", name, i+1)
		}
		seen[name] = struct{}{}
	}
}

// 名稱要保留 pod hostname 前綴：management UI / 監控靠它對應到來源 pod，
// 且 check_unmanaged.py 以 "gateway.sse." 前綴將這類臨時 queue 排除在 drift 偵測外。
func TestBroadcastQueueName_KeepsHostPrefix(t *testing.T) {
	name := broadcastQueueName()

	if !strings.HasPrefix(name, "gateway.sse.") {
		t.Errorf("queue 名必須以 gateway.sse. 開頭（check_unmanaged.py 靠此前綴忽略臨時 queue），got %q", name)
	}

	host, err := os.Hostname()
	if err == nil && host != "" {
		if !strings.HasPrefix(name, "gateway.sse."+host+".") {
			t.Errorf("queue 名應為 gateway.sse.<hostname>.<id>，got %q（hostname=%q）", name, host)
		}
	}

	// 必須真的有 per-connection 後綴，不能只是 gateway.sse.<host>
	if parts := strings.Split(name, "."); len(parts) < 4 || parts[len(parts)-1] == "" {
		t.Errorf("queue 名缺少 per-connection 後綴，got %q", name)
	}
}
