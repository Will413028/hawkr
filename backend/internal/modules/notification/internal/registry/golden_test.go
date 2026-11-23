package registry

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
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
			// en 目錄沒有這個分類的文字：整則退回 zh-TW，不夾中文理由。
			typ: "account_suspended", locale: uitext.EN,
			params: `{"suspension_id":"0190a000-0000-7000-8000-000000000001","category":"FRAUD","suspended_at":"2026-11-23T04:30:00Z"}`,
			want: Rendered{
				Title: "帳號已被停權", Body: "你的帳號已於 2026/11/23 12:30 因「詐欺或操縱交易」被停權，部分功能受到限制。",
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
