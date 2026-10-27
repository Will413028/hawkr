package governance

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// EraseContactsArgs 是匿名檢舉人聯絡欄位的保留期限清理。
type EraseContactsArgs struct{}

// Kind 是 River 的 job 種類。
func (EraseContactsArgs) Kind() string { return "seller.erase_report_contacts" }

// InsertOpts 把同一小時的排程與重啟合併。
func (EraseContactsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Hour}}
}

// eraseBatch 是清理一次 transaction 處理的通報數。
const eraseBatch = 500

// EraseExpiredContacts 把清除時間已到的匿名檢舉的電話、email、姓名清成 NULL，分批各自一個 transaction；回傳清掉的筆數與其中只靠絕對上限清的筆數。清除時間有兩條：
// 有結果的是 greatest(建立 + ContactRetention（90 天）, 結果設定 + ContactRetentionAfterOutcome（30 天）)——結果信要寄到這個地址，所以結果設定之後還要留 30 天，
// 而且不早於建立後 90 天；不論有沒有結果，建立滿 ContactRetentionCap（365 天）一律清（Will 2026-10-18，設計審查 #11：保留要有期限，沒有結果的通報不再無限期保留）。
// 被上限強制清除的筆數記在 log（沒有結果的匿名通報拖滿一年，代表處理流程卡住了，這是看得到的觀測點）。不做 hash：保留期限後沒有人需要比對聯絡方式。
func (s *Service) EraseExpiredContacts(ctx context.Context) (model.ContactErasure, error) {
	now := s.now()
	params := store.EraseExpiredContactsParams{
		Now: now, CapCutoff: now.Add(-model.ContactRetentionCap), CreatedCutoff: now.Add(-model.ContactRetention),
		ResolvedCutoff: now.Add(-model.ContactRetentionAfterOutcome), BatchSize: eraseBatch,
	}
	var total model.ContactErasure
	for {
		var rows []bool
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			rows, err = store.New(s.d.DB.Conn(ctx)).EraseExpiredContacts(ctx, params)
			return err
		})
		if err != nil {
			return total, err
		}
		for _, capped := range rows {
			total.Erased++
			if capped {
				total.Capped++
			}
		}
		if len(rows) < eraseBatch {
			break
		}
	}
	if total.Capped > 0 {
		s.d.Logger.WarnContext(ctx, "seller：匿名通報的聯絡方式滿 365 天，不論有沒有結果都強制清除", "capped", total.Capped, "erased", total.Erased)
	}
	return total, nil
}

type eraseWorker struct {
	river.WorkerDefaults[EraseContactsArgs]
	service *Service
}

// Work 執行清理。
func (w *eraseWorker) Work(ctx context.Context, _ *river.Job[EraseContactsArgs]) error {
	_, err := w.service.EraseExpiredContacts(ctx)
	return err
}

// Timeout 是一次清理的時間上限。
func (w *eraseWorker) Timeout(*river.Job[EraseContactsArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &eraseWorker{service: s})
}

// PeriodicJobs 回傳啟動即執行、每小時補跑的聯絡欄位清理。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return EraseContactsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.erase_report_contacts", RunOnStart: true},
	)}
}
