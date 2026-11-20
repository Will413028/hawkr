// Package purge 是買家刪帳與店家退場的 crm 資料清除。可見性在 job 跑之前已經改好：訂閱者在刪帳（或退場）的 transaction 內把該身分記進
// inactive_parties（投影遇到它就不寫），只排一個 job，transaction 內的工作量固定（單體規則 4）；實際刪除由 job 分批做
// （一個身分的資料量沒有上限）。job 冪等：述詞是「這個身分還有 crm 資料」。
//
// 買家刪帳：刪該買家在各店的 order_nets（ledger 隨 ON DELETE CASCADE）與 customers（customer_tags 隨 cascade）。
// 店家退場：刪該店的 order_nets、customers 與 tags。
package purge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Kind 是被刪除的身分種類，值與 inactive_parties 的 CHECK 一致。
type Kind string

const (
	Buyer Kind = "BUYER"
	Store Kind = "STORE"
)

// Valid 回報 k 是不是已知的身分種類。
func (k Kind) Valid() bool { return k == Buyer || k == Store }

// batchSize 是每一批刪除的列數。
const batchSize = 500

// sweepBatch 是保底掃描每次最多補排的身分數。
const sweepBatch = 100

// SweepInterval 是保底掃描的間隔。
const SweepInterval = 15 * time.Minute

// Deps 是清除的依賴。
type Deps struct {
	DB     *db.DB
	Jobs   jobs.Enqueuer
	Logger *slog.Logger
}

// Service 執行清除。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// ErasePartyArgs 清除一個身分（買家或店家）的 crm 資料。
type ErasePartyArgs struct {
	PartyKind string `json:"party_kind" river:"unique"`
	PartyID   int64  `json:"party_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (ErasePartyArgs) Kind() string { return "crm.erase_party" }

// InsertOpts 讓同一個身分還沒完成的 job 合併：唯一鍵是 (PartyKind, PartyID)，含執行中的狀態（River 要求 available、pending、running、
// retryable、scheduled 都在內）。身分進入刪除之後投影不再寫它的資料（inactive 檢查與 Deactivate 以身分鎖串行），所以執行中的 job 已涵蓋全部。
func (ErasePartyArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

// Deactivate 是刪帳或退場時同 transaction 的原子狀態變更：先持該身分的 advisory lock（與投影串行），記進 inactive_parties、排清除 job。
// 冪等。必須在 transaction 內。
func (s *Service) Deactivate(ctx context.Context, kind Kind, id int64, at time.Time) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return fmt.Errorf("crm：Deactivate 必須在 transaction 內")
	}
	if !kind.Valid() || id <= 0 {
		return fmt.Errorf("crm：停用的身分 %q/%d 不合法", kind, id)
	}
	q := s.q(ctx)
	if err := q.LockParty(ctx, store.LockPartyParams{Kind: string(kind), PartyID: id}); err != nil {
		return err
	}
	if _, err := q.InsertInactiveParty(ctx, store.InsertInactivePartyParams{Kind: string(kind), PartyID: id, Since: at.UTC()}); err != nil {
		return err
	}
	return s.enqueue(ctx, kind, id)
}

func (s *Service) enqueue(ctx context.Context, kind Kind, id int64) error {
	opts := ErasePartyArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, ErasePartyArgs{PartyKind: string(kind), PartyID: id}, &opts)
}

// Sweep 是清除的保底：job 用完 MaxAttempts 被丟棄後，身分仍在 inactive_parties 又還有資料；每次最多補排 sweepBatch 個身分，
// 同一個身分還沒完成的 job 由唯一鍵合併。回傳這次檢查到的身分數。
func (s *Service) Sweep(ctx context.Context) (int, error) {
	var n int
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		parties, err := s.q(ctx).ListInactivePartiesWithData(ctx, sweepBatch)
		if err != nil {
			return err
		}
		for _, p := range parties {
			if err := s.enqueue(ctx, Kind(p.Kind), p.PartyID); err != nil {
				return err
			}
		}
		n = len(parties)
		return nil
	})
	return n, err
}

// Erase 清除一個身分的 crm 資料，分批、每批一個 transaction；重跑只處理剩下的。
// 順序固定（order_nets、customers、tags）：投影的鎖序是 order_nets 列 → customers 列。
func (s *Service) Erase(ctx context.Context, kind Kind, id int64) error {
	if !kind.Valid() || id <= 0 {
		return fmt.Errorf("crm：清除的身分 %q/%d 不合法", kind, id)
	}
	steps := []func(context.Context, *store.Queries) (int64, error){
		func(ctx context.Context, q *store.Queries) (int64, error) {
			if kind == Buyer {
				return q.DeleteOrderNetsByBuyer(ctx, store.DeleteOrderNetsByBuyerParams{BuyerID: id, MaxRows: batchSize})
			}
			return q.DeleteOrderNetsByStore(ctx, store.DeleteOrderNetsByStoreParams{StoreID: id, MaxRows: batchSize})
		},
		func(ctx context.Context, q *store.Queries) (int64, error) {
			if kind == Buyer {
				return q.DeleteCustomersByBuyer(ctx, store.DeleteCustomersByBuyerParams{BuyerID: id, MaxRows: batchSize})
			}
			return q.DeleteCustomersByStore(ctx, store.DeleteCustomersByStoreParams{StoreID: id, MaxRows: batchSize})
		},
	}
	if kind == Store {
		steps = append(steps, func(ctx context.Context, q *store.Queries) (int64, error) {
			return q.DeleteTagsByStore(ctx, store.DeleteTagsByStoreParams{StoreID: id, MaxRows: batchSize})
		})
	}
	for _, step := range steps {
		for {
			var n int64
			err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
				var err error
				n, err = step(ctx, s.q(ctx))
				return err
			})
			if err != nil {
				return err
			}
			if n == 0 {
				break
			}
		}
	}
	return nil
}

type worker struct {
	river.WorkerDefaults[ErasePartyArgs]
	svc *Service
}

// Work 清除一個身分的資料；失敗由 River 重試（冪等）。
func (w *worker) Work(ctx context.Context, job *river.Job[ErasePartyArgs]) error {
	return w.svc.Erase(ctx, Kind(job.Args.PartyKind), job.Args.PartyID)
}

// Timeout 是一次清除的時間上限；超過時 job 重試，從剩下的列接著做。
func (w *worker) Timeout(*river.Job[ErasePartyArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記清除的 River worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &worker{svc: s})
	river.AddWorker(workers, &sweepWorker{svc: s})
}

// SweepArgs 是清除的保底掃描。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "crm.erase_sweep" }

// InsertOpts 以 SweepInterval 為唯一性的時間窗：同一個週期內的排程與重啟合併，窗過了就能再排。
func (SweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByPeriod: SweepInterval}}
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

// PeriodicJobs 回傳啟動即執行、每 SweepInterval 的清除保底掃描。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: "crm.erase_sweep", RunOnStart: true},
		),
	}
}
