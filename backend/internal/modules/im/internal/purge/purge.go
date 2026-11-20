// Package purge 是買家刪帳與店家退場的訊息抹除。可見性在 job 跑之前已經改好：訂閱者在刪帳（或退場）的 transaction 內把該身分記進
// inactive_parties（送出擋寄件人與收件人），只排一個 job，transaction 內的工作量固定（單體規則 4）；實際抹除由 job 依 id 升序分批做
// （一個身分寄出的訊息數沒有上限）。訊息列保留（seq 連續），內容與圖片清空；job 冪等：述詞是「這個身分還有沒抹除的訊息」。
package purge

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// batchSize 是每一批抹除的訊息數。
const batchSize = 200

// sweepBatch 是保底掃描每次最多補排的身分數。
const sweepBatch = 100

// SweepInterval 是保底掃描的間隔。
const SweepInterval = 15 * time.Minute

// Deps 是抹除的依賴。
type Deps struct {
	DB     *db.DB
	Jobs   jobs.Enqueuer
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

// InsertOpts 讓同一個身分還沒開始的 job 合併：唯一鍵是 (PartyKind, PartyID)，不含執行中的狀態（job 可重跑），
// 執行中又排入的那一次排在它後面，不會被略過。身分進入刪除之後沒有新訊息：Send 在 transaction 內持 LockParty 再查 inactive_parties，
// 與 Deactivate 串行，所以抹除 job 看不到之後才送出的訊息。
func (ErasePartyArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

// Deactivate 是刪帳或退場時同 transaction 的原子狀態變更：記進 inactive_parties、排抹除 job。先持該身分的 advisory lock，
// 與 Send 的 insert transaction 串行。冪等。必須在 transaction 內。
func (s *Service) Deactivate(ctx context.Context, kind model.PartyKind, id int64, at time.Time) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return fmt.Errorf("im：Deactivate 必須在 transaction 內")
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

func (s *Service) enqueue(ctx context.Context, kind model.PartyKind, id int64) error {
	opts := ErasePartyArgs{}.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, ErasePartyArgs{PartyKind: string(kind), PartyID: id}, &opts)
}

// Sweep 是抹除的保底：job 用完 MaxAttempts 被丟棄後，身分仍在 inactive_parties 又還有未抹除的訊息；
// 這裡每次最多補排 sweepBatch 個身分的 job，同一個身分還沒開始的 job 由唯一鍵合併。回傳這次檢查到的身分數。
func (s *Service) Sweep(ctx context.Context) (int, error) {
	var n int
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		parties, err := s.q(ctx).ListInactivePartiesWithMessages(ctx, sweepBatch)
		if err != nil {
			return err
		}
		for _, p := range parties {
			if err := s.enqueue(ctx, model.PartyKind(p.Kind), p.PartyID); err != nil {
				return err
			}
		}
		n = len(parties)
		return nil
	})
	return n, err
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
	asset *string
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
		for _, m := range batch {
			if m.asset == nil {
				continue
			}
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.MessageRef(m.id), *m.asset); err != nil {
				return err
			}
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

// InsertOpts 讓同時只有一個掃描排隊。
func (SweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true}}
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
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(SweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "im.erase_sweep", RunOnStart: true},
	)}
}
