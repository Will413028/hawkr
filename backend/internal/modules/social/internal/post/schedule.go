package post

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/schedule"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// PublishScheduledArgs 是排程到點的 job：只帶 schedule_id，唯一的真相是 post_schedules 的列。排程列一次一個、不重用，所以 schedule_id 是冪等鍵：
// 唯一鍵涵蓋排隊中、執行中與重試中的狀態、不含已完成的；到點的執行讀排程列，已經不是 PENDING 就什麼都不做，所以重複的 job（重試）無害。
type PublishScheduledArgs struct {
	ScheduleID uuid.UUID `json:"schedule_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (PublishScheduledArgs) Kind() string { return "social.publish_scheduled" }

// InsertOpts 見 PublishScheduledArgs。
func (PublishScheduledArgs) InsertOpts() river.InsertOpts {
	return jobs.UniqueByArgs(10)
}

// ValidateAuthorizers 確認每一種發起者都有授權實作；缺一種就回錯誤（social.New 在啟動時 panic）。
func ValidateAuthorizers(a map[model.ScheduleKind]model.ScheduleAuthorizer) error {
	for _, k := range model.ScheduleKindValues() {
		if a[k] == nil {
			return fmt.Errorf("social：排程發起者 %s 沒有授權實作", k)
		}
	}
	return nil
}

// ownerAuthorizer 是店主排程的授權：與手動發布同一個店家授權（seller.Authorize 的 CapMarketing）。只有「沒有這項權利」（PermissionDenied）與「店家不存在」（NotFound）是永久的拒絕；
// FailedPrecondition 與其他錯誤是暫時的，回傳讓 job 重試。
// social 自己的檢查（沒刪除、沒下架、仍滿足公開要件）對每一種發起者都在到點時先做（RunScheduled）。
type ownerAuthorizer struct{ stores readcache.Stores }

func (a ownerAuthorizer) AuthorizeScheduledPublish(ctx context.Context, p model.ScheduledPublish) error {
	st, err := a.stores.Store(ctx, p.StoreID)
	if err == nil {
		err = seller.Authorize(st, seller.CapMarketing)
	}
	if k := apperr.KindOf(err); k == apperr.KindNotFound || k == apperr.KindPermissionDenied {
		return fmt.Errorf("%w：%w", model.ErrScheduleDenied, err)
	}
	return err
}

// OwnerAuthorizer 回傳店主排程的授權實作；stores 是讀店家的 seller 公開 API（不鎖列）。
func OwnerAuthorizer(stores readcache.Stores) model.ScheduleAuthorizer {
	return ownerAuthorizer{stores: stores}
}

func (s *Service) hasPendingSchedule(ctx context.Context, q *store.Queries, postID uuid.UUID) (bool, error) {
	_, err := q.GetPendingScheduleOfPost(ctx, postID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("social：讀取貼文的排程 %s：%w", postID, err)
	}
	return true, nil
}

func (s *Service) scheduledBy(st seller.Store, by model.ScheduledBy) (model.ScheduledBy, error) {
	switch by.Kind {
	case model.ScheduleByOwner:
		return model.ScheduledBy{Kind: by.Kind, Ref: strconv.FormatInt(st.AccountID, 10)}, nil
	case model.ScheduleByDelegation:
		if by.Ref == "" || utf8.RuneCountInString(by.Ref) > model.MaxScheduledByRefRunes || trimtext.Trim(by.Ref) != by.Ref {
			return model.ScheduledBy{}, fmt.Errorf("social：排程發起者 %s 的 ref 不合法", by.Kind)
		}
		return by, nil
	}
	return model.ScheduledBy{}, fmt.Errorf("social：不認得的排程發起者種類 %q", by.Kind)
}

// Schedule 排程發布：DRAFT 或 UNPUBLISHED 的貼文，publish_at 距現在 5 分鐘到 90 天，仍要滿足公開要件；貼文已有待發布的排程時先把它轉 REPLACED。
// OWNER 的 ref 由伺服器填店家的帳號 ID（呼叫方給的忽略），DELEGATION 的 ref 由呼叫方給（代行 session 的 ID）。店家要有 CapMarketing。
func (s *Service) Schedule(ctx context.Context, st seller.Store, in model.SchedulePostInput, actor *delegatedactor.Actor) (model.ScheduledPost, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return model.ScheduledPost{}, err
	}
	if in.Version <= 0 {
		return model.ScheduledPost{}, apperr.InvalidInput("%s", text.PostScheduleEtagRequired(uitext.LocaleFrom(ctx)))
	}
	by, err := s.scheduledBy(st, in.By)
	if err != nil {
		return model.ScheduledPost{}, err
	}
	now := s.d.Now().UTC()
	publishAt := in.PublishAt.UTC().Truncate(time.Microsecond)
	if publishAt.Before(now.Add(model.MinScheduleLead)) || publishAt.After(now.Add(model.MaxScheduleLead)) {
		return model.ScheduledPost{}, apperr.InvalidInputOn([]string{"PublishAt"}, "%s", text.PostScheduleTimeOutOfRange(uitext.LocaleFrom(ctx), text.PostScheduleTimeOutOfRangeArgs{
			Minutes: int(model.MinScheduleLead / time.Minute), Days: int(model.MaxScheduleLead / (24 * time.Hour)),
		}))
	}
	var out store.SocialPostSchedule
	var version int64
	var posted model.Post
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := s.lockOwned(ctx, q, st.StoreID, in.PostID)
		if err != nil {
			return err
		}
		if model.PostStatus(row.Status) == model.StatusPublished {
			return apperr.FailedPrecondition("%s", text.PostScheduleStatusInvalid(uitext.LocaleFrom(ctx)))
		}
		if row.Version != in.Version {
			return versionConflict(ctx)
		}
		if err := rejectIfTakenDown(ctx, q, row.ID, text.PostScheduleTakenDown(uitext.LocaleFrom(ctx))); err != nil {
			return err
		}
		if err := requirePublishable(ctx, row.Body, row.ImageKey != nil); err != nil {
			return err
		}
		if _, err := s.d.Cancel.FromLockedPosts(ctx, []uuid.UUID{row.ID}, model.CancelReplaced, now, schedule.CallerBumps); err != nil {
			return err
		}
		if version, err = s.bumpVersion(ctx, q, row, now); err != nil {
			return err
		}
		out, err = q.InsertSchedule(ctx, store.InsertScheduleParams{
			ID: s.d.NewID(), PostID: row.ID, StoreID: row.StoreID, PublishAt: publishAt,
			ScheduledByKind: string(by.Kind), ScheduledByRef: by.Ref, Now: now,
		})
		if err != nil {
			return fmt.Errorf("social：新增排程：%w", err)
		}
		opts := PublishScheduledArgs{}.InsertOpts()
		opts.ScheduledAt = publishAt
		if err := s.d.Jobs.Enqueue(ctx, PublishScheduledArgs{ScheduleID: out.ID}, &opts); err != nil {
			return err
		}
		if posted, err = s.Own(ctx, st.StoreID, row.ID); err != nil {
			return err
		}
		return emit(ctx, s.d.Events, model.PostScheduled{PostWritten: s.postWritten(posted.ID, posted.StoreID, posted.Body, posted.Version, actor)}, actor)
	})
	if err != nil {
		return model.ScheduledPost{}, err
	}
	return model.ScheduledPost{Schedule: schedule.Of(out), PostVersion: version, Post: posted}, nil
}

// bumpVersion 遞增貼文版本並回傳新版本：排程與取消排程改變貼文的狀態組合，持有舊 etag 的編輯、發布要重新讀取。貼文列已鎖。
func (s *Service) bumpVersion(ctx context.Context, q *store.Queries, row store.SocialPost, now time.Time) (int64, error) {
	upd, err := q.BumpPostVersion(ctx, store.BumpPostVersionParams{ID: row.ID, Now: now, ExpectedVersion: row.Version})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, versionConflict(ctx)
	}
	if err != nil {
		return 0, fmt.Errorf("social：遞增貼文版本 %s：%w", row.ID, err)
	}
	return upd.Version, nil
}

// CancelSchedule 是店主取消貼文待發布的排程（OWNER_CANCELED），version 是讀取時的版本，回傳取消後貼文的新版本；沒有待發布的排程時什麼都不做
// （冪等：先看有沒有排程、再比版本，所以不檢查版本），回傳貼文目前的版本。店家要有 CapMarketing。
func (s *Service) CancelSchedule(ctx context.Context, st seller.Store, postID uuid.UUID, version int64) (int64, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return 0, err
	}
	if version <= 0 {
		return 0, apperr.InvalidInput("%s", text.PostScheduleCancelEtagRequired(uitext.LocaleFrom(ctx)))
	}
	now := s.d.Now().UTC()
	var out int64
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := s.lockOwned(ctx, q, st.StoreID, postID)
		if err != nil {
			return err
		}
		out = row.Version
		pending, err := s.hasPendingSchedule(ctx, q, row.ID)
		if err != nil || !pending {
			return err
		}
		if row.Version != version {
			return versionConflict(ctx)
		}
		if _, err := s.d.Cancel.FromLockedPosts(ctx, []uuid.UUID{row.ID}, model.CancelOwnerCanceled, now, schedule.CallerBumps); err != nil {
			return err
		}
		out, err = s.bumpVersion(ctx, q, row, now)
		return err
	})
	return out, err
}

// PendingSchedule 讀貼文待發布的排程；貼文不是這家店的或沒有待發布的排程回 NotFound。
func (s *Service) PendingSchedule(ctx context.Context, storeID int64, postID uuid.UUID) (model.Schedule, error) {
	row, err := s.q(ctx).GetPendingScheduleOfPost(ctx, postID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && row.StoreID != storeID {
		return model.Schedule{}, apperr.NotFound("%s", text.PostScheduleNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return model.Schedule{}, fmt.Errorf("social：讀取貼文的排程 %s：%w", postID, err)
	}
	return schedule.Of(row), nil
}

// ListPendingSchedules 列出店家待發布的排程：發布時間早的在前（keyset）。
func (s *Service) ListPendingSchedules(ctx context.Context, storeID int64, after *model.ScheduleCursor, size int32) (model.SchedulePage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.SchedulePage{}, err
	}
	at, id := after.Key()
	rows, err := s.q(ctx).ListPendingSchedulesOfStore(ctx, store.ListPendingSchedulesOfStoreParams{StoreID: storeID, AfterPublishAt: at, AfterID: id, MaxRows: size + 1})
	if err != nil {
		return model.SchedulePage{}, fmt.Errorf("social：列出店家的排程：%w", err)
	}
	rows, more := page(rows, size)
	out := model.SchedulePage{Schedules: make([]model.Schedule, len(rows))}
	for i, r := range rows {
		out.Schedules[i] = schedule.Of(r)
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.ScheduleCursor{PublishAt: last.PublishAt, ID: last.ID}
	}
	return out, nil
}

// PendingSchedulerRefs 回傳某種發起者目前有待發布排程的 ref（去重，筆數以待發布的排程為界）。
func (s *Service) PendingSchedulerRefs(ctx context.Context, kind model.ScheduleKind) ([]string, error) {
	refs, err := s.q(ctx).PendingSchedulerRefs(ctx, string(kind))
	if err != nil {
		return nil, fmt.Errorf("social：列出發起者 %s 的待發布排程：%w", kind, err)
	}
	return refs, nil
}

// resolveCanceled 把排程寫成 CANCELED（原因凍結在列上），遞增貼文版本並發事件；貼文列已鎖。貼文已不存在時 bump 為 CallerBumps。不排快取失效：有待發布排程的貼文不是公開的。
func (s *Service) resolveCanceled(ctx context.Context, q *store.Queries, id uuid.UUID, reason model.CancelReason, now time.Time, bump schedule.VersionBump) error {
	r := string(reason)
	row, err := q.ResolveSchedule(ctx, store.ResolveScheduleParams{ID: id, State: string(model.ScheduleCanceled), CancelReason: &r, Now: &now})
	if err != nil {
		return fmt.Errorf("social：取消排程 %s：%w", id, err)
	}
	return s.d.Cancel.After(ctx, []store.SocialPostSchedule{row}, now, bump)
}

// RunScheduled 是排程到點的 job 內容：同一個 transaction 先鎖貼文列、再鎖排程列（與手動發布、刪除、取消同一個順序，所以和它們序列化），不是 PENDING 就
// 什麼都不做（重試、重複的 job 冪等）。先做 social 自己的檢查：貼文已刪或被下架就取消（DELETED、TAKEN_DOWN），貼文已是 PUBLISHED 取消為 REPLACED，
// 到點已超過 model.ScheduleExpiry 取消為 EXPIRED；再問發起者種類的授權實作：ErrScheduleDenied 取消為 AUTHORITY_LOST，其他錯誤回傳讓 job 重試；
// 然後在任何寫入之前檢查公開要件，不滿足（內文或圖片被拿掉）取消為 NOT_PUBLISHABLE（授權失效優先）。這些取消都不重試。通過才經 publishLocked 發布，
// 排程轉 PUBLISHED；發布與訂閱者的任何錯誤都原樣回傳（整個 transaction 回滾、job 重試）。
func (s *Service) RunScheduled(ctx context.Context, scheduleID uuid.UUID) error {
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		peek, err := q.GetSchedule(ctx, scheduleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("social：讀取排程 %s：%w", scheduleID, err)
		}
		if model.ScheduleState(peek.State) != model.SchedulePending {
			return nil
		}
		row, err := q.LockPost(ctx, peek.PostID)
		gone := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !gone {
			return fmt.Errorf("social：鎖定貼文 %s：%w", peek.PostID, err)
		}
		sc, err := q.LockSchedule(ctx, scheduleID)
		if err != nil {
			return fmt.Errorf("social：鎖定排程 %s：%w", scheduleID, err)
		}
		if model.ScheduleState(sc.State) != model.SchedulePending {
			return nil
		}
		if gone {
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelDeleted, now, schedule.CallerBumps)
		}
		down, err := q.PostIsTakenDown(ctx, &row.ID)
		if err != nil {
			return fmt.Errorf("social：讀取貼文的下架 %s：%w", row.ID, err)
		}
		if down {
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelTakenDown, now, schedule.BumpVersion)
		}
		if model.PostStatus(row.Status) == model.StatusPublished {
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelReplaced, now, schedule.BumpVersion)
		}
		if now.After(sc.PublishAt.Add(model.ScheduleExpiry)) {
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelExpired, now, schedule.BumpVersion)
		}
		auth := s.d.Authorizers[model.ScheduleKind(sc.ScheduledByKind)]
		if auth == nil {
			return fmt.Errorf("social：排程 %s 的發起者 %s 沒有授權實作", scheduleID, sc.ScheduledByKind)
		}
		err = auth.AuthorizeScheduledPublish(ctx, model.ScheduledPublish{
			ScheduleID: sc.ID, PostID: sc.PostID, StoreID: sc.StoreID, PublishAt: sc.PublishAt,
			By: model.ScheduledBy{Kind: model.ScheduleKind(sc.ScheduledByKind), Ref: sc.ScheduledByRef},
		})
		switch {
		case errors.Is(err, model.ErrScheduleDenied):
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelAuthorityLost, now, schedule.BumpVersion)
		case err != nil:
			return err
		}
		if err := checkPublishable(ctx, row.Body, row.ImageKey != nil); err != nil {
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelNotPublishable, now, schedule.BumpVersion)
		}
		if err := s.publishLocked(ctx, q, row, now, model.OriginScheduled, sc, nil); err != nil {
			return err
		}
		if _, err := q.ResolveSchedule(ctx, store.ResolveScheduleParams{ID: scheduleID, State: string(model.SchedulePublished), Now: &now}); err != nil {
			return fmt.Errorf("social：完成排程 %s：%w", scheduleID, err)
		}
		return nil
	})
}

// RunScheduledAttempt 是 job 的一次嘗試：final 為真（最後一次）而且失敗時，在失敗的 transaction 回滾之後另開 transaction 呼叫 ExhaustSchedule，仍回傳原錯誤。
func (s *Service) RunScheduledAttempt(ctx context.Context, scheduleID uuid.UUID, final bool) error {
	err := s.RunScheduled(ctx, scheduleID)
	if err == nil || !final {
		return err
	}
	exhaust, cancel := context.WithTimeout(context.WithoutCancel(ctx), exhaustTimeout)
	defer cancel()
	return errors.Join(err, s.ExhaustSchedule(exhaust, scheduleID, err))
}

// ExhaustSchedule 在到點發布的 job 重試用盡後把排程寫成 CANCELED（RETRY_EXHAUSTED），在自己的 transaction（貼文列 → 排程列）內；排程已不是 PENDING 什麼都不做。
// cause 是最後一次失敗，連同結果記 error 等級的 log。
func (s *Service) ExhaustSchedule(ctx context.Context, scheduleID uuid.UUID, cause error) error {
	now := s.d.Now().UTC()
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		peek, err := q.GetSchedule(ctx, scheduleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("social：讀取排程 %s：%w", scheduleID, err)
		}
		if model.ScheduleState(peek.State) != model.SchedulePending {
			return nil
		}
		_, err = q.LockPost(ctx, peek.PostID)
		gone := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !gone {
			return fmt.Errorf("social：鎖定貼文 %s：%w", peek.PostID, err)
		}
		sc, err := q.LockSchedule(ctx, scheduleID)
		if err != nil {
			return fmt.Errorf("social：鎖定排程 %s：%w", scheduleID, err)
		}
		if model.ScheduleState(sc.State) != model.SchedulePending {
			return nil
		}
		bump := schedule.BumpVersion
		if gone {
			bump = schedule.CallerBumps
		}
		return s.resolveCanceled(ctx, q, scheduleID, model.CancelRetryExhausted, now, bump)
	})
	s.d.Logger.ErrorContext(ctx, "social：排程發布重試用盡，排程已取消", "schedule_id", scheduleID, "cause", cause, "resolve_error", err)
	return err
}

type publishScheduledWorker struct {
	river.WorkerDefaults[PublishScheduledArgs]
	svc *Service
}

func (w *publishScheduledWorker) Work(ctx context.Context, job *river.Job[PublishScheduledArgs]) error {
	return w.svc.RunScheduledAttempt(ctx, job.Args.ScheduleID, job.Attempt >= job.MaxAttempts)
}
func (w *publishScheduledWorker) Timeout(*river.Job[PublishScheduledArgs]) time.Duration {
	return time.Minute
}

const exhaustTimeout = 30 * time.Second

// AddWorkers 登記排程發布的 worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &publishScheduledWorker{svc: s})
}
