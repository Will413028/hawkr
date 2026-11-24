package registry

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 每個型別在每個語系的固定輸出：改文案、改參數的格式或時間版面都會在這裡紅，要改就連預期一起改。
func TestGoldenRendering(t *testing.T) {
	cases := []struct {
		typ    string
		locale uitext.Locale
		params string
		want   Rendered
	}{
		{
			typ: "account_suspended", locale: uitext.ZhTW,
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","category":"SPAM","suspended_at":"2026-11-23T04:30:00Z"}`,
			want: Rendered{
				Title: "帳號已被停權", Body: "你的帳號已於 2026/11/23 12:30 因「垃圾訊息」被停權，部分功能受到限制。",
				Presentation: model.PresentationAccount, Locale: uitext.ZhTW,
			},
		},
		{
			typ: "account_suspended", locale: uitext.EN,
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","category":"SPAM","suspended_at":"2026-11-23T04:30:00Z"}`,
			want: Rendered{
				Title: "Your account has been suspended", Body: "Your account was suspended on 2026/11/23 12:30 for the following reason: Spam. Some features are restricted.",
				Presentation: model.PresentationAccount, Locale: uitext.EN,
			},
		},
		{
			typ: "account_suspended", locale: uitext.EN,
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","category":"FRAUD","suspended_at":"2026-11-23T04:30:00Z"}`,
			want: Rendered{
				Title: "Your account has been suspended", Body: "Your account was suspended on 2026/11/23 12:30 for the following reason: Fraud or transaction manipulation. Some features are restricted.",
				Presentation: model.PresentationAccount, Locale: uitext.EN,
			},
		},
		{
			// 目錄不認得的語系：整則用 zh-TW，Rendered.Locale 是文字真正的語系，不是請求的 "ja"。
			typ: "account_suspended", locale: uitext.Locale("ja"),
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","category":"FRAUD","suspended_at":"2026-11-23T04:30:00Z"}`,
			want: Rendered{
				Title: "帳號已被停權", Body: "你的帳號已於 2026/11/23 12:30 因「詐欺或操縱交易」被停權，部分功能受到限制。",
				Presentation: model.PresentationAccount, Locale: uitext.ZhTW,
			},
		},
		{
			typ: "account_unsuspended", locale: uitext.Locale("ja"),
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","released_at":"2026-11-24T16:05:00Z"}`,
			want: Rendered{
				Title: "帳號已解除停權", Body: "你的帳號已於 2026/11/25 00:05 解除停權，可以正常使用。",
				Presentation: model.PresentationAccount, Locale: uitext.ZhTW,
			},
		},
		{
			typ: "account_unsuspended", locale: uitext.ZhTW,
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","released_at":"2026-11-24T16:05:00Z"}`,
			want: Rendered{
				Title: "帳號已解除停權", Body: "你的帳號已於 2026/11/25 00:05 解除停權，可以正常使用。",
				Presentation: model.PresentationAccount, Locale: uitext.ZhTW,
			},
		},
		{
			typ: "account_unsuspended", locale: uitext.EN,
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","released_at":"2026-11-24T16:05:00Z"}`,
			want: Rendered{
				Title: "Your account suspension has been lifted", Body: "Your account suspension was lifted on 2026/11/25 00:05. You can use your account normally again.",
				Presentation: model.PresentationAccount, Locale: uitext.EN,
			},
		},
	}
	for _, c := range cases {
		e, ok := Lookup(c.typ)
		if !ok {
			t.Fatalf("沒有型別 %s", c.typ)
		}
		got, err := e.Render(c.locale, []byte(c.params))
		if err != nil {
			t.Errorf("%s/%s：%v", c.typ, c.locale, err)
			continue
		}
		if got.Title != c.want.Title || got.Body != c.want.Body || got.Locale != c.want.Locale || got.Presentation != c.want.Presentation ||
			got.DeepLink != "" || got.ImageKey != "" || got.AggregationKey != "" || len(got.ActorAccounts) != 0 || got.StateHint != model.StateHintNone {
			t.Errorf("%s/%s 的輸出\n got  %+v\n want %+v", c.typ, c.locale, got, c.want)
		}
	}
}

// 冪等鍵的固定輸出：事件來源身分變了，同一個事件會被當成新事件。
func TestGoldenIdempotencyKeys(t *testing.T) {
	const id = "0190a000-0000-7000-8000-000000000001"
	for typ, params := range map[string]string{
		"account_suspended":   `{"suspension_id":"` + id + `","category":"SPAM","suspended_at":"2026-11-23T04:30:00Z"}`,
		"account_unsuspended": `{"suspension_id":"` + id + `","released_at":"2026-11-23T04:30:00Z"}`,
	} {
		e, _ := Lookup(typ)
		got, err := e.IdempotencyKey([]byte(params))
		if err != nil || got != typ+":"+id {
			t.Errorf("%s 的冪等鍵 = %q（%v）", typ, got, err)
		}
	}
}

// en 發佈門檻（multi-locale ADR）：型別能組出的每個值在 en 都有文字，不退回中文。政策分類是 policy.Values() 的全部（停權可用任何一個）。
// 缺任何一個 en 文字，這個測試就紅；不在執行期偵測、也不退回 zh-TW。
func TestEveryRenderableValueHasEnglishText(t *testing.T) {
	han := regexp.MustCompile(`\p{Han}`)
	e, _ := Lookup("account_suspended")
	for _, c := range policy.Values() {
		raw, _ := json.Marshal(AccountSuspendedParams{SuspensionID: exampleSuspensionID, Category: c, SuspendedAt: exampleAt})
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			got, err := e.Render(l, raw)
			if err != nil {
				t.Errorf("%s/%s：%v", c, l, err)
				continue
			}
			if got.Locale != l {
				t.Errorf("%s/%s 實際用了 %s", c, l, got.Locale)
			}
			if l == uitext.EN && (han.MatchString(got.Title) || han.MatchString(got.Body)) {
				t.Errorf("政策分類 %s 的 en 通知含中文：%q", c, got.Body)
			}
		}
		if l, err := categoryLabel(uitext.EN, c); err != nil || l.Locale != uitext.EN {
			t.Errorf("categoryLabel(en, %s) = %+v（%v）", c, l, err)
		}
	}
	for label, entries := range wholeRegistries() {
		for _, e := range entries {
			if e.Spec().Broadcast {
				continue
			}
			raw, _, _ := e.ExampleParams()
			got, err := e.RenderLoaded(uitext.EN, raw, e.ExampleLoaded())
			if err != nil || got.Locale != uitext.EN || han.MatchString(got.Title+got.Body) {
				t.Errorf("%s：%s 的 en 範例：%+v（%v）", label, e.Spec().Name, got, err)
			}
		}
	}
}

// 公告：參數只有公告 ID，標題與內文來自 Load 解析出來的公告；任何請求語系都是同一份英文，Locale 是 EN。
func TestGoldenAnnouncementRendering(t *testing.T) {
	e, ok := Lookup("announcement")
	if !ok {
		t.Fatal("沒有型別 announcement")
	}
	raw := []byte(`{"announcement_id":"0190a000-0000-7000-8000-0000000000a1"}`)
	loaded := Loaded{Announcement: &AnnouncementText{Title: "Scheduled maintenance", Body: "Down on Sunday."}}
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN, uitext.Locale("ja")} {
		got, err := e.RenderLoaded(l, raw, loaded)
		if err != nil {
			t.Fatalf("%s：%v", l, err)
		}
		if got.Title != "Scheduled maintenance" || got.Body != "Down on Sunday." || got.Locale != uitext.EN ||
			got.Presentation != model.PresentationSystem || got.DeepLink != "" || got.AggregationKey != "" || len(got.ActorAccounts) != 0 {
			t.Errorf("%s 的輸出 %+v", l, got)
		}
	}
	if _, err := e.Render(uitext.EN, raw); err == nil {
		t.Error("沒有 Load 的結果就組字應該失敗，不該組出空文字")
	}
}

type announcementSource struct {
	texts map[uuid.UUID]AnnouncementText
	calls int
}

func (s *announcementSource) Announcements(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]AnnouncementText, error) {
	s.calls++
	out := map[uuid.UUID]AnnouncementText{}
	for _, id := range ids {
		if t, ok := s.texts[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

// Load 以一批參數裡的公告 ID 一次向 Sources 取文字：一次呼叫、結果同序、查不到的是零值、壞掉的快照只影響那一列；沒有 Load 的型別回零值。
func TestLoadResolvesTheAnnouncementsInOneCall(t *testing.T) {
	e, _ := Lookup("announcement")
	known := uuid.MustParse("0190a000-0000-7000-8000-0000000000a1")
	missing := uuid.MustParse("0190a000-0000-7000-8000-0000000000a2")
	src := &announcementSource{texts: map[uuid.UUID]AnnouncementText{known: {Title: "t", Body: "b"}}}
	raws := []json.RawMessage{
		[]byte(`{"announcement_id":"` + known.String() + `"}`),
		[]byte(`{"announcement_id":"` + missing.String() + `"}`),
		[]byte(`not json`),
	}
	ld, rowErrs, err := e.Load(context.Background(), src, raws)
	if err != nil || len(ld) != 3 || len(rowErrs) != 3 {
		t.Fatalf("Load = %+v %v（%v）", ld, rowErrs, err)
	}
	if src.calls != 1 {
		t.Errorf("Sources 被呼叫 %d 次，一批只該一次", src.calls)
	}
	if ld[0].Announcement == nil || ld[0].Announcement.Title != "t" || rowErrs[0] != nil {
		t.Errorf("第 0 筆 = %+v（%v）", ld[0], rowErrs[0])
	}
	if ld[1].Announcement != nil || rowErrs[1] != nil {
		t.Errorf("查不到的公告應是零值且不是列錯誤：%+v（%v）", ld[1], rowErrs[1])
	}
	if rowErrs[2] == nil {
		t.Error("壞掉的快照應是列錯誤")
	}
	plain, _ := Lookup("account_unsuspended")
	pl, pe, err := plain.Load(context.Background(), nil, []json.RawMessage{[]byte(`{}`)})
	if err != nil || len(pl) != 1 || pl[0] != (Loaded{}) || pe[0] != nil {
		t.Errorf("沒有 Load 的型別應回零值：%+v %v（%v）", pl, pe, err)
	}
}
