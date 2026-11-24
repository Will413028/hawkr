// Package expiry 是券的到期提醒：週期 job 找出落進提醒窗的 AVAILABLE 券，在 transaction 內發布 VoucherExpiring，
// 並把持久化的游標推進到處理過的位置，重跑不重發。時間一律 UTC，時鐘由呼叫方注入。
package expiry

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// LeadDays 是提醒的提前天數（coupon-service 的 days=1）。提醒窗是「到下一個 UTC 日結束」：UTC 日一開始，
// 當天與隔天到期的券都進窗，所以買家至少有一整天的提前量。
const LeadDays = 1

// batchSize 是每個 transaction 處理的券數；游標隨批推進，job 中斷後從上一批之後繼續。
const batchSize int32 = 200

// Interval 是 job 的排程間隔：窗以 UTC 日為單位，一天內只有第一次有工作，其餘只是一次游標查詢，間隔也讓停機後補得上。
const Interval = time.Hour

// Horizon 回傳 now 當下的提醒窗上界（含）：now 所在 UTC 日的零時加 (LeadDays+1) 天。
func Horizon(now time.Time) time.Time {
	t := now.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, LeadDays+1)
}

// Deps 是到期提醒的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Service 執行到期提醒。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// epoch 是游標的初始值：所有券都在它之後。
var epoch = time.Unix(0, 0).UTC()

// Run 處理目前提醒窗內還沒處理過的券並回傳發布的事件數。同一時間只有一個 Run 在處理（鎖游標列）；
// 每一批的事件與游標推進在同一個 transaction，任何訂閱者失敗整批回滾、游標不動。
func (s *Service) Run(ctx context.Context) (int, error) {
	if s.d.Events == nil {
		return 0, errors.New("coupon：到期提醒需要 Events")
	}
	now := s.d.Now()
	horizon := Horizon(now)
	total := 0
	for {
		n, done, err := s.batch(ctx, now, horizon)
		total += n
		if err != nil {
			return total, err
		}
		if done {
			return total, nil
		}
	}
}

func (s *Service) batch(ctx context.Context, now, horizon time.Time) (emitted int, done bool, err error) {
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if err := q.EnsureExpiryCursor(ctx, store.EnsureExpiryCursorParams{EndAt: epoch, VoucherID: uuid.Nil()}); err != nil {
			return err
		}
		cur, err := q.LockExpiryCursor(ctx)
		if err != nil {
			return err
		}
		list, err := q.ListExpiringVouchers(ctx, store.ListExpiringVouchersParams{
			Now: now, Horizon: horizon, AfterEndAt: cur.EndAt, AfterID: cur.VoucherID, BatchSize: batchSize,
		})
		if err != nil {
			return err
		}
		for _, v := range list {
			if err := events.Publish(ctx, s.d.Events, model.VoucherExpiring{
				EventID: s.d.NewID(), OccurredAt: now, BuyerID: v.BuyerID, VoucherID: v.ID,
				CampaignID: v.CampaignID, CampaignTitle: v.Title, ExpiresAt: v.EndAt.UTC(),
			}); err != nil {
				return err
			}
		}
		emitted = len(list)
		next := store.AdvanceExpiryCursorParams{EndAt: horizon, VoucherID: uuid.Max()}
		if len(list) == int(batchSize) {
			last := list[len(list)-1]
			next = store.AdvanceExpiryCursorParams{EndAt: last.EndAt, VoucherID: last.ID}
		} else {
			done = true
		}
		if !cursorBefore(cur, next) {
			return nil
		}
		return q.AdvanceExpiryCursor(ctx, next)
	})
	return emitted, done, err
}

// cursorBefore 回報游標 cur 是否在 next 之前：游標只前進，時鐘倒退不會把它拉回。
func cursorBefore(cur store.LockExpiryCursorRow, next store.AdvanceExpiryCursorParams) bool {
	if !cur.EndAt.Equal(next.EndAt) {
		return cur.EndAt.Before(next.EndAt)
	}
	return cur.VoucherID.Compare(next.VoucherID) < 0
}

// Args 是週期 job 的參數。
type Args struct{}

// Kind 是 River 的 job 種類。
func (Args) Kind() string { return "coupon.expiry_reminder" }

// InsertOpts 讓同一個 interval 內的排程與重啟合併。
func (Args) InsertOpts() river.InsertOpts { return jobs.SweepOpts(Interval) }

type worker struct {
	river.WorkerDefaults[Args]
	service *Service
}

func (w *worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	if _, err := w.service.Run(ctx); err != nil {
		return fmt.Errorf("coupon：到期提醒：%w", err)
	}
	return nil
}

// AddWorkers 登記 worker。
func AddWorkers(workers *river.Workers, s *Service) { river.AddWorker(workers, &worker{service: s}) }

// PeriodicJobs 回傳排程：啟動即跑，之後每 Interval 一次。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(Interval),
		func() (river.JobArgs, *river.InsertOpts) { return Args{}, nil },
		&river.PeriodicJobOpts{ID: "coupon.expiry_reminder", RunOnStart: true},
	)}
}
