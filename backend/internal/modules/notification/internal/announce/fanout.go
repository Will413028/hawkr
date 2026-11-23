package announce

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
)

// ChunkSize 是一個派送 job 處理的收件人數上限。每個收件人自己一個 transaction（鎖順序要求先鎖帳號再鎖買家或店家，一頁合成一個 transaction 會交錯），
// 所以一個 job 的工作量由它界定。
const ChunkSize = 200

// InboxArgs 是收件匣派送的一頁：公告、受眾、與上一頁最後一個收件人的排序鍵（0 是從頭）。受眾逐個處理，一頁沒有收件人時換下一個受眾，都處理完就標記完成。
type InboxArgs struct {
	AnnouncementID uuid.UUID      `json:"announcement_id"`
	Audience       model.Audience `json:"audience"`
	After          int64          `json:"after"`
}

// Kind 是 River 的 job 種類。
func (InboxArgs) Kind() string { return "notification.announcement_inbox" }

// PushArgs 是推播派送的一頁，結構同 InboxArgs；它為每個沒有關掉公告推播的收件人寫一筆投遞紀錄並排渠道 job（不看有沒有 device token，之後的渠道 job 處理），
// 與收件匣分開所以任一邊失敗都不影響另一邊。
type PushArgs struct {
	AnnouncementID uuid.UUID      `json:"announcement_id"`
	Audience       model.Audience `json:"audience"`
	After          int64          `json:"after"`
}

// Kind 是 River 的 job 種類。
func (PushArgs) Kind() string { return "notification.announcement_push" }

type inboxWorker struct {
	river.WorkerDefaults[InboxArgs]
	s *Service
}

// Work 處理收件匣的一頁；失敗由 River 重試（每個收件人的寫入以 (公告, 收件人) 唯一，重試安全）。
func (w *inboxWorker) Work(ctx context.Context, job *river.Job[InboxArgs]) error {
	return w.s.ProcessInbox(ctx, job.Args)
}

// ProcessInbox 處理收件匣的一頁（worker 與測試共用）。
func (s *Service) ProcessInbox(ctx context.Context, a InboxArgs) error {
	return s.page(ctx, pageSpec{
		id: a.AnnouncementID, audience: a.Audience, after: a.After,
		write: s.writeInbox,
		nextPage: func(after int64) river.JobArgs {
			return InboxArgs{AnnouncementID: a.AnnouncementID, Audience: a.Audience, After: after}
		},
		nextAudience: func(aud model.Audience) river.JobArgs {
			return InboxArgs{AnnouncementID: a.AnnouncementID, Audience: aud}
		},
		markDone: func(ctx context.Context, q *store.Queries, now time.Time) error {
			return q.MarkAnnouncementInboxDone(ctx, store.MarkAnnouncementInboxDoneParams{ID: a.AnnouncementID, Now: &now})
		},
	})
}

type pushWorker struct {
	river.WorkerDefaults[PushArgs]
	s *Service
}

// Work 處理推播的一頁；重試安全（投遞紀錄以 (意圖, 收件人, 渠道) 唯一）。
func (w *pushWorker) Work(ctx context.Context, job *river.Job[PushArgs]) error {
	return w.s.ProcessPush(ctx, job.Args)
}

// ProcessPush 處理推播的一頁（worker 與測試共用）。
func (s *Service) ProcessPush(ctx context.Context, a PushArgs) error {
	return s.page(ctx, pageSpec{
		id: a.AnnouncementID, audience: a.Audience, after: a.After,
		write: s.writePush,
		nextPage: func(after int64) river.JobArgs {
			return PushArgs{AnnouncementID: a.AnnouncementID, Audience: a.Audience, After: after}
		},
		nextAudience: func(aud model.Audience) river.JobArgs {
			return PushArgs{AnnouncementID: a.AnnouncementID, Audience: aud}
		},
		markDone: func(ctx context.Context, q *store.Queries, now time.Time) error {
			return q.MarkAnnouncementPushDone(ctx, store.MarkAnnouncementPushDoneParams{ID: a.AnnouncementID, Now: &now})
		},
	})
}

// AddWorkers 登記兩條派送 job 鏈的 worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &inboxWorker{s: s})
	river.AddWorker(workers, &pushWorker{s: s})
}

// pageSpec 描述一條 job 鏈的一頁：做什麼（write）、下一頁與下一個受眾的 job、全部處理完時怎麼標記。
type pageSpec struct {
	id           uuid.UUID
	audience     model.Audience
	after        int64
	write        func(context.Context, store.NotificationAnnouncement, model.Audience, Recipient) error
	nextPage     func(after int64) river.JobArgs
	nextAudience func(model.Audience) river.JobArgs
	markDone     func(context.Context, *store.Queries, time.Time) error
}

// page 處理一頁：讀一頁收件人、逐人寫（各自一個 transaction），再在一個 transaction 內排下一頁；沒有收件人了就換下一個受眾，
// 受眾都處理完就標記完成。下一個 job 在寫完整頁之後才排，所以 job 中途失敗重跑，都從同一頁重來，已寫的收件人被唯一鍵略過。
func (s *Service) page(ctx context.Context, p pageSpec) error {
	ann, err := store.New(s.d.DB.Conn(ctx)).AnnouncementByID(ctx, p.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return river.JobCancel(fmt.Errorf("公告 %s 不存在", p.id))
	}
	if err != nil {
		return fmt.Errorf("讀取公告：%w", err)
	}
	recipients, err := s.recipients(ctx, p.audience, p.after)
	if err != nil {
		return err
	}
	for _, r := range recipients {
		if err := p.write(ctx, ann, p.audience, r); err != nil {
			return err
		}
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if len(recipients) > 0 {
			return s.d.Jobs.Enqueue(ctx, p.nextPage(recipients[len(recipients)-1].Cursor), uniqueByArgs)
		}
		if i := slices.Index(ann.Audiences, string(p.audience)); i >= 0 && i+1 < len(ann.Audiences) {
			return s.d.Jobs.Enqueue(ctx, p.nextAudience(model.Audience(ann.Audiences[i+1])), uniqueByArgs)
		}
		return p.markDone(ctx, store.New(s.d.DB.Conn(ctx)), s.d.Now())
	})
}

func (s *Service) recipients(ctx context.Context, aud model.Audience, after int64) ([]Recipient, error) {
	switch aud {
	case model.AudienceBuyer:
		return s.d.Directory.Buyers(ctx, after, s.d.Chunk)
	case model.AudienceStore:
		return s.d.Directory.Stores(ctx, after, s.d.Chunk)
	default:
		return nil, river.JobCancel(fmt.Errorf("公告受眾 %q 不合法", aud))
	}
}

func storeID(r Recipient) *int64 {
	if r.StoreID > 0 {
		return &r.StoreID
	}
	return nil
}

// writeInbox 把公告寫進一個收件人的收件匣。收件匣不可關（D18），所以不看偏好，也不看有沒有 device token。收件人已刪除、已關閉或已退場時略過。
func (s *Service) writeInbox(ctx context.Context, ann store.NotificationAnnouncement, aud model.Audience, r Recipient) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		gone, err := intake.LockRecipient(ctx, s.d.Locks, aud, r.AccountID, r.StoreID)
		if err != nil || gone != intake.GoneNone {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		n, err := q.InsertAnnouncementInbox(ctx, store.InsertAnnouncementInboxParams{
			ID: s.d.NewID(), AccountID: r.AccountID, Audience: string(aud), StoreID: storeID(r),
			OccurredAt: ann.CreatedAt, Now: s.d.Now(), AnnouncementID: ann.ID,
		})
		if err != nil {
			return fmt.Errorf("寫入公告的收件匣列：%w", err)
		}
		if n == 0 {
			return nil
		}
		if err := q.AddAnnouncementInboxCount(ctx, store.AddAnnouncementInboxCountParams{ID: ann.ID, Delta: int32(n)}); err != nil { //nolint:gosec // 一次只寫一列
			return fmt.Errorf("遞增公告的收件匣人數：%w", err)
		}
		return nil
	})
}

// writePush 為一個收件人寫公告的推播投遞紀錄並排渠道 job；收件人在偏好裡關掉公告推播（D18）時什麼都不寫。紀錄已存在（重試）時不重複排 job。
func (s *Service) writePush(ctx context.Context, ann store.NotificationAnnouncement, aud model.Audience, r Recipient) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		gone, err := intake.LockRecipient(ctx, s.d.Locks, aud, r.AccountID, r.StoreID)
		if err != nil || gone != intake.GoneNone {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		overrides, err := q.PreferenceOverrides(ctx, store.PreferenceOverridesParams{
			AccountID: r.AccountID, Audience: string(aud), Category: string(model.CategoryAnnouncement),
		})
		if err != nil {
			return fmt.Errorf("讀取偏好：%w", err)
		}
		byChannel := make(map[model.Channel]bool, len(overrides))
		for _, o := range overrides {
			byChannel[model.Channel(o.Channel)] = o.Enabled
		}
		if !model.Enabled(aud, model.CategoryAnnouncement, model.ChannelPush, byChannel) {
			return nil
		}
		deliveryID, err := q.InsertDelivery(ctx, store.InsertDeliveryParams{
			ID: s.d.NewID(), IntentID: ann.ID, AccountID: r.AccountID, Audience: string(aud), StoreID: storeID(r),
			Channel: string(model.ChannelPush), Now: s.d.Now(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("寫入公告的投遞紀錄：%w", err)
		}
		if err := s.d.Jobs.Enqueue(ctx, intake.DeliverArgs{DeliveryID: deliveryID}, uniqueByArgs); err != nil {
			return fmt.Errorf("排入渠道 job：%w", err)
		}
		return nil
	})
}
