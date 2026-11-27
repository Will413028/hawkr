// Package expiry 是券的到期提醒：週期 job 找出 24 小時內到期的 AVAILABLE 券，每張券在同一個 transaction 內
// 寫一筆提醒紀錄並發布 VoucherExpiring，至多提醒一次。時間一律 UTC，時鐘由呼叫方注入。
package expiry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// LeadTime 是提醒的提前量：活動 end_at 在 (now, now+LeadTime] 內的券進窗，而且領券時距到期不少於 LeadTime。
const LeadTime = 24 * time.Hour

// DefaultBatchSize 是每個 transaction 處理的券數。
const DefaultBatchSize int32 = 200

// Interval 是 job 的排程間隔。
const Interval = time.Hour

// Deps 是到期提醒的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	NewID  func() uuid.UUID
	Now    func() time.Time
	// BatchSize 是每批券數；0 用 DefaultBatchSize。
	BatchSize int32
	// Logger 記錄單張券的失敗；nil 用 slog.Default。
	Logger *slog.Logger
}

// Service 執行到期提醒。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.BatchSize <= 0 {
		d.BatchSize = DefaultBatchSize
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

// Run 處理一次到期提醒，回傳這次發布的事件數。券以 id 分批，每批一個 transaction：
// 鎖住符合條件的券（FOR UPDATE SKIP LOCKED，別處正在處理的略過、下一輪再算）、寫提醒紀錄並發布事件。
// 整批失敗時回滾，改為每張券各一個 transaction 重試，失敗的券記錄後跳過、不擋其他券，Run 最後回傳這些失敗（下一輪重試）。
// 事件指名要 model.ExpiringHandler 處理：沒有這個 handler 時每張券都失敗，不會默默寫下提醒紀錄。
func (s *Service) Run(ctx context.Context) (int, error) {
	if s.d.Events == nil {
		return 0, errors.New("coupon：到期提醒需要 Events")
	}
	now := s.d.Now()
	total := 0
	var failures []error
	after := uuid.Nil()
	for {
		ids, n, err := s.process(ctx, now, after, nil, s.d.BatchSize)
		if err != nil {
			if len(ids) == 0 {
				return total, errors.Join(append(failures, err)...)
			}
			for _, id := range ids {
				if ctx.Err() != nil {
					return total, errors.Join(append(failures, ctx.Err())...)
				}
				_, one, err := s.process(ctx, now, uuid.Nil(), &id, 1)
				if err != nil {
					s.d.Logger.ErrorContext(ctx, "coupon：到期提醒單張券失敗，下一輪重試", "voucher_id", id, "error", err)
					failures = append(failures, fmt.Errorf("voucher %s：%w", id, err))
					continue
				}
				total += one
			}
		}
		total += n
		if len(ids) < int(s.d.BatchSize) {
			return total, errors.Join(failures...)
		}
		after = ids[len(ids)-1]
	}
}

func (s *Service) process(ctx context.Context, now time.Time, after uuid.UUID, only *uuid.UUID, limit int32) (ids []uuid.UUID, emitted int, err error) {
	onlyID := pgtype.UUID{}
	if only != nil {
		onlyID = pgtype.UUID{Bytes: *only, Valid: true}
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		list, err := q.ClaimExpiringVouchers(ctx, store.ClaimExpiringVouchersParams{
			Now: now, LeadSeconds: LeadTime.Seconds(), AfterID: after, OnlyID: onlyID, BatchSize: limit,
		})
		if err != nil {
			return err
		}
		for _, v := range list {
			ids = append(ids, v.ID)
		}
		for _, v := range list {
			if err := q.InsertExpiryReminder(ctx, store.InsertExpiryReminderParams{VoucherID: v.ID, RemindedAt: now}); err != nil {
				return err
			}
			if err := events.PublishRequired(ctx, s.d.Events, model.VoucherExpiring{
				EventID: s.d.NewID(), OccurredAt: now, BuyerID: v.BuyerID, VoucherID: v.ID,
				CampaignID: v.CampaignID, CampaignTitle: v.Title, ExpiresAt: v.EndAt.UTC(),
			}, model.ExpiringHandler); err != nil {
				return err
			}
		}
		emitted = len(list)
		return nil
	})
	if err != nil {
		emitted = 0
	}
	return ids, emitted, err
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
