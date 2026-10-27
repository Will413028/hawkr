package commerce

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func samplePayloads() map[string]any {
	return map[string]any{
		"procurement.rfq_publication_invite.created.seller": inviteCreatedPayload{
			InviteID:      "7502175358066622464",
			PublicationID: "7502175358066622465",
			StoreID:       "7502175358066622466",
		},
		"procurement.rfq_publication_invite.external_created.vendor": inviteExternalCreatedPayload{
			InviteID:      "7502175358066622467",
			PublicationID: "7502175358066622465",
			ProspectID:    "7502175358066622468",
			ExpiresAt:     "2026-10-11T06:00:00Z",
		},
	}
}

func TestGoldenEmittedKeysAreSubsetOfPublishedPayload(t *testing.T) {
	golden := loadGolden(t)
	samples := samplePayloads()
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
		if entry.Publisher != "ai-service" {
			t.Errorf("%s: golden 的 publisher 是 %q，不是 ai-service", eventType, entry.Publisher)
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

type goldenEntry struct {
	Publisher   string   `json:"publisher"`
	RoutingKey  string   `json:"routing_key"`
	EmittedKeys []string `json:"emitted_keys"`
}

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
