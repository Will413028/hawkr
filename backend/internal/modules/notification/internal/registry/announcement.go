package registry

import (
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// AnnouncementParams 是 announcement 的參數快照：只有公告 ID，標題與內文讀取時由 Sources 解析（公告在 announcements 表，英文單一份）。
type AnnouncementParams struct {
	AnnouncementID uuid.UUID `json:"announcement_id"`
}

// 公告發給兩個受眾，收件人由公告的派送 job 展開（Fanout），所以沒有冪等鍵與收件人；渠道是收件匣與推播，沒有 email。
// 文字只有英文，不依請求語系，所以請求 zh-TW 也是同一份英文。
var (
	exampleAnnouncementID = uuid.MustParse("0190a000-0000-7000-8000-0000000000a1")
	announcement          = Define(Definition[AnnouncementParams]{
		Name:         "announcement",
		Audiences:    model.Audiences(),
		Fanout:       true,
		FixedLocale:  uitext.EN,
		Category:     model.CategoryAnnouncement,
		Presentation: model.PresentationSystem,
		Channels:     []model.Channel{model.ChannelInbox, model.ChannelPush},
		Load: func(ctx context.Context, src Sources, ps []AnnouncementParams) ([]Loaded, error) {
			ids := make([]uuid.UUID, len(ps))
			for i, p := range ps {
				ids[i] = p.AnnouncementID
			}
			found, err := src.Announcements(ctx, ids)
			if err != nil {
				return nil, err
			}
			out := make([]Loaded, len(ps))
			for i, id := range ids {
				if a, ok := found[id]; ok {
					out[i] = Loaded{Announcement: &a}
				}
			}
			return out, nil
		},
		TextLoaded: func(_ uitext.Locale, _ AnnouncementParams, ld Loaded) (Content, error) {
			if ld.Announcement == nil {
				return Content{}, fmt.Errorf("公告的文字沒有解析")
			}
			return Content{
				Title: uitext.Message{Text: ld.Announcement.Title, Locale: uitext.EN},
				Body:  uitext.Message{Text: ld.Announcement.Body, Locale: uitext.EN},
			}, nil
		},
		Example:       AnnouncementParams{AnnouncementID: exampleAnnouncementID},
		ExampleLoaded: Loaded{Announcement: &AnnouncementText{Title: "Scheduled maintenance", Body: "The service will be unavailable on Sunday from 02:00 to 03:00."}},
	})
)
