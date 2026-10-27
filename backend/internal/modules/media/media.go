// Package media 是 media 模組的公開 API：圖片等靜態資產的轉正與引用，以及擁有媒體的模組共用的引用同步。
package media

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/refsync"
)

// Uploader 是上傳暫存圖片的人。
type Uploader = model.Uploader

// Ref 是引用資產的業務列。
type Ref = model.Ref

// Asset 是轉正後的資產。
type Asset = model.Asset

// SecureAsset 是機敏資產的內容（ReadSecure 的結果）。
type SecureAsset = model.SecureAsset

// ErrNotSecure 表示 ReadSecure 的資產不是機敏資產。
var ErrNotSecure = model.ErrNotSecure

// Media 是其他模組對 media 的呼叫：轉正、登記引用、解除精確列出的引用，以及讀機敏資產。
type Media = model.Media

// 引用同步（internal/refsync）：擁有媒體的模組提供自己 schema 的 SQL（SyncQueries）與目前資產的讀取，演算法、seq 協定與
// 保底掃描只有這一份。
type (
	SyncQueries  = refsync.Queries
	SyncRef      = refsync.Ref
	SyncUnsynced = refsync.UnsyncedRef
	RefSyncer    = refsync.Syncer
	SyncResult   = refsync.Result
	// SyncCandidate 是要寫回 ref 的資產與它的輸入欄位（SyncReject 的參數）。
	SyncCandidate = refsync.Candidate
)

// 同步的時間參數與重試次數。
const (
	SyncMaxAttempts    = refsync.MaxAttempts
	SyncJobTimeout     = refsync.JobTimeout
	SyncResyncDelay    = refsync.ResyncDelay
	SyncHealDelay      = refsync.HealDelay
	SyncSweepInterval  = refsync.SweepInterval
	SyncSweepIdle      = refsync.SweepIdle
	SyncMaxLag         = refsync.MaxLag
	TombstoneRetention = refsync.TombstoneRetention
)

// SyncChanged 在寫入 ref 的業務列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job。
func SyncChanged(ctx context.Context, q SyncQueries, enqueue func(context.Context, Ref) error, now time.Time, ref Ref, retired ...string) error {
	return refsync.Changed(ctx, q, enqueue, now, ref, retired...)
}

// SyncReject 拒絕把已退場的資產寫回 ref（InvalidInput）；必須在鎖住業務列的 transaction 內呼叫。
func SyncReject(ctx context.Context, q SyncQueries, ref Ref, candidates ...SyncCandidate) error {
	return refsync.Reject(ctx, q, ref, candidates...)
}

// SyncWork 是各模組的同步 job 共用的 Work：需要再跑一輪時 snooze，失敗時記 log 並回錯誤讓 River 重試，一切就緒時在結算的
// transaction 內完成 job。
func SyncWork[A river.JobArgs](ctx context.Context, s *RefSyncer, job *river.Job[A], ref Ref) error {
	return refsync.Work(ctx, s, job, ref)
}

// SyncNextRetry 是各模組的同步 job 共用的重試時間（退避封頂，見 refsync.MaxBackoff）。
func SyncNextRetry(attemptedAt time.Time, attempt int) time.Time {
	return refsync.NextRetry(attemptedAt, attempt)
}

// SyncInsertOpts 是同步 job 的排入選項：以 (ref_type, ref_id) 去重，最多重試 SyncMaxAttempts 次。
func SyncInsertOpts() river.InsertOpts { return refsync.InsertOpts() }

// SyncSweepInsertOpts 是保底掃描 job 的排入選項。
func SyncSweepInsertOpts() river.InsertOpts { return refsync.SweepInsertOpts() }
