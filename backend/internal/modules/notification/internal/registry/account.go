package registry

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// AccountSuspendedParams 是 account_suspended 的參數快照，欄位取自 accountsuspension.Suspended。
type AccountSuspendedParams struct {
	AccountID    int64           `json:"account_id"`
	SuspensionID uuid.UUID       `json:"suspension_id"`
	Category     policy.Category `json:"category"`
	SuspendedAt  time.Time       `json:"suspended_at"`
}

// AccountUnsuspendedParams 是 account_unsuspended 的參數快照，欄位取自 accountsuspension.Unsuspended。
type AccountUnsuspendedParams struct {
	AccountID    int64     `json:"account_id"`
	SuspensionID uuid.UUID `json:"suspension_id"`
	ReleasedAt   time.Time `json:"released_at"`
}

// 只有無店的帳號能被停權（accountsuspension 的 requireNoStore），所以受眾是 BUYER。同 transaction 會刪 token（A8-3），
// 渠道是收件匣與 email，沒有推播。受限登入仍可讀收件匣。沒有深連結（沒有可去的畫面）。
var (
	exampleSuspensionID = uuid.MustParse("0190a000-0000-7000-8000-000000000001")
	exampleAt           = time.Date(2026, 11, 23, 4, 30, 0, 0, time.UTC)
)

var accountSuspended = Define(Definition[AccountSuspendedParams]{
	Name:              "account_suspended",
	Audience:          model.AudienceBuyer,
	Category:          model.CategoryAccount,
	Presentation:      model.PresentationAccount,
	Channels:          []model.Channel{model.ChannelInbox, model.ChannelEmail},
	IdempotencySource: "accountsuspension.Suspended.SuspensionID",
	IdempotencyKey:    func(p AccountSuspendedParams) string { return p.SuspensionID.String() },
	OccurredAt:        func(p AccountSuspendedParams) time.Time { return p.SuspendedAt },
	Recipient:         func(p AccountSuspendedParams) Recipient { return Recipient{AccountID: p.AccountID} },
	Text: func(l uitext.Locale, p AccountSuspendedParams) (Content, error) {
		reason, err := categoryLabel(l, p.Category)
		if err != nil {
			return Content{}, err
		}
		return Content{
			Title:     text.AccountSuspendedTitle(l),
			Body:      text.AccountSuspendedBody(l, text.AccountSuspendedBodyArgs{Reason: reason.Text, Time: uitext.FormatDateTime(l, p.SuspendedAt)}),
			Fragments: []uitext.Message{reason},
		}, nil
	},
	Example: AccountSuspendedParams{AccountID: 42, SuspensionID: exampleSuspensionID, Category: policy.Spam, SuspendedAt: exampleAt},
})

var accountUnsuspended = Define(Definition[AccountUnsuspendedParams]{
	Name:              "account_unsuspended",
	Audience:          model.AudienceBuyer,
	Category:          model.CategoryAccount,
	Presentation:      model.PresentationAccount,
	Channels:          []model.Channel{model.ChannelInbox, model.ChannelEmail},
	IdempotencySource: "accountsuspension.Unsuspended.SuspensionID",
	IdempotencyKey:    func(p AccountUnsuspendedParams) string { return p.SuspensionID.String() },
	OccurredAt:        func(p AccountUnsuspendedParams) time.Time { return p.ReleasedAt },
	Recipient:         func(p AccountUnsuspendedParams) Recipient { return Recipient{AccountID: p.AccountID} },
	Text: func(l uitext.Locale, p AccountUnsuspendedParams) (Content, error) {
		return Content{
			Title: text.AccountUnsuspendedTitle(l),
			Body:  text.AccountUnsuspendedBody(l, text.AccountUnsuspendedBodyArgs{Time: uitext.FormatDateTime(l, p.ReleasedAt)}),
		}, nil
	},
	Example: AccountUnsuspendedParams{AccountID: 42, SuspensionID: exampleSuspensionID, ReleasedAt: exampleAt},
})
