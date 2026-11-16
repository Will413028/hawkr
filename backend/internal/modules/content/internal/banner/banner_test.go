package banner

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

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
		{model.BannerAction{Type: model.BannerActionPost, Target: "018f0000-0000-7000-8000-000000000001"}, true},
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

// fakeSocial 以固定的集合回答「公開可讀」，並記下被問的次數。
type fakeSocial struct {
	readable map[uuid.UUID]bool
	err      error
	calls    int
}

func (f *fakeSocial) PubliclyReadablePosts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if f.readable[id] {
			out[id] = true
		}
	}
	return out, nil
}

var (
	postPublic = uuid.UUID{0x01, 0x8f, 0, 0, 0, 0, 0x70, 0, 0x80, 0, 0, 0, 0, 0, 0, 0x01}
	postGone   = uuid.UUID{0x01, 0x8f, 0, 0, 0, 0, 0x70, 0, 0x80, 0, 0, 0, 0, 0, 0, 0x02}
)

func postBanner(id uuid.UUID) model.Banner {
	return model.Banner{ID: id, Action: model.BannerAction{Type: model.BannerActionPost, Target: postTarget(id)}}
}

func postTarget(id uuid.UUID) string { return id.String() }

// CreateBanner 的貼文目標檢查在碰 DB 之前：Service 沒有 DB 時，被拒的建立仍回得出錯誤。
func TestCreateBannerRejectsAPostTargetThatIsNotPubliclyReadable(t *testing.T) {
	soc := &fakeSocial{readable: map[uuid.UUID]bool{postPublic: true}}
	svc := New(Deps{Social: soc})
	in := model.BannerInput{Title: "t", PreviewImageToken: "p", PopupImageToken: "q"}
	for name, target := range map[string]string{
		"不存在或不公開（草稿、隱藏、下架、已刪除都在 social 回不可讀）": postTarget(postGone),
		"不是 UUID（舊的 TSID 格式）":                 "456",
	} {
		in.Action = model.BannerAction{Type: model.BannerActionPost, Target: target}
		_, err := svc.CreateBanner(context.Background(), 1, in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 InvalidInput", name, err)
		}
	}
}

func TestCreateBannerDoesNotSwallowASocialFailure(t *testing.T) {
	boom := errors.New("boom")
	svc := New(Deps{Social: &fakeSocial{err: boom}})
	in := model.BannerInput{Title: "t", PreviewImageToken: "p", PopupImageToken: "q",
		Action: model.BannerAction{Type: model.BannerActionPost, Target: postTarget(postPublic)}}
	if _, err := svc.CreateBanner(context.Background(), 1, in); !errors.Is(err, boom) {
		t.Errorf("err = %v，預期包著 social 的錯誤", err)
	}
}

func TestPublicReadHidesBannersWhosePostIsNoLongerReadable(t *testing.T) {
	soc := &fakeSocial{readable: map[uuid.UUID]bool{postPublic: true}}
	svc := New(Deps{Social: soc})
	product := model.Banner{Action: model.BannerAction{Type: model.BannerActionProduct, Target: "42"}}
	legacy := model.Banner{Action: model.BannerAction{Type: model.BannerActionPost, Target: "456"}}
	got, err := svc.dropUnreadablePostTargets(context.Background(), []model.Banner{postBanner(postPublic), postBanner(postGone), product, legacy})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != postPublic || got[1].Action.Type != model.BannerActionProduct {
		t.Errorf("留下 %+v，預期只剩公開貼文的 banner 與商品 banner", got)
	}
	if soc.calls != 1 {
		t.Errorf("問了 social %d 次，預期一次（批次）", soc.calls)
	}
}

func TestPublicReadAsksSocialNothingWhenNoBannerTargetsAPost(t *testing.T) {
	soc := &fakeSocial{}
	svc := New(Deps{Social: soc})
	if _, err := svc.dropUnreadablePostTargets(context.Background(), []model.Banner{{Action: model.BannerAction{Type: model.BannerActionPopup}}}); err != nil {
		t.Fatal(err)
	}
	if soc.calls != 0 {
		t.Errorf("問了 social %d 次，預期 0", soc.calls)
	}
}

func TestPostBannerTargetIgnoresSurroundingWhitespaceInAStoredTarget(t *testing.T) {
	id := uuid.New()
	got, ok := postBannerTarget(model.Banner{Action: model.BannerAction{Type: model.BannerActionPost, Target: " " + id.String() + "\t"}})
	if !ok || got != id {
		t.Fatalf("postBannerTarget = %v, %v，預期 %v", got, ok, id)
	}
	if _, ok := postBannerTarget(model.Banner{Action: model.BannerAction{Type: model.BannerActionPost, Target: "not-a-uuid"}}); ok {
		t.Fatal("非 UUID 的目標不是貼文目標")
	}
}
