// golden_contract_test.go — publisher-side 反向契約測試（13 號設計書 §14.6）。
//
// notification-service 的 TestTemplateContract 只驗一個方向：「template 讀的 key ⊆
// golden emitted_keys」。反過來沒有人驗——golden 宣稱某個 key 會被 emit，但 publisher
// 的 struct 根本沒有那個欄位時，兩邊都是綠的，而通知渲染出來少一個變數（#1529 的
// action_label 即此，靜默 0 投遞）。
//
// 本檔補上另一個方向：golden emitted_keys ⊆ 本服務 payload struct 實際序列化出來的 key。
// 用生產同一組 struct ＋ 同一個 encoding/json，所以改 json tag、刪欄位、改大小寫都會轉紅。
// Java 側同型前例：services/java/shipping-service/.../ShippingPayloadGoldenContractTest.java。
//
// 為什麼不改 contract-tests.yml 的 paths：那支 workflow 的輸入只有 template、golden 與
// consumer 自己的 template_handler.go，從頭到尾沒有讀過任何 publisher 原始碼，加 paths
// 是 5 倍觸發成本換零偵測力。本檔是普通 Go 測試，由 go-checks.yml（`**/*.go`）帶跑。
package mq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

// samplePayloads maps 每個由本服務 publish 的 golden event_type 到一個「所有欄位都填了
// 非零值」的樣本。填滿是必要的：omitempty 欄位留空會從序列化結果消失，那時本測試會對
// 著一份殘缺的 key 集合做比對，反而放行 golden 的錯誤宣稱。
//
// chat.message.sent 的 wire 就是 usecase.ChatMessageSentEvent 的直接序列化
// （chat_event_publisher.go 把整個 struct 交給 pkgmq.Publisher.Publish，無 envelope 包裝）。
func samplePayloads() map[string]any {
	return map[string]any{
		"chat.message.sent": usecase.ChatMessageSentEvent{
			MessageID:      "7100000000000001",
			ConversationID: "7100000000000002",
			SenderID:       "2685914682",
			SenderName:     "範例鮮果",
			SenderAvatar:   "s/avatar.jpg",
			ReceiverID:     "2623900520",
			ReceiverType:   "buyer",
			StoreID:        "2900000001",
			ContentPreview: "您好，商品已出貨",
			SentAt:         "2026-01-01T00:00:00Z",
		},
	}
}

func TestGoldenEmittedKeysAreSubsetOfPublishedPayload(t *testing.T) {
	assertGoldenSubset(t, samplePayloads())
}

// ── 以下是讀 golden 的共用機制。每個 publisher module 各持一份：它們是彼此獨立的 Go
// module，共用只能靠把它抽進 pkg/，而那會讓一個純測試 helper 成為所有服務的生產依賴。
// Java 側同樣是各 module 一份（shipping 的 locateGolden、aftersales 的 GoldenPayloadKeys）。

type goldenEntry struct {
	Publisher   string   `json:"publisher"`
	RoutingKey  string   `json:"routing_key"`
	EmittedKeys []string `json:"emitted_keys"`
}

// assertGoldenSubset 對每個樣本斷言 golden 宣稱的 key 都真的序列化得出來。
// 樣本對不到 golden 條目也是失敗——golden 改名或條目被刪時，靜默少驗比驗錯更糟。
func assertGoldenSubset(t *testing.T, samples map[string]any) {
	t.Helper()
	golden := loadGolden(t)
	if len(samples) == 0 {
		t.Fatal("samplePayloads 是空的——本測試會變成永遠通過的空殼")
	}

	eventTypes := make([]string, 0, len(samples))
	for eventType := range samples {
		eventTypes = append(eventTypes, eventType)
	}
	sort.Strings(eventTypes)

	for _, eventType := range eventTypes {
		entry, ok := golden[eventType]
		if !ok {
			t.Errorf("golden 找不到 event_type %q —— 本測試預期覆蓋它，條目被改名或刪除時必須同步改這裡", eventType)
			continue
		}
		if len(entry.EmittedKeys) == 0 {
			t.Errorf("%s: golden 的 emitted_keys 是空的，斷言等於沒有", eventType)
			continue
		}
		actual := payloadKeys(t, samples[eventType])
		for _, k := range entry.EmittedKeys {
			if !actual[k] {
				t.Errorf("%s (%s): golden 宣稱 emit %q，但 %T 序列化後的 key 只有 %v",
					eventType, entry.RoutingKey, k, samples[eventType], sortedKeys(actual))
			}
		}
	}
}

// payloadKeys 走生產同一條路徑（encoding/json.Marshal）取頂層 key 集合。
func payloadKeys(t *testing.T, payload any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %T: %v", payload, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %T: %v", payload, err)
	}
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// loadGolden 從測試 cwd 逐層往上找 repo root 的 golden 檔。用往上找而不是寫死
// ../../../..：各 module 的測試檔深度不同，寫死的相對層數在檔案搬家時會變成一個
// 「找不到檔案」的紅字，而不是它真正要守的契約。
func loadGolden(t *testing.T) map[string]goldenEntry {
	t.Helper()
	const rel = "tests/contract/golden/event_payload_keys.json"

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if _, statErr := os.Stat(path); statErr == nil {
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			var g map[string]goldenEntry
			if err := json.Unmarshal(b, &g); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}
			if len(g) == 0 {
				t.Fatalf("%s 解析出空 map", path)
			}
			return g
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("找不到 %s（從測試 cwd 逐層往上找到檔案系統根目錄）", rel)
		}
		dir = parent
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
