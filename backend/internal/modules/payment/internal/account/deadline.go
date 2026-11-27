package account

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	deadlineWindow  = 7 * 24 * time.Hour
	deadlineLastDay = 24 * time.Hour
	deadlineBatch   = 200
	deadlineScanInterval = time.Hour
)

// DeadlineScanArgs 是每小時的補件期限掃描。
type DeadlineScanArgs struct{}

// Kind 是 River 的 job 種類。
func (DeadlineScanArgs) Kind() string { return "payment.deadline_scan" }

// InsertOpts 把同一段時間內的排程與重啟合併。
func (DeadlineScanArgs) InsertOpts() river.InsertOpts { return jobs.SweepOpts(deadlineScanInterval) }

type deadlineScanWorker struct {
	river.WorkerDefaults[DeadlineScanArgs]
	service *Service
}

// Work 執行掃描；失敗由 River 重試（重複發布無害）。
func (w *deadlineScanWorker) Work(ctx context.Context, _ *river.Job[DeadlineScanArgs]) error {
	_, err := w.service.ScanDeadlines(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *deadlineScanWorker) Timeout(*river.Job[DeadlineScanArgs]) time.Duration {
	return 5 * time.Minute
}

// TierFor 回傳距離期限 remaining 時的檔位：超過一天是 SEVEN_DAYS，一天以內是 ONE_DAY。
func TierFor(remaining time.Duration) model.DeadlineTier {
	if remaining > deadlineLastDay {
		return model.DeadlineSevenDays
	}
	return model.DeadlineOneDay
}

// ScanDeadlines 對補件期限落在 (now, now+7 天] 的帳戶各發布一次 PayoutDeadlineApproaching（每批在同一個 transaction 內讀取並發布），回傳發布數。
// 每小時執行：同一個期限在 7 天窗口內會被發布多次，檔位只有兩種，消費者以「店家＋期限＋檔位」去重，所以每個檔位只會通知一次；期限改了就是新的提醒。
func (s *Service) ScanDeadlines(ctx context.Context) (int, error) {
	now := s.d.Now().UTC()
	until := now.Add(deadlineWindow)
	published := 0
	var after int64
	for {
		var rows []store.PaymentAccountDeadlinesRow
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			rows, err = store.New(s.d.DB.Conn(ctx)).PaymentAccountDeadlines(ctx, store.PaymentAccountDeadlinesParams{
				Now: now, Until: until, AfterStoreID: after, Batch: deadlineBatch,
			})
			if err != nil {
				return fmt.Errorf("讀取補件期限的帳戶：%w", err)
			}
			for _, r := range rows {
				deadline := r.RequirementsDeadline.UTC()
				if err := events.Publish(ctx, s.d.Events, model.PayoutDeadlineApproaching{
					OccurredAt: now, StoreID: r.StoreID, Deadline: deadline, Tier: TierFor(deadline.Sub(now)),
				}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return published, err
		}
		published += len(rows)
		if len(rows) < deadlineBatch {
			return published, nil
		}
		after = rows[len(rows)-1].StoreID
	}
}
