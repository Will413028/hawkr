package mq

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSubscriptionMaxRetriesMatchesTopology 對拍 §6.3.17 五組成對登記中**唯一沒有 CI gate**
// 的那一組：Go `Subscription.MaxRetries` ↔ `topology.yaml` 的 `retry.max_attempts`。
//
// 為什麼它值得一支測試：不一致不會編譯錯、不會讓任何既有測試變紅。`pkg/mq` 的 framework
// 在 `MaxRetries > 0` 時會 passive 檢查 `{queue}.retry` exchange 存在，而那個 exchange
// **只由 topology 的 `retry:` policy 產生** ⇒ 少了它，本機綠、CI 綠、上線整條 subscription
// 被 skip，服務健康、`/metrics` 乾淨、DLQ 空的，而發票一張都不會開。
//
// 🔴 先斷言「找到的 consumer 數 > 0」：這類「逐項比對清單」的測試最常見的死法是清單為空
// 而恆綠（路徑打錯、YAML 格式變了、Subscriptions 回空片段，三者都會讓它靜靜通過）。
func TestSubscriptionMaxRetriesMatchesTopology(t *testing.T) {
	subs := Subscriptions(
		NewOrderPaidConsumer(nil, nil, nil, nil, nil, nil, nil),
		NewRefundFinalizedConsumer(nil, nil, nil, nil),
		NewOrderCancelledConsumer(nil, nil, nil, nil),
	)
	if len(subs) == 0 {
		t.Fatal("Subscriptions() 回空片段：這支測試會恆綠，先修 Subscriptions")
	}

	topology := loadTopologyRetries(t)
	if len(topology) == 0 {
		t.Fatal("topology.yaml 解析出 0 條 queue：解析器或路徑壞了，這支測試會恆綠")
	}

	checked := 0
	for _, sub := range subs {
		attempts, ok := topology[sub.Queue]
		if !ok {
			t.Errorf("queue %q 沒有登記在 infra/mq/topology/topology.yaml —— "+
				"passive declare 查不到 queue 不 crash、不進 DLQ，該訂閱會靜默失效", sub.Queue)
			continue
		}
		if sub.MaxRetries != attempts {
			t.Errorf("queue %q：Subscription.MaxRetries=%d 但 topology 的 retry.max_attempts=%d —— "+
				"兩者必須相同（58 §6.3.17）", sub.Queue, sub.MaxRetries, attempts)
		}
		checked++
	}
	if checked != len(subs) {
		t.Fatalf("只對拍了 %d/%d 條 subscription", checked, len(subs))
	}
}

// loadTopologyRetries 從 topology.yaml 的 `queues:` 區塊解出 {queue 名: retry.max_attempts}。
//
// 只認帶 `retry:` 的 queue —— 沒有 `retry:` 的 queue 本來就不該配 `MaxRetries > 0`，
// 而「找不到」與「max_attempts 不同」在上面是兩種不同的失敗訊息。
//
// 刻意不引入 YAML library：它現在是 go.mod 的 indirect 依賴，為了一支測試把它拉成
// direct 會讓 `go mod tidy` 的輸出跟著動。本檔只需要辨識三種縮排層級固定的行。
func loadTopologyRetries(t *testing.T) map[string]int {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "infra", "mq", "topology", "topology.yaml")
	f, err := os.Open(path) //nolint:gosec // 固定的 repo 內路徑
	if err != nil {
		t.Fatalf("開 topology.yaml（%s）：%v", path, err)
	}
	defer func() { _ = f.Close() }()

	out := map[string]int{}
	var currentQueue string
	inRetry := false
	inQueues := false

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "queues:":
			inQueues = true
			continue
		case len(line) > 0 && line[0] != ' ' && line[0] != '-' && strings.HasSuffix(line, ":"):
			// 另一個頂層區塊（exchanges: / bindings: …）開始了。
			inQueues = false
			currentQueue, inRetry = "", false
			continue
		}
		if !inQueues {
			continue
		}

		if name, ok := strings.CutPrefix(line, "- name: "); ok {
			currentQueue = strings.TrimSpace(name)
			inRetry = false
			continue
		}
		if strings.TrimRight(line, " ") == "  retry:" {
			inRetry = true
			continue
		}
		// 任何回到 2 空格縮排的 key（bindings: / arguments: / dlq:）都離開 retry 區塊。
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			inRetry = false
			continue
		}
		if inRetry && currentQueue != "" {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "max_attempts:"); ok {
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil {
					t.Fatalf("queue %s 的 max_attempts 解析失敗：%q", currentQueue, line)
				}
				out[currentQueue] = n
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("讀 topology.yaml：%v", err)
	}
	return out
}

// TestTopologyParserFindsKnownQueues 是上一支測試的**鑑別力**驗證。
//
// 解析器若因為 YAML 格式改變而回空 map，TestSubscriptionMaxRetriesMatchesTopology 會在
// 「找不到 queue」那一支報錯——但如果有人把那條 `t.Errorf` 改成 `continue`，整支就恆綠了。
// 這一支直接釘住「解析器認得出三條發票 queue，而且值是 3」。
func TestTopologyParserFindsKnownQueues(t *testing.T) {
	got := loadTopologyRetries(t)
	for _, q := range []string{"invoice.order.paid", "invoice.order.refund.finalized", "invoice.order.cancelled"} {
		v, ok := got[q]
		if !ok {
			t.Fatalf("解析器沒認出 %q", q)
		}
		if v != ConsumerMaxRetries {
			t.Fatalf("%q 的 retry.max_attempts = %d，want %d", q, v, ConsumerMaxRetries)
		}
	}
	// 不相關的 queue 也要認得出來，證明解析器沒有只對 invoice 開特例。
	if _, ok := got["notification.invoice"]; !ok {
		t.Error("解析器沒認出 notification.invoice —— 它同樣帶 retry:")
	}
}
