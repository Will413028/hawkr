package imrelay

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const goldenFile = "../../../tests/contract/golden/event_payload_keys.json"

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

// golden 的 im-service 項目只有 chat.message.sent（通知模板讀的 key）；有新項目要補案例。
func TestGoldenIMServiceEntriesAreAllCovered(t *testing.T) {
	var names []string
	for name, e := range loadGolden(t) {
		if e.Publisher == "im-service" {
			names = append(names, name)
		}
	}
	if len(names) != 1 || names[0] != "chat.message.sent" {
		t.Fatalf("golden 的 im-service 項目 = %v，relay 只有 chat.message.sent 的案例", names)
	}
}

// golden 宣稱 chat.message.sent 送出的 key 與 payload 的 key 完全相同，而且買家寄與店家寄兩個方向的值都不是空的：模板讀到空字串的通知是壞的
// （sender_avatar 例外：沒有頭像的寄件人是空字串，舊碼同樣）。
func TestGoldenEmittedKeysEqualPayloadKeysWithValues(t *testing.T) {
	entry := loadGolden(t)["chat.message.sent"]
	if entry.RoutingKey != keySent || len(entry.EmittedKeys) != 10 {
		t.Fatalf("golden chat.message.sent = %+v", entry)
	}
	for _, sender := range []string{"buyer", "store"} {
		f := newFixture()
		if sender == "store" {
			f.messages.msg.SenderKind = "STORE"
		}
		if err := f.worker.messageSent(t.Context(), sentArgs()); err != nil {
			t.Fatal(err)
		}
		pubs := f.jobs.published()
		if len(pubs) != 1 {
			t.Fatalf("%s：送出 %d 則", sender, len(pubs))
		}
		m := body(t, pubs[0])
		keys := slices.Sorted(maps.Keys(m))
		if !slices.Equal(keys, slices.Sorted(slices.Values(entry.EmittedKeys))) {
			t.Errorf("%s：payload keys %v 與 golden %v 不同", sender, keys, entry.EmittedKeys)
		}
		for _, key := range entry.EmittedKeys {
			if v := m[key]; v == nil || v == "" {
				t.Errorf("%s：golden 的 %q 在 payload 是空值 %#v", sender, key, v)
			}
		}
	}
}
