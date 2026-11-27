package arch

import (
	"os"
	"regexp"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/lockorder"
)

func queryText(t *testing.T, path, name string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)-- name: ` + name + ` :.*?(?:\n-- name: |\z)`).FindString(string(raw))
	if m == "" {
		t.Fatalf("%s 裡找不到查詢 %s", path, name)
	}
	return regexp.MustCompile(`\n-- name: $`).ReplaceAllString(m, "")
}

// TestGuardQueriesClassifyAsOwnerRowLocks 守住 buyer 與 seller 的守門查詢是列鎖：lockorder 檢查器靠分類才看得到它們。
// 有人把它改成 advisory lock、或拿掉 FOR KEY SHARE，檢查器就不再看見守門在順序裡的位置。
func TestGuardQueriesClassifyAsOwnerRowLocks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		path, name string
		want       lockorder.Lock
	}{
		{"../modules/buyer/db/queries/buyers.sql", "LockBuyerShared", lockorder.Lock{Schema: "buyer", Table: "buyers", Kind: "FOR KEY SHARE", Waits: true}},
		{"../modules/seller/db/queries/stores.sql", "LockStoreKeyShare", lockorder.Lock{Schema: "seller", Table: "stores", Kind: "FOR KEY SHARE", Waits: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := lockorder.Classify(queryText(t, c.path, c.name))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0] != c.want {
				t.Fatalf("Classify(%s) = %+v，預期 [%+v]", c.name, got, c.want)
			}
		})
	}
}
