package registry

import (
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type deliveryParams struct{ ID int64 }

func deliveryType(name string, a model.Audience, c model.PrefCategory, channels ...model.Channel) Entry {
	return Define(Definition[deliveryParams]{
		Name: name, Audiences: []model.Audience{a}, Category: c, Presentation: model.PresentationMessage, Channels: channels,
		IdempotencySource: "test.Event.ID",
		IdempotencyKey:    func(p deliveryParams) string { return "x" },
		OccurredAt:        func(deliveryParams) time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
		Recipient:         func(p deliveryParams) Recipient { return Recipient{AccountID: p.ID} },
		Text: func(l uitext.Locale, _ deliveryParams) (Content, error) {
			return Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
		},
		Example: deliveryParams{ID: 1},
	})
}

// 偏好 API 只列註冊表實際送達的組合：沒有型別走 email 的分類沒有 email；沒有型別的受眾與分類什麼都沒有。
func TestDeliveredFollowsTheRegisteredTypes(t *testing.T) {
	entries := []Entry{
		deliveryType("chat_message", model.AudienceBuyer, model.CategoryChat, model.ChannelInbox, model.ChannelPush),
		deliveryType("account_note", model.AudienceBuyer, model.CategoryAccount, model.ChannelInbox, model.ChannelEmail),
		announcement,
	}
	for _, c := range []struct {
		a    model.Audience
		cat  model.PrefCategory
		ch   model.Channel
		want bool
	}{
		{model.AudienceBuyer, model.CategoryChat, model.ChannelInbox, true},
		{model.AudienceBuyer, model.CategoryChat, model.ChannelPush, true},
		{model.AudienceBuyer, model.CategoryChat, model.ChannelEmail, false},
		{model.AudienceStore, model.CategoryChat, model.ChannelInbox, false},
		{model.AudienceBuyer, model.CategoryLike, model.ChannelInbox, false},
		{model.AudienceBuyer, model.CategoryAccount, model.ChannelEmail, true},
		{model.AudienceBuyer, model.CategoryAccount, model.ChannelPush, false},
		{model.AudienceBuyer, model.CategoryPayout, model.ChannelInbox, false},
		{model.AudienceBuyer, model.CategoryAnnouncement, model.ChannelInbox, true},
		{model.AudienceStore, model.CategoryAnnouncement, model.ChannelPush, true},
		{model.AudienceBuyer, model.CategoryAnnouncement, model.ChannelEmail, false},
	} {
		if got := Delivered(entries, c.a, c.cat, c.ch); got != c.want {
			t.Errorf("Delivered(%s, %s, %s) = %v，預期 %v", c.a, c.cat, c.ch, got, c.want)
		}
	}
	bad := []Entry{deliveryType("bad", model.AudienceBuyer, model.CategoryAnnouncement, model.ChannelEmail)}
	if Delivered(bad, model.AudienceBuyer, model.CategoryAnnouncement, model.ChannelEmail) {
		t.Error("公告不寄 email，型別宣告了也不送達")
	}
}

func TestTypeNamesFiltersByAudienceAndCategory(t *testing.T) {
	entries := []Entry{
		deliveryType("a", model.AudienceBuyer, model.CategoryChat, model.ChannelInbox),
		deliveryType("b", model.AudienceBuyer, model.CategoryLike, model.ChannelInbox),
		deliveryType("c", model.AudienceStore, model.CategoryChat, model.ChannelInbox),
	}
	chat := model.CategoryChat
	for _, c := range []struct {
		a    model.Audience
		cat  *model.PrefCategory
		want []string
	}{
		{model.AudienceBuyer, nil, []string{"a", "b"}},
		{model.AudienceBuyer, &chat, []string{"a"}},
		{model.AudienceStore, &chat, []string{"c"}},
		{model.AudienceStore, nil, []string{"c"}},
	} {
		if got := TypeNames(entries, c.a, c.cat); !slices.Equal(got, c.want) {
			t.Errorf("TypeNames(%s, %v) = %v，預期 %v", c.a, c.cat, got, c.want)
		}
	}
	if got := TypeNames(nil, model.AudienceBuyer, nil); got == nil {
		t.Error("沒有型別時要回空切片（SQL 的 ANY 參數不接受 NULL 以外的 nil 切片語意）")
	}
}
