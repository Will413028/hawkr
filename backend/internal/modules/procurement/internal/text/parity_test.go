package text

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

type sourceFile struct {
	Default map[string]json.RawMessage `json:"default"`
}

func defaultKeys(t *testing.T, name string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var f sourceFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	keys := make(map[string]bool, len(f.Default))
	for k := range f.Default {
		keys[k] = true
	}
	return keys
}

// 預設受眾的每個 key 在 en 都要有文字；staff 受眾只有 zh-TW（ADR 2026-10-17 D1），不在比對範圍。
func TestEnglishCoversEveryDefaultAudienceKey(t *testing.T) {
	zh := defaultKeys(t, "zh-TW.json")
	en := defaultKeys(t, "en.json")
	var missing []string
	for k := range zh {
		if !en[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("en.json 缺預設受眾的 key：%v", missing)
	}
}
