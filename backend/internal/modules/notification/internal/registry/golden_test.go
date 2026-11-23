package registry

import (
	"encoding/json"
	"regexp"
	"testing"

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
	for _, e := range All() {
		raw, _, _ := e.ExampleParams()
		got, err := e.Render(uitext.EN, raw)
		if err != nil || got.Locale != uitext.EN || han.MatchString(got.Title+got.Body) {
			t.Errorf("%s 的 en 範例：%+v（%v）", e.Spec().Name, got, err)
		}
	}
}
