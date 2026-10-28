package banner

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestBannerActionMustBeComplete(t *testing.T) {
	cases := []struct {
		action model.BannerAction
		ok     bool
	}{
		{model.BannerAction{Type: model.BannerActionPopup}, true},
		{model.BannerAction{Type: model.BannerActionURL, Target: "https://example.com/promo"}, true},
		{model.BannerAction{Type: model.BannerActionURL, Target: "http://example.com"}, true},
		{model.BannerAction{Type: model.BannerActionProduct, Target: "123"}, true},
		{model.BannerAction{Type: model.BannerActionPost, Target: "456"}, true},
		{model.BannerAction{}, false},                            // 被洗成 {} 的那種
		{model.BannerAction{Type: model.BannerActionURL}, false}, // 選了 url 沒填網址
		{model.BannerAction{Type: model.BannerActionURL, Target: " "}, false},
		// 只有 http、https 可以開啟，後台不能對 App 觸發任意 scheme。
		{model.BannerAction{Type: model.BannerActionURL, Target: "javascript:alert(1)"}, false},
		{model.BannerAction{Type: model.BannerActionURL, Target: "hawkr://order/1"}, false},
		{model.BannerAction{Type: model.BannerActionURL, Target: "https://"}, false},
		{model.BannerAction{Type: model.BannerActionProduct}, false}, // 選了商品沒填 id
		{model.BannerAction{Type: "coupon", Target: "1"}, false},
	}
	for _, c := range cases {
		err := validateAction(c.action)
		if (err == nil) != c.ok {
			t.Errorf("%+v：err = %v，預期 ok = %v", c.action, err, c.ok)
		}
		if err != nil && apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%+v：錯誤的分類是 %v", c.action, apperr.KindOf(err))
		}
	}
}

func TestPopupDropsALeftoverTarget(t *testing.T) {
	if got := normalizeAction(model.BannerAction{Type: model.BannerActionPopup, Target: "表單殘留"}); got != (model.BannerAction{Type: model.BannerActionPopup}) {
		t.Fatalf("normalized = %+v", got)
	}
	url := model.BannerAction{Type: model.BannerActionURL, Target: "https://example.com"}
	if got := normalizeAction(url); got != url {
		t.Fatalf("normalized = %+v", got)
	}
}

func TestBannerInputRejectsAnInvertedSchedule(t *testing.T) {
	start := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	end := start.Add(-time.Hour)
	in := model.BannerInput{Title: "中秋", Action: model.BannerAction{Type: model.BannerActionPopup}, StartsAt: &start, EndsAt: &end}
	if err := validateInput(in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err = %v", err)
	}
}

// 按鈕文字的上限來自後台 banner 編輯器（12 字）；其他文字欄位不設上限。
func TestBannerButtonTextIsLimitedTo12Characters(t *testing.T) {
	in := model.BannerInput{Title: "中秋", Action: model.BannerAction{Type: model.BannerActionPopup}, ButtonText: "一二三四五六七八九十一二"}
	if err := validateInput(in); err != nil {
		t.Fatalf("12 個字應通過：%v", err)
	}
	in.ButtonText += "三"
	if err := validateInput(in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("13 個字應回 InvalidInput：%v", err)
	}
}
