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
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/schedule"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// ScheduleSweepGrace 是保底掃描的寬限：到點超過這麼久還是 PENDING 的排程才補排 job。
const ScheduleSweepGrace = 10 * time.Minute

// scheduleSweepBatch 是保底掃描每輪補排的上限；沒補完的下一輪（每 5 分鐘）再補。
const scheduleSweepBatch = 200

// PublishScheduledArgs 是排程到點的 job：只帶 schedule_id，唯一的真相是 post_schedules 的列。排程列一次一個、不重用，所以 schedule_id 是冪等鍵：
// 唯一鍵涵蓋排隊中、執行中與重試中的狀態、不含已完成的；到點的執行讀排程列，已經不是 PENDING 就什麼都不做，所以重複的 job（重試、保底掃描補排）無害。
type PublishScheduledArgs struct {
	ScheduleID uuid.UUID `json:"schedule_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (PublishScheduledArgs) Kind() string { return "social.publish_scheduled" }

// InsertOpts 見 PublishScheduledArgs。
func (PublishScheduledArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: unfinishedStates()}}
}

func unfinishedStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}
}

// SweepSchedulesArgs 是排程的保底掃描：補排到點很久還是 PENDING 的排程的 job（job 遺失或被清掉時）。
type SweepSchedulesArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepSchedulesArgs) Kind() string { return "social.sweep_schedules" }

// InsertOpts 讓同一個時段內的排程與重啟合併。
func (SweepSchedulesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 4 * time.Minute}}
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

// ownerAuthorizer 是店主排程的授權：店主沒有 social 之外的授權要重驗，social 自己的檢查（沒刪除、沒下架、仍滿足公開要件）對每一種發起者都在
// 到點時先做（RunScheduled）。
type ownerAuthorizer struct{}

func (ownerAuthorizer) AuthorizeScheduledPublish(context.Context, model.ScheduledPublish) error {
	return nil
}

// OwnerAuthorizer 回傳店主排程的授權實作。
func OwnerAuthorizer() model.ScheduleAuthorizer { return ownerAuthorizer{} }

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
func (s *Service) Schedule(ctx context.Context, st seller.Store, in model.SchedulePostInput) (model.Schedule, error) {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return model.Schedule{}, err
	}
	if in.Version <= 0 {
		return model.Schedule{}, apperr.InvalidInput("%s", text.PostScheduleEtagRequired(uitext.LocaleFrom(ctx)))
	}
	by, err := s.scheduledBy(st, in.By)
	if err != nil {
		return model.Schedule{}, err
	}
	now := s.d.Now().UTC()
	publishAt := in.PublishAt.UTC().Truncate(time.Microsecond)
	if publishAt.Before(now.Add(model.MinScheduleLead)) || publishAt.After(now.Add(model.MaxScheduleLead)) {
		return model.Schedule{}, apperr.InvalidInputOn([]string{"PublishAt"}, "%s", text.PostScheduleTimeOutOfRange(uitext.LocaleFrom(ctx), text.PostScheduleTimeOutOfRangeArgs{
			Minutes: int(model.MinScheduleLead / time.Minute), Days: int(model.MaxScheduleLead / (24 * time.Hour)),
		}))
	}
	var out store.SocialPostSchedule
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
		if err := checkPublishable(ctx, row.Body, row.ImageKey != nil); err != nil {
			return err
		}
		if _, err := s.d.Cancel.FromLockedPosts(ctx, []uuid.UUID{row.ID}, model.CancelReplaced, now); err != nil {
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
		return s.d.Jobs.Enqueue(ctx, PublishScheduledArgs{ScheduleID: out.ID}, &opts)
	})
	if err != nil {
		return model.Schedule{}, err
	}
	return schedule.Of(out), nil
}

// CancelSchedule 是店主取消貼文待發布的排程（OWNER_CANCELED）；沒有待發布的排程時什麼都不做（冪等）。店家要有 CapMarketing。
func (s *Service) CancelSchedule(ctx context.Context, st seller.Store, postID uuid.UUID) error {
	if err := seller.Authorize(st, seller.CapMarketing); err != nil {
		return err
	}
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		row, err := s.lockOwned(ctx, s.q(ctx), st.StoreID, postID)
		if err != nil {
			return err
		}
		_, err = s.d.Cancel.FromLockedPosts(ctx, []uuid.UUID{row.ID}, model.CancelOwnerCanceled, now)
		return err
	})
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

// resolveCanceled 把排程寫成 CANCELED（原因凍結在列上），並發事件與排快取失效。
func (s *Service) resolveCanceled(ctx context.Context, q *store.Queries, id uuid.UUID, reason model.CancelReason, now time.Time) error {
	r := string(reason)
	row, err := q.ResolveSchedule(ctx, store.ResolveScheduleParams{ID: id, State: string(model.ScheduleCanceled), CancelReason: &r, Now: &now})
	if err != nil {
		return fmt.Errorf("social：取消排程 %s：%w", id, err)
	}
	return s.d.Cancel.After(ctx, []store.SocialPostSchedule{row}, now)
}

// RunScheduled 是排程到點的 job 內容：同一個 transaction 先鎖貼文列、再鎖排程列（與手動發布、刪除、取消同一個順序，所以和它們序列化），不是 PENDING 就
// 什麼都不做（重試、重複的 job 冪等）；先做 social 自己的檢查（貼文已刪或被下架就取消，原因 DELETED、TAKEN_DOWN），再問發起者種類的授權實作：
// ErrScheduleDenied 把排程寫成 CANCELED（AUTHORITY_LOST）而且不重試，其他錯誤回傳讓 job 重試；貼文不再滿足公開要件（內文或圖片被拿掉）取消為 NOT_PUBLISHABLE，同樣不重試（授權失效優先）；
// 通過才經手動發布的同一條路徑發布，排程轉 PUBLISHED。
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
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelDeleted, now)
		}
		down, err := q.PostIsTakenDown(ctx, &row.ID)
		if err != nil {
			return fmt.Errorf("social：讀取貼文的下架 %s：%w", row.ID, err)
		}
		if down {
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelTakenDown, now)
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
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelAuthorityLost, now)
		case err != nil:
			return err
		}
		if err := checkPublishable(ctx, row.Body, row.ImageKey != nil); err != nil {
			if apperr.KindOf(err) != apperr.KindInvalidInput {
				return err
			}
			return s.resolveCanceled(ctx, q, scheduleID, model.CancelNotPublishable, now)
		}
		if model.PostStatus(row.Status) == model.StatusPublished {
			return fmt.Errorf("social：排程 %s 的貼文 %s 已經是公開的", scheduleID, row.ID)
		}
		if err := s.publishLocked(ctx, q, row, now, model.OriginScheduled, sc); err != nil {
			return err
		}
		if _, err := q.ResolveSchedule(ctx, store.ResolveScheduleParams{ID: scheduleID, State: string(model.SchedulePublished), Now: &now}); err != nil {
			return fmt.Errorf("social：完成排程 %s：%w", scheduleID, err)
		}
		return nil
	})
}

// SweepSchedules 是保底掃描：到點超過 ScheduleSweepGrace 還是 PENDING 的排程補排 job（唯一鍵讓還在排隊、執行或重試的不重複）。回傳補排的數量。
func (s *Service) SweepSchedules(ctx context.Context) (int, error) {
	n := 0
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListOverduePendingScheduleIDs(ctx, store.ListOverduePendingScheduleIDsParams{Before: s.d.Now().UTC().Add(-ScheduleSweepGrace), MaxRows: scheduleSweepBatch})
		if err != nil {
			return fmt.Errorf("social：列出逾期的排程：%w", err)
		}
		for _, r := range rows {
			if err := s.d.Jobs.Enqueue(ctx, PublishScheduledArgs{ScheduleID: r.ID}, nil); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

type publishScheduledWorker struct {
	river.WorkerDefaults[PublishScheduledArgs]
	svc *Service
}

func (w *publishScheduledWorker) Work(ctx context.Context, job *river.Job[PublishScheduledArgs]) error {
	return w.svc.RunScheduled(ctx, job.Args.ScheduleID)
}
func (w *publishScheduledWorker) Timeout(*river.Job[PublishScheduledArgs]) time.Duration {
	return time.Minute
}

type sweepSchedulesWorker struct {
	river.WorkerDefaults[SweepSchedulesArgs]
	svc *Service
}

func (w *sweepSchedulesWorker) Work(ctx context.Context, _ *river.Job[SweepSchedulesArgs]) error {
	_, err := w.svc.SweepSchedules(ctx)
	return err
}
func (w *sweepSchedulesWorker) Timeout(*river.Job[SweepSchedulesArgs]) time.Duration {
	return 5 * time.Minute
}

// AddWorkers 登記排程發布的 worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &publishScheduledWorker{svc: s})
	river.AddWorker(workers, &sweepSchedulesWorker{svc: s})
}

// PeriodicJobs 是每 5 分鐘的排程保底掃描。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(5*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return SweepSchedulesArgs{}, nil },
		&river.PeriodicJobOpts{ID: "social.sweep_schedules", RunOnStart: true},
	)}
}
