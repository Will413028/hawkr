package hashtag

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// social 與 catalog 共用 platform/hashtag 的同一份規則：這裡鎖住 social 沒有自己的副本。
func TestSocialUsesThePlatformRule(t *testing.T) {
	for _, raw := range []string{"Fruit", "  ##Org ", "有機", "", "#"} {
		n1, d1, ok1 := Normalize(raw)
		n2, d2, ok2 := hashtag.Normalize(raw)
		if n1 != n2 || d1 != d2 || ok1 != ok2 {
			t.Errorf("Normalize(%q): social %q %q %v, platform %q %q %v", raw, n1, d1, ok1, n2, d2, ok2)
		}
	}
	got, err := RequireSet(uitext.ZhTW, []string{"#Fruit", "FRUIT", "水果"})
	want, _ := hashtag.RequireSet([]string{"#Fruit", "FRUIT", "水果"})
	if err != nil || len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("%+v %v want %+v", got, err, want)
	}
}
