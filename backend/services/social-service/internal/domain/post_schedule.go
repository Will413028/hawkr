package domain

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// cms_post.schedule_state 的合法值（56 §5.1.5 的 chk_cms_post_schedule_state）。
// NULL ＝ 從未被排程，因此沒有對應常數。
//
// 三個 cancelled_* 分開記而不是一個籠統的 cancelled：hidden_at 可被 RestorePost 清成
// NULL，事後靠反查 deleted_at／hidden_at 回答「這則排程為什麼沒發」，會在申訴成功之後
// 得到「看起來沒被下架過」的錯誤答案。終態值必須凍結取消當下的原因，因為它所依據的
// 欄位本身會變。
const (
	ScheduleStatePending               = "pending"
	ScheduleStateDone                  = "done"
	ScheduleStateExpired               = "expired"
	ScheduleStateCancelledByRevocation = "cancelled_by_revocation"
	ScheduleStateCancelledByDeletion   = "cancelled_by_deletion"
	ScheduleStateCancelledByGovernance = "cancelled_by_governance"
)

// 排程發布時點的合法區間（56 §6.1.9）。兩端都要擋，只擋過去時間等於同時接受「1 秒後」
// 與「10 年後」：前者製造與立即發布競態的排程列、後者製造永久待執行列，兩者都不報錯，
// 只會在很久以後才被發現。
//
// 這兩個值刻意不是 env：它們是對外契約的一部分（Gateway 與本服務各驗一次，兩處必須同值），
// 而 §8.5.1 的 env 表管的是 worker 的執行參數。
const (
	// SchedulePublishMinLead 之下的排程與立即發布沒有可觀察差別（worker tick 見 §8.5.1），
	// 卻要多付一整條排程路徑的失敗面；它同時讓「排程建立後還來得及撤回」真的成立。
	SchedulePublishMinLead = 5 * time.Minute
	// SchedulePublishMaxLead：排程列會佔著 schedule_state='pending' 直到執行，而工作階段
	// 最長 30 分鐘。
	SchedulePublishMaxLead = 90 * 24 * time.Hour
)

// 排程執行落稽核時用到的固定值，鏡射 CodeMeta（DelegationAuditAction 的使用服務欄只列
// account／gateway，故本服務端保留一份具名常數而不是散在呼叫點的字面值）。
const (
	DelegationActionPostPublish = "POST_PUBLISH"
	DelegationCapabilityPosts   = "posts.publish"
	DelegationResourceTypePost  = "post"
	// DelegationOriginScheduled 讓 account-service 改用排程的授權條件
	// （status <> 'revoked'，允許已到期）。它是提權欄位：account-service 只在內部
	// 授權簽章驗證通過且 serviceName == social-service 時才接受這個值。
	DelegationOriginScheduled = "scheduled"
)

// ScheduledPostRepository 是排程發布 worker 的全部資料庫出入口（56 §5.1.5 / §7.6.2）。
//
// 它與 PostRepository 分開，因為兩者的不變式不同：PostRepository 的寫入路徑一律要
// bump version，而這裡的終寫刻意不 bump（排程不覆蓋內容）；混在同一個介面裡，日後
// 新增方法的人會套錯那一條。
//
// 每個方法都回 rows affected 而不是把 0 轉成錯誤：這一組的 rows = 0 全部是預期結果
// （被別的 pod 接手、已被收斂成終態、已被刪除或下架），呼叫端據此決定要不要往下走，
// 不是失敗。
type ScheduledPostRepository interface {
	// ConvergeDeleted／ConvergeHidden／ConvergeExpired 是每個 tick 開頭的三條收斂，
	// 順序固定：刪除 → 下架 → 窗口。順序決定互斥性（下架那條帶 deleted_at IS NULL）
	// 也決定原因的具體度（逾窗口最不具體，排最後）。
	ConvergeDeleted(ctx context.Context, now time.Time) (int64, error)
	ConvergeHidden(ctx context.Context, now time.Time) (int64, error)
	ConvergeExpired(ctx context.Context, now time.Time, windowHours int) (int64, error)

	// ListDue 回傳本輪候選的貼文 id（不是整列）：claim 才是取值的地方，這裡取回的欄位
	// 到 claim 當下都可能已經變了。
	ListDue(ctx context.Context, now time.Time, windowHours, limit int) ([]int64, error)

	// Claim 以單列 CAS 取得租約。搶不到（或該列已不合資格）回 nil, nil。
	// leaseExpiresAt 由呼叫端逐筆計算，見 §29 不變式 4。
	Claim(ctx context.Context, id int64, owner string, now, leaseExpiresAt time.Time, windowHours int) (*gen.CmsPost, error)

	// Publish 執行終寫 CAS；只有 CAS 真的贏了才在**同一個交易內**執行 sameTx。
	// hashtag 計數的 +1 走這個 hook：計數與狀態轉換分屬兩個交易時，中間崩潰會留下
	// 一則已發布但標籤計數少一的貼文，而沒有任何對帳程序會發現。
	// 回傳 false ＝ CAS rows = 0，呼叫端必須放棄本次發布且不做任何後續動作。
	Publish(ctx context.Context, id int64, owner string, publishStatus int16, now time.Time,
		sameTx func(ctx context.Context, q gen.DBTX) error) (bool, error)

	// ReleaseLease 是失敗釋放：清租約、retry_count + 1、推遲下次嘗試。
	ReleaseLease(ctx context.Context, id int64, owner string, now, nextAttemptAt time.Time) (bool, error)
	// ExhaustRetries 是重試用盡時的終態轉換（OBS8 的 retry_exhausted）。
	ExhaustRetries(ctx context.Context, id int64, owner string, now time.Time) (bool, error)

	// IsLeaseHeldBy 是唯讀探針，用來分辨終寫 CAS 落空的兩個成因：租約被接手（不可寫終
	// 態，會擋掉接手者的結論）與述詞不成立（必須寫終態，否則稽核列永久 PENDING）。
	// 兩者在 rows = 0 上完全同形，而處置相反。
	IsLeaseHeldBy(ctx context.Context, id int64, owner string) (bool, error)
	// CancelByRevocation 對應 RecordAuditIntent 回 9271。
	CancelByRevocation(ctx context.Context, id int64, owner string, now time.Time) (bool, error)
}
