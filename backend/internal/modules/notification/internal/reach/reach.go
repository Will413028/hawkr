// Package reach 是投遞紀錄的後台操作：重送永久失敗的推播與 email，以及未觸達查詢。
// 投遞紀錄是推播與 email 觸達與否的唯一來源（收件匣不是渠道）；兩個操作都只讀寫它。
package reach

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/deliver"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

const (
	DefaultMaxCount = 100
	MaxMaxCount     = 500
	// MaxResendWindow 是重送失敗時間窗的長度上限。
	MaxResendWindow = 90 * 24 * time.Hour

	DefaultDays = 7
	MaxDays     = 90

	DefaultPageSize int32 = 20
	MaxPageSize     int32 = 50
)

var resendable = []string{
	deliver.ReasonNoDeviceToken, deliver.ReasonTokenGone, deliver.ReasonPushRejected,
	deliver.ReasonNoEmailAddress, deliver.ReasonEmailRejected, deliver.ReasonRetriesExhausted,
}

var notReached = []string{deliver.ReasonNoLongerValid, deliver.ReasonRecipientGone}

// Resendable 回傳可重送的失敗原因（每次回傳新的切片）。
func Resendable() []string { return slices.Clone(resendable) }

// Registry 是型別註冊表。
type Registry interface {
	All() []registry.Entry
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB       *db.DB
	Audit    *adminaudit.Log
	Jobs     jobs.Enqueuer
	Registry Registry
	Now      func() time.Time
	NewID    func() uuid.UUID
}

// Service 是重送與未觸達查詢。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

// ResendInput 是重送的條件。FailStart、FailEnd 是失敗時間窗 [FailStart, FailEnd)，兩端必填；Category 為空、Channel 為空、Reasons 為空、AccountID 為 0 都表示不限。
type ResendInput struct {
	Actor     adminaudit.Actor
	FailStart time.Time
	FailEnd   time.Time
	Category  model.PrefCategory
	Channel   model.Channel
	Reasons   []string
	AccountID int64
	MaxCount  int
	DryRun    bool
}

// ResendResult 是重送的結果：Count 是重排的筆數（DryRun 時是會重排的筆數）；HasMore 表示還有符合條件但因 MaxCount 沒處理的投遞。
type ResendResult struct {
	Count   int
	HasMore bool
}

type resendFilter struct {
	reasons   []string
	channel   *string
	accountID *int64
	types     []string
	start     time.Time
	end       time.Time
}

func (s *Service) resendFilter(in ResendInput) (resendFilter, int, error) {
	now := s.d.Now()
	f := resendFilter{start: in.FailStart.UTC(), end: in.FailEnd.UTC()}
	maxCount := in.MaxCount
	switch {
	case in.FailStart.IsZero():
		return f, 0, apperr.InvalidInputOn([]string{"FailTimeStart"}, "需要失敗時間窗的起點")
	case in.FailEnd.IsZero():
		return f, 0, apperr.InvalidInputOn([]string{"FailTimeEnd"}, "需要失敗時間窗的終點")
	case !in.FailStart.Before(in.FailEnd):
		return f, 0, apperr.InvalidInputOn([]string{"FailTimeStart", "FailTimeEnd"}, "失敗時間窗的起點必須早於終點")
	case in.FailEnd.After(now):
		return f, 0, apperr.InvalidInputOn([]string{"FailTimeEnd"}, "失敗時間窗的終點不可在未來")
	case in.FailEnd.Sub(in.FailStart) > MaxResendWindow:
		return f, 0, apperr.InvalidInputOn([]string{"FailTimeStart", "FailTimeEnd"}, "失敗時間窗不可超過 %d 天", int(MaxResendWindow/(24*time.Hour)))
	case in.AccountID < 0:
		return f, 0, apperr.InvalidInputOn([]string{"AccountID"}, "收件人帳號不可為負數")
	case maxCount < 0:
		return f, 0, apperr.InvalidInputOn([]string{"MaxCount"}, "筆數不可為負數")
	}
	if maxCount == 0 {
		maxCount = DefaultMaxCount
	}
	maxCount = min(maxCount, MaxMaxCount)
	if in.Channel != "" {
		if !model.DeliveryChannel(in.Channel) {
			return f, 0, apperr.InvalidInputOn([]string{"Channel"}, "渠道只能是推播或 email")
		}
		ch := string(in.Channel)
		f.channel = &ch
	}
	f.reasons = resendable
	if len(in.Reasons) > 0 {
		for _, r := range in.Reasons {
			if !slices.Contains(resendable, r) {
				return f, 0, apperr.InvalidInputOn([]string{"Reasons"}, "失敗原因 %q 不可重送", r)
			}
		}
		f.reasons = in.Reasons
	}
	if in.AccountID > 0 {
		f.accountID = &in.AccountID
	}
	if in.Category != "" {
		if !slices.Contains(model.Categories(), in.Category) {
			return f, 0, apperr.InvalidInputOn([]string{"Category"}, "不認得的通知分類")
		}
		f.types = []string{}
		for _, e := range s.d.Registry.All() {
			if e.Spec().Category == in.Category {
				f.types = append(f.types, e.Spec().Name)
			}
		}
	}
	return f, maxCount, nil
}

// Resend 把符合條件、永久失敗且原因可重送的投遞改回待送並排入渠道 job（同一個 transaction），一次最多 MaxCount 筆，依失敗時間由舊到新。
// 只動永久失敗的列，所以已送出或已重排的投遞不會再被選中；不可重送的原因（Resendable 以外）指定時回 InvalidInput（Reasons）。
// 排的 job 與原本的 job 不同鍵（DeliverArgs.RequeuedAt），送出時仍走渠道 job 的全部檢查（收件人在不在、停寄名單、來源事實、靜音時段）。
// DryRun 只計數，不改資料、不寫稽核紀錄。
func (s *Service) Resend(ctx context.Context, in ResendInput) (ResendResult, error) {
	f, maxCount, err := s.resendFilter(in)
	if err != nil {
		return ResendResult{}, err
	}
	count := func(ctx context.Context, limit int) (int, error) {
		n, err := store.New(s.d.DB.Conn(ctx)).CountRequeueableDeliveries(ctx, store.CountRequeueableDeliveriesParams{
			Reasons: f.reasons, StartTime: f.start, EndTime: f.end, Channel: f.channel, AccountID: f.accountID, Types: f.types, MaxRows: int32(limit), //nolint:gosec // limit 最多 MaxMaxCount+1
		})
		if err != nil {
			return 0, fmt.Errorf("計算可重送的投遞：%w", err)
		}
		return int(n), nil
	}
	if in.DryRun {
		n, err := count(ctx, maxCount+1)
		if err != nil {
			return ResendResult{}, err
		}
		return ResendResult{Count: min(n, maxCount), HasMore: n > maxCount}, nil
	}
	var out ResendResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Now().UTC().Truncate(time.Microsecond)
		ids, err := store.New(s.d.DB.Conn(ctx)).RequeueFailedDeliveries(ctx, store.RequeueFailedDeliveriesParams{
			Reasons: f.reasons, StartTime: f.start, EndTime: f.end, Channel: f.channel, AccountID: f.accountID, Types: f.types, MaxRows: int32(maxCount), Now: now, //nolint:gosec // maxCount 最多 MaxMaxCount
		})
		if err != nil {
			return fmt.Errorf("重排永久失敗的投遞：%w", err)
		}
		for _, id := range ids {
			if err := s.d.Jobs.Enqueue(ctx, intake.DeliverArgs{DeliveryID: id, RequeuedAt: now.UnixMicro()}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}); err != nil {
				return fmt.Errorf("排入渠道 job：%w", err)
			}
		}
		out.Count = len(ids)
		if len(ids) == maxCount {
			more, err := count(ctx, 1)
			if err != nil {
				return err
			}
			out.HasMore = more > 0
		}
		return s.d.Audit.Record(ctx, adminaudit.Entry{
			Actor: in.Actor, TargetType: "delivery_resend", TargetID: s.d.NewID().String(),
			Detail: map[string]any{
				"count": out.Count, "has_more": out.HasMore, "fail_start": f.start, "fail_end": f.end,
				"category": string(in.Category), "channel": string(in.Channel), "reasons": f.reasons, "account_id": in.AccountID,
			},
		})
	})
	if err != nil {
		return ResendResult{}, err
	}
	return out, nil
}

// UnreachedInput 是未觸達查詢的條件：Days 是往回看的天數（0 用預設）；After 是上一頁的游標。
type UnreachedInput struct {
	Days     int
	After    *Cursor
	PageSize int32
}

// Cursor 是未觸達列表的排序鍵：(CreatedAt, DeliveryID) 由大到小。
type Cursor struct {
	CreatedAt  time.Time `json:"created_at"`
	DeliveryID uuid.UUID `json:"delivery_id"`
}

// Valid 回報游標是否完整。
func (c Cursor) Valid() bool { return !c.CreatedAt.IsZero() && c.DeliveryID != uuid.UUID{} }

// Channel 是一個渠道的失敗結果。
type Channel struct {
	Channel model.Channel
	Reason  string
}

// Notification 是一個收件人沒有觸達的一則通知；只有參考 ID，沒有文字、位址與 token。
type Notification struct {
	DeliveryID uuid.UUID
	IntentID   uuid.UUID
	Type       string
	Category   model.PrefCategory
	Audience   model.Audience
	AccountID  int64
	StoreID    int64
	CreatedAt  time.Time
	Channels   []Channel
}

// Page 是一頁未觸達的通知；Count 是窗內的總筆數；Next 為 nil 表示沒有下一頁。
type Page struct {
	Count         int64
	Notifications []Notification
	Next          *Cursor
}

// TransactionalTypes 回傳交易必要分類（使用者不能關閉的分類）的型別名稱，由註冊表推導。
func (s *Service) TransactionalTypes() []string {
	var out []string
	for _, e := range s.d.Registry.All() {
		if class, ok := model.ClassOf(e.Spec().Category); ok && class == model.ClassTransactional {
			out = append(out, e.Spec().Name)
		}
	}
	return out
}

// Unreached 回傳近 Days 天交易必要型別的通知中，某個收件人所有投遞都永久失敗（沒有任何推播或 email 送出）的筆數與明細，新到舊。
// 只看投遞紀錄：收件匣不是渠道，寫進收件匣不算觸達；還在待送的、原因是刻意不送或收件人已刪除的不算；沒有投遞紀錄的意圖不算。
func (s *Service) Unreached(ctx context.Context, in UnreachedInput) (Page, error) {
	days := in.Days
	switch {
	case days < 0:
		return Page{}, apperr.InvalidInputOn([]string{"Days"}, "天數不可為負數")
	case days == 0:
		days = DefaultDays
	}
	days = min(days, MaxDays)
	size, err := pagination.PageSize(in.PageSize, DefaultPageSize, MaxPageSize)
	if err != nil {
		return Page{}, err
	}
	types := s.TransactionalTypes()
	since := s.d.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	q := store.New(s.d.DB.Conn(ctx))
	total, err := q.CountUnreachedNotifications(ctx, store.CountUnreachedNotificationsParams{Since: since, Types: types, ExcludedReasons: notReached})
	if err != nil {
		return Page{}, fmt.Errorf("計算未觸達的通知：%w", err)
	}
	p := store.ListUnreachedNotificationsParams{Since: since, Types: types, ExcludedReasons: notReached, PageSize: size + 1}
	if in.After != nil {
		p.AfterCreatedAt, p.AfterID = &in.After.CreatedAt, &in.After.DeliveryID
	}
	rows, err := q.ListUnreachedNotifications(ctx, p)
	if err != nil {
		return Page{}, fmt.Errorf("列出未觸達的通知：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := Page{Count: total, Notifications: make([]Notification, len(rows))}
	byType := make(map[string]model.PrefCategory)
	for _, e := range s.d.Registry.All() {
		byType[e.Spec().Name] = e.Spec().Category
	}
	for i, r := range rows {
		n := Notification{
			DeliveryID: r.RefID, IntentID: r.IntentID, Type: r.Type, Category: byType[r.Type], Audience: model.Audience(r.Audience),
			AccountID: r.AccountID, CreatedAt: r.IntentCreatedAt, Channels: make([]Channel, len(r.Channels)),
		}
		if r.StoreID != nil {
			n.StoreID = *r.StoreID
		}
		for j := range r.Channels {
			n.Channels[j] = Channel{Channel: model.Channel(r.Channels[j]), Reason: r.Reasons[j]}
		}
		out.Notifications[i] = n
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &Cursor{CreatedAt: last.IntentCreatedAt, DeliveryID: last.RefID}
	}
	return out, nil
}
