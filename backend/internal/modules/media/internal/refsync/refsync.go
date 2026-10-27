// Package refsync 是擁有媒體的模組共用的引用同步：業務列是引用的唯一來源，每個 ref 只有這個同步寫入引用。
//
// 擁有者在 transaction 內寫業務列、把被換掉的資產記成退場（tombstone）、遞增這個 ref 的 seq 並排入同步 job（Changed）；job 呼叫
// Syncer.Work：讀擁有者目前的資產與退場資產，Link 目前的、Unlink 退場的，再在同一個 transaction 內結算並完成 job。引用只增加目前
// 指著的、只移除精確列出的退場資產，退場資產不會被寫回同一個 ref（Reject），所以遲到的 RPC 不會誤刪還在使用的引用。
// 版本、退場表與 SQL 各模組在自己的 schema（Queries）；演算法、seq 協定與排程只有這一份。
package refsync

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 時間參數。Link 與 Unlink 各是一次最多 5 秒的呼叫（internal/legacy/mediaservice，沒有內層重試）。
const (
	// ResyncDelay 是同步期間擁有者的狀態又改變時，再跑下一輪前等待的時間。
	ResyncDelay = 2 * time.Second
	// HealDelay 是退場資產第一次解除之後，再解除一次前等待的時間：一次呼叫的逾時（5 秒）加上遠端排隊的餘裕（10 秒），
	// 讓逾時之後才在遠端生效的 LinkImages 也被第二次解除收掉。遠端遲到多久沒有上界，這不是保證，只是把殘餘機率壓低（ADR 後果）。
	HealDelay = 15 * time.Second
	// TombstoneRetention 是退場紀錄至少保留的時間，從退場時算起：staged token 的有效期是上傳會話的期限（一般圖片 30 分鐘，
	// 機敏上傳完成後延長為預覽期限 1 小時），同一個 token 在期限內重新轉正會回到同一個資產，過了期限才不可能再被寫回；
	// 保留時間取最長期限的兩倍。
	TombstoneRetention = 2 * time.Hour
	// MaxBackoff 是兩次重試之間最長的間隔。River 預設的退避（attempt^4 秒）到第 18 次失敗時間隔已超過一天，media 長時間故障
	// 恢復後可能要等到 GC 寬限（7 天）之後才重試；封頂之後，media 恢復最晚一小時內就會登記。
	MaxBackoff = time.Hour
	// MaxAttempts 是一個同步 job 的重試次數：退避封頂後 25 次約橫跨 19 小時。用盡而被 discard 的 job 不再佔唯一鍵，
	// 保底掃描（Sweep）會為落後的 ref 排入新的 job，所以不會留下沒有人處理的 ref。
	MaxAttempts = 25
	// JobTimeout 是一次同步的時間上限：Link、Unlink 各 5 秒，加上資料庫與餘裕。
	JobTimeout = 30 * time.Second
	// SweepInterval 是保底掃描的週期。
	SweepInterval = 15 * time.Minute
	// SweepIdle 是 ref 的版本比已同步的新、而且這麼久沒有變動，掃描才補排它（給正常的 job 先處理的時間）。
	SweepIdle = 10 * time.Minute
	// SweepBatch 是一次掃描最多補排的 ref 數。
	SweepBatch = 500
	// MaxLag 是落後的上限：超過這個時間還沒同步的 ref 代表同步失效，掃描記 error log。它遠小於 GC 的 7 天寬限。
	MaxLag = 24 * time.Hour
)

// Ref 是業務列的引用鍵。
type Ref = model.Ref

// UnsyncedRef 是版本比已同步的新的 ref。
type UnsyncedRef struct {
	Ref       Ref
	UpdatedAt time.Time
}

// Queries 是擁有者在自己 schema 上的 SQL，綁定在呼叫當下的連線（transaction 內或外）。
type Queries interface {
	// RetireAsset 記下被換掉的資產；已經記過的不變。
	RetireAsset(ctx context.Context, ref Ref, assetID string, now time.Time) error
	// BumpRefSeq 遞增 ref 的版本（沒有列時建立），並記下變動時間。
	BumpRefSeq(ctx context.Context, ref Ref, now time.Time) error
	// RefSeq 讀 ref 的版本；沒有列時 found 是 false（讀取不寫入）。
	RefSeq(ctx context.Context, ref Ref) (seq int64, found bool, err error)
	// RetiredAssetIDs 是 ref 還沒清掉的退場資產。
	RetiredAssetIDs(ctx context.Context, ref Ref) ([]string, error)
	// RetiredAmong 是 assetIDs 之中已經在這個 ref 退場的。
	RetiredAmong(ctx context.Context, ref Ref, assetIDs []string) ([]string, error)
	// MarkUnlinked 記下第一次解除的時間（已有的不變）。
	MarkUnlinked(ctx context.Context, ref Ref, assetIDs []string, now time.Time) error
	// SettleRefSeq 只在版本仍等於 seq 時記下已同步，回傳更新的列數。
	SettleRefSeq(ctx context.Context, ref Ref, seq int64) (int64, error)
	// HealingRetired 回報 ref 是否還有第一次解除不早於 healBefore 的退場資產。
	HealingRetired(ctx context.Context, ref Ref, healBefore time.Time) (bool, error)
	// PurgeRetired 刪除解除時間早於 healBefore 而且退場時間早於 retireBefore 的退場紀錄（整個模組）。
	PurgeRetired(ctx context.Context, healBefore, retireBefore time.Time) error
	// UnsyncedRefs 列出版本比已同步的新、而且最後變動早於 idleBefore 的 ref，最舊的先。
	UnsyncedRefs(ctx context.Context, idleBefore time.Time, limit int32) ([]UnsyncedRef, error)
}

// Changed 在寫入 ref 的業務列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job（enqueue）。
// 必須在 transaction 內。版本遞增先於排入：寫入端持有 ref 的列鎖，同步的結算會等它（見 Settle）。
func Changed(ctx context.Context, q Queries, enqueue func(context.Context, Ref) error, now time.Time, ref Ref, retired ...string) error {
	for _, assetID := range retired {
		if err := q.RetireAsset(ctx, ref, assetID, now); err != nil {
			return err
		}
	}
	if err := q.BumpRefSeq(ctx, ref, now); err != nil {
		return err
	}
	return enqueue(ctx, ref)
}

// Candidate 是一個要寫回 ref 的資產，以及它在模組輸入裡的欄位（與轉正時的 media.Field 同一個名字；清單的元素用
// apperr.IndexedField）。
type Candidate struct {
	Field   string
	AssetID string
}

// Reject 拒絕把已退場的資產寫回 ref：同一個 staged token 重新轉正會回到已被換掉的資產。錯誤指到每個已退場資產的欄位（與轉正被
// 拒絕時一致）。必須在鎖住業務列的 transaction 內呼叫（退場與寫入都在同一把列鎖下，所以讀到的退場紀錄是完整的）。
func Reject(ctx context.Context, q Queries, ref Ref, candidates ...Candidate) error {
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.AssetID
	}
	retired, err := q.RetiredAmong(ctx, ref, ids)
	if err != nil {
		return err
	}
	if len(retired) == 0 {
		return nil
	}
	var fields []string
	for _, c := range candidates {
		if slices.Contains(retired, c.AssetID) {
			fields = append(fields, c.Field)
		}
	}
	return apperr.InvalidInputOn(fields, "圖片無法使用，請重新上傳")
}

// Syncer 是一個擁有模組的同步：Media 是遠端，Queries 與 Current 是擁有者的資料，Enqueue 排入這個模組的同步 job。
type Syncer struct {
	DB     *db.DB
	Media  model.Media
	Now    func() time.Time
	Logger *slog.Logger
	// Queries 把擁有者的 SQL 綁到連線上。
	Queries func(db.DBTX) Queries
	// Current 讀 ref 的業務列目前指著的資產（終態的 ref 是空的）。
	Current func(ctx context.Context, ref Ref) ([]string, error)
	// Enqueue 排入 ref 的同步 job（ctx 內有 transaction 時在該 transaction 內）。
	Enqueue func(ctx context.Context, ref Ref) error
}

// Result 是一次同步之後 job 該做的事。
type Result struct {
	// Again 為 true 時，等 After 之後再跑一輪。
	Again bool
	After time.Duration
}

type state struct {
	seq              int64
	found            bool
	current, retired []string
}

// load 先讀版本，再讀退場資產，最後讀目前的資產。這個順序讓並行的寫入只會讓版本與退場資產偏舊（結算的版本比對會多跑一輪），
// 目前與退場重疊只可能來自真正的不變式破壞。
func (s *Syncer) load(ctx context.Context, ref Ref) (state, error) {
	q := s.Queries(s.DB.Conn(ctx))
	var st state
	var err error
	if st.seq, st.found, err = q.RefSeq(ctx, ref); err != nil {
		return st, err
	}
	if st.retired, err = q.RetiredAssetIDs(ctx, ref); err != nil {
		return st, err
	}
	st.current, err = s.Current(ctx, ref)
	return st, err
}

// Sync 同步一個 ref：Link 目前的、Unlink 退場的，再結算。不完成 job（給測試與不在 job 內的呼叫）。
func (s *Syncer) Sync(ctx context.Context, ref Ref) (Result, error) {
	return s.syncOnce(ctx, ref, nil)
}

func (s *Syncer) syncOnce(ctx context.Context, ref Ref, complete func(context.Context) error) (Result, error) {
	st, err := s.load(ctx, ref)
	if err != nil {
		return Result{}, fmt.Errorf("refsync：讀取 %s/%s：%w", ref.Type, ref.ID, err)
	}
	unlink := st.retired
	if overlap := intersect(st.current, st.retired); len(overlap) > 0 {
		// 不變式被破壞（目前指著的資產同時是退場的）：寧可不解除。仍然 Link 目前的並完成，免得新圖在 7 天寬限內一直沒有引用。
		if s.Logger != nil {
			s.Logger.ErrorContext(ctx, "media 引用的不變式被破壞：資產同時是目前與退場，不解除這些資產", "ref_type", ref.Type, "ref_id", ref.ID, "assets", overlap)
		}
		unlink = slices.DeleteFunc(slices.Clone(st.retired), func(a string) bool { return slices.Contains(overlap, a) })
	}
	if len(st.current) > 0 {
		if err := s.Media.Link(ctx, ref, st.current); err != nil {
			return Result{}, err
		}
	}
	if len(unlink) > 0 {
		if err := s.Media.Unlink(ctx, ref, unlink); err != nil {
			return Result{}, err
		}
	}
	now := s.Now()
	var res Result
	err = s.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.Queries(s.DB.Conn(ctx))
		if len(unlink) > 0 {
			if err := q.MarkUnlinked(ctx, ref, unlink, now); err != nil {
				return err
			}
		}
		// 先比對版本（鎖 ref 的版本列、等寫入端 commit），再碰 river_job：寫入端的鎖序是版本列、再 river_job 的唯一索引，
		// 這裡同一個順序，不會互鎖。
		matched := true
		if st.found {
			n, err := q.SettleRefSeq(ctx, ref, st.seq)
			if err != nil {
				return err
			}
			matched = n == 1
		}
		healing, err := q.HealingRetired(ctx, ref, now.Add(-HealDelay))
		if err != nil {
			return err
		}
		switch {
		case !matched:
			res = Result{Again: true, After: ResyncDelay}
		case healing:
			res = Result{Again: true, After: HealDelay}
		case complete != nil:
			// 結算與完成同一個 transaction：不留下「已結算、job 還是 running」的空檔（那段期間寫入端排入的新 job 會被唯一鍵略過）。
			if err := complete(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("refsync：結算 %s/%s：%w", ref.Type, ref.ID, err)
	}
	return res, nil
}

// Work 是同步 job 的 Work：需要再跑一輪時回 River 的 snooze，失敗時記 log 並回錯誤讓 River 重試；一切就緒時在結算的
// transaction 內完成 job。
func Work[A river.JobArgs](ctx context.Context, s *Syncer, job *river.Job[A], ref Ref) error {
	complete := func(ctx context.Context) error {
		tx, ok := db.TxFrom(ctx)
		if !ok {
			return fmt.Errorf("refsync：完成 job 需要 transaction")
		}
		_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
		return err
	}
	res, err := s.syncOnce(ctx, ref, complete)
	if err != nil {
		if s.Logger != nil {
			s.Logger.ErrorContext(ctx, "media 引用同步失敗，會重試", "ref_type", ref.Type, "ref_id", ref.ID, "attempt", job.Attempt, "error", err)
		}
		return err
	}
	if res.Again {
		return river.JobSnooze(res.After)
	}
	return nil
}

// Sweep 是保底掃描：清掉過期的退場紀錄（模組範圍），並為版本比已同步的新、閒置超過 SweepIdle 的 ref 補排同步 job
// （job 遺失、被丟棄或重試用盡都會被補上；排入有唯一性，重複補排安全）。落後超過 MaxLag 記 error log，這是同步失效的監控訊號。
func (s *Syncer) Sweep(ctx context.Context) (int, error) {
	now := s.Now()
	q := s.Queries(s.DB.Conn(ctx))
	if err := q.PurgeRetired(ctx, now.Add(-HealDelay), now.Add(-TombstoneRetention)); err != nil {
		return 0, fmt.Errorf("refsync：清除退場紀錄：%w", err)
	}
	refs, err := q.UnsyncedRefs(ctx, now.Add(-SweepIdle), SweepBatch)
	if err != nil {
		return 0, fmt.Errorf("refsync：列出未同步的 ref：%w", err)
	}
	for _, r := range refs {
		if now.Sub(r.UpdatedAt) > MaxLag && s.Logger != nil {
			s.Logger.ErrorContext(ctx, "media 引用同步落後過久", "ref_type", r.Ref.Type, "ref_id", r.Ref.ID, "since", r.UpdatedAt)
		}
		if err := s.Enqueue(ctx, r.Ref); err != nil {
			return 0, err
		}
	}
	return len(refs), nil
}

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// NextRetry 是同步 job 第 attempt 次失敗之後的下一次重試時間：attempt^4 秒，最長 MaxBackoff。
func NextRetry(attemptedAt time.Time, attempt int) time.Time {
	backoff := MaxBackoff
	if attempt < 8 {
		if d := time.Duration(attempt*attempt*attempt*attempt) * time.Second; d < backoff {
			backoff = d
		}
	}
	return attemptedAt.Add(backoff)
}

// InsertOpts 是同步 job 的排入選項：以 (ref_type, ref_id) 去重，唯一鍵涵蓋還沒結束的狀態（含執行中，River 的規定）。執行期間
// 發生的改變不靠重新排入，而靠結算比對版本後再跑一輪；結算與完成同一個 transaction，job 遺失與重試用盡由 Sweep 補上。
func InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: MaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
			rivertype.JobStateRetryable, rivertype.JobStateScheduled,
		}},
	}
}

// SweepInsertOpts 把同一個週期內的排程與重啟合併。
func SweepInsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: SweepInterval}}
}
