// Package purge 是買家刪帳與店家退場的訊息抹除。可見性在 job 跑之前已經改好：擁有者（buyer、seller）的狀態就是事實，
// 送出以 transaction 內的 LockNotClosed、LockNotRetired 擋寄件人與收件人，讀取 join buyer.pub_closed_buyers()、seller.pub_retired_stores()；
// 訂閱者在刪帳（或退場）的 transaction 內只排一個 job，工作量固定（單體規則 4）；實際抹除由 job 依 id 升序分批做
// （一個身分寄出的訊息數沒有上限）。訊息列保留（seq 連續），內容與圖片清空；job 冪等：述詞是「這個身分還有沒抹除的訊息」。
package purge

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const batchSize = 200

const sweepBatch = 100

// SweepInterval 是保底掃描的間隔。
const SweepInterval = 15 * time.Minute

// Deps 是抹除的依賴。
type Deps struct {
	DB     *db.DB
	Jobs   jobs.Enqueuer
	Media  model.Media
	NewID  func() uuid.UUID
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 執行抹除。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// ErasePartyArgs 抹除一個身分（買家或店家）寄出的訊息。
type ErasePartyArgs struct {
	PartyKind string `json:"party_kind" river:"unique"`
	PartyID   int64  `json:"party_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (ErasePartyArgs) Kind() string { return "im.erase_party" }

// InsertOpts 讓同一個身分還沒開始的 job 合併：唯一鍵是 (PartyKind, PartyID)，含執行中的狀態（River 要求 available、pending、running、scheduled 都在內）：執行中又排入的那一次併入它：
// 該身分進入刪除後不再有新資料，執行中的 job 已涵蓋全部。身分進入刪除之後沒有新訊息：Send 在 transaction 內持擁有者列的 FOR KEY SHARE（買家 LockNotClosed、店家 LockNotRetired），
// 擁有者轉移（關閉買家、退場店家）的 FOR UPDATE 與它互斥，所以抹除 job 看不到之後才送出的訊息。
func (ErasePartyArgs) InsertOpts() river.InsertOpts {
	return jobs.UniqueByArgs(10)
}

// EnqueueErase 在買家關閉或店家退場時排抹除 job：只排 job，不要求 transaction（訂閱者在轉移的 transaction 內呼叫時併入它）。
// 冪等：抹除 job 以 (PartyKind, PartyID) 為唯一鍵合併。事實不在這裡存，由擁有者的狀態與 pub 函式提供。
func (s *Service) EnqueueErase(ctx context.Context, kind model.PartyKind, id int64) error {
	if !kind.Valid() || id <= 0 {
		return fmt.Errorf("im：停用的身分 %q/%d 不合法", kind, id)
	}
	return s.enqueue(ctx, kind, id)
}

func (s *Service) enqueue(ctx context.Context, kind model.PartyKind, id int64) error {
	opts := ErasePartyArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, ErasePartyArgs{PartyKind: string(kind), PartyID: id}, &opts)
}

// Sweep 是抹除的保底：job 用完 MaxAttempts 被丟棄後，身分已在 buyer.pub_closed_buyers()／seller.pub_retired_stores() 又還有未抹除的訊息；
// 以 (kind, party_id) 的游標逐頁（每頁 sweepBatch 個、每頁一個 transaction）走完全部，同一個身分還沒開始的 job 由唯一鍵合併；
// 每頁固定上限，前面的身分一直失敗也不會餓死後面的。回傳這次檢查到的身分數。
func (s *Service) Sweep(ctx context.Context) (int, error) {
	total := 0
	afterKind, afterID := "", int64(0)
	for {
		var parties []store.ListInactivePartiesWithMessagesRow
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			parties, err = s.q(ctx).ListInactivePartiesWithMessages(ctx, store.ListInactivePartiesWithMessagesParams{AfterKind: afterKind, AfterID: afterID, MaxRows: sweepBatch})
			if err != nil {
				return err
			}
			for _, p := range parties {
				if err := s.enqueue(ctx, model.PartyKind(p.Kind), p.PartyID); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return total, err
		}
		total += len(parties)
		if len(parties) < sweepBatch {
			return total, nil
		}
		last := parties[len(parties)-1]
		afterKind, afterID = last.Kind, last.PartyID
	}
}

// Erase 抹除一個身分寄出的訊息：依 id 升序分批，每批一個 transaction，內含清空內容與圖片欄位（寫 erased_at）、釋放的圖記成退場並排媒體同步。
// 重跑只處理剩下的。
func (s *Service) Erase(ctx context.Context, kind model.PartyKind, id int64) error {
	if !kind.Valid() || id <= 0 {
		return fmt.Errorf("im：抹除的身分 %q/%d 不合法", kind, id)
	}
	for {
		n, err := s.eraseBatch(ctx, kind, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

type locked struct {
	id    uuid.UUID
	asset *uuid.UUID
}

func (s *Service) eraseBatch(ctx context.Context, kind model.PartyKind, id int64) (int, error) {
	n := 0
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var batch []locked
		if kind == model.PartyBuyer {
			rs, err := q.LockBuyerMessageBatch(ctx, store.LockBuyerMessageBatchParams{BuyerID: id, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("im：鎖定買家的訊息：%w", err)
			}
			for _, r := range rs {
				batch = append(batch, locked{r.ID, r.ImageAssetID})
			}
		} else {
			rs, err := q.LockStoreMessageBatch(ctx, store.LockStoreMessageBatchParams{StoreID: id, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("im：鎖定店家的訊息：%w", err)
			}
			for _, r := range rs {
				batch = append(batch, locked{r.ID, r.ImageAssetID})
			}
		}
		if len(batch) == 0 {
			return nil
		}
		now := s.d.Now().UTC()
		ids := make([]uuid.UUID, len(batch))
		for i, m := range batch {
			ids[i] = m.id
		}
		if _, err := q.EraseMessages(ctx, store.EraseMessagesParams{Ids: ids, Now: &now}); err != nil {
			return fmt.Errorf("im：抹除訊息：%w", err)
		}
		var assets []uuid.UUID
		for _, m := range batch {
			if m.asset != nil {
				assets = append(assets, *m.asset)
			}
		}
		if err := s.d.Media.MarkDeleted(ctx, assets); err != nil {
			return err
		}
		n = len(batch)
		return nil
	})
	return n, err
}

type worker struct {
	river.WorkerDefaults[ErasePartyArgs]
	svc *Service
}

// Work 抹除一個身分的訊息；失敗由 River 重試（冪等）。
func (w *worker) Work(ctx context.Context, job *river.Job[ErasePartyArgs]) error {
	return w.svc.Erase(ctx, model.PartyKind(job.Args.PartyKind), job.Args.PartyID)
}

// Timeout 是一次抹除的時間上限；超過時 job 重試，從剩下的列接著做。
func (w *worker) Timeout(*river.Job[ErasePartyArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記抹除的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &worker{svc: s})
	river.AddWorker(workers, &sweepWorker{svc: s})
}

// SweepArgs 是抹除的保底掃描。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "im.erase_sweep" }

// InsertOpts 以 SweepInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併，窗過了就能再排。
func (SweepArgs) InsertOpts() river.InsertOpts {
	o := jobs.SweepOpts(SweepInterval)
	o.MaxAttempts = 1
	return o
}

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	svc *Service
}

// Work 執行保底掃描。
func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := w.svc.Sweep(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return time.Minute }

// PeriodicJobs 回傳啟動即執行、每 SweepInterval 的抹除保底掃描。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: "im.erase_sweep", RunOnStart: true},
		),
	}
}
