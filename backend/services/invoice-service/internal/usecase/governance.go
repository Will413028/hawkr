package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.gov —— 治理到期掃描 ＋ 跨期關帳窗口唯讀掃描（§7.5.3、§8.2.3 M20、§4.5.10）
//
// 兩件事，都是「時間到了就轉態／就數一次」的集合式操作，不打第三方：
//   C4  資格逾期：SELF_ISSUE_PENDING／SUPPLEMENT_REQUIRED 且 pending_valid_until
//       到期 ⇒ EXPIRED（T10），並寫入重新申請冷卻
//   M20 跨期關帳窗口：唯讀計數，不改任何狀態（§4.5.10 明訂）
//
// 🔴 **C2（停權 14 天自動恢復）刻意不在這裡**。設計書兩處對它的歸屬互相矛盾
// （§8.5.1 A-4 說由 seller-service 既有 cron 執行、§乙丙類說是 INV.gov 的職責），
// 而實查 seller-service：`LiftSellerSuspension` 的唯一呼叫端是
// `LiftSellerSuspensionForAdmin`（gRPC 管理動作）、`seller_suspensions` 沒有
// `expires_at`、`internal/worker/` 沒有 governance 排程 ⇒ **那支 cron 不存在**。
// 三個出路與各自代價寫在 backend#2969 的 comment，需要跨服務裁決，不由本層決定。
// 這裡留空是刻意的——假裝涵蓋它比留空更糟。
//
// 因為沒有外部副作用，多副本同時執行是安全的（第二個副本 rows=0）⇒ 依
// 《多副本排程搶佔規範》規則 ①（約束的是「副作用之前」）**不需要 claim**。
// 它也不比對兩份資料、不回填、不修正，所以不是 drift scanner。
// ---------------------------------------------------------------------------

// govBatch 是單輪處理上限（§7.5.3 FetchBatch = 500）。
const govBatch = 500

// reapplyCooldown 是 EXPIRED 之後的重新申請冷卻（§9.3.3 REAPPLY_COOLDOWN_DAYS = 7）。
const reapplyCooldown = 7 * 24 * time.Hour

// CrossPeriodWindow 是 M20 的 label 值域（§8.2.3 M20 逐字三值）。
type CrossPeriodWindow string

const (
	// WindowOpen ＝ 台北時間單月 1–2 日：還有時間處理。
	WindowOpen CrossPeriodWindow = "open"
	// WindowClosing ＝ 單月 3–5 日：關帳日將至。
	WindowClosing CrossPeriodWindow = "closing"
	// WindowClosed ＝ 單月 6 日起至下一期開始：已過關帳日，作廢會被 3050126 擋掉。
	WindowClosed CrossPeriodWindow = "closed"
)

// crossPeriodWindow 依**台北時間的日**判定窗口（§8.2.3 M20 逐字）。
//
// 🔴 **判定放在產生指標的這一端，不放進 PromQL**：PromQL 沒有時區安全的日曆函式，
// 寫成 `hour`／`day_of_month` 會用 UTC 算，**每兩個月的邊界日剛好錯 8 小時**
// ——而那正是本指標唯一有意義的那幾天。
//
// 🔴 只有**單月**（奇數月，雙月制的期初）才有窗口語意；雙月一律 closed
// ——上一期的關帳日早就過了。
func crossPeriodWindow(now time.Time) CrossPeriodWindow {
	lt := now.In(timeutil.LocationTaipei)
	if int(lt.Month())%2 == 0 {
		return WindowClosed
	}
	switch d := lt.Day(); {
	case d <= 2:
		return WindowOpen
	case d <= 5:
		return WindowClosing
	default:
		return WindowClosed
	}
}

// GovernanceStore 是 INV.gov 需要的 repository 面。
type GovernanceStore interface {
	// ExpirePending 是 C4（T10）：到期的 pending 資格轉 EXPIRED 並寫入冷卻。
	ExpirePending(ctx context.Context, now, cooldownUntil time.Time, limit int32) ([]domain.SellerTaxProfile, error)

	// CountApprovalRecheckDue 是「屆期待複審」的家數（#3261 ③）。
	//
	// 🔴 `approval_valid_until` 屆滿**不改狀態**（該查詢的 SQL 註解逐字：「DB 就是 SSOT，
	// 不需要 worker 改狀態、不需要事件、更不需要『已通知』旗標」）⇒ 除了這支查詢之外
	// 沒有第二個訊號來源。在 2026-09-08 之前它零呼叫端、且 metric 名在全 repo 零命中
	// ⇒ 賣家的稅務資格到期了，沒有任何東西會知道。
	CountApprovalRecheckDue(ctx context.Context, now time.Time) (int64, error)
}

// InboxPayloadPurger 是 §5.4.6 的資料保存那一半：90 天後只清 payload，去重鍵永久保留。
//
// 🔴 用窄介面而不是整個 InboxRepository：INV.gov 只需要這一件事，而那份介面有 20 幾支。
// 可以是 nil（本地 wiring 缺件時跳過），與 businessDays／holidays 同一個慣例。
type InboxPayloadPurger interface {
	PurgeProcessedPayloads(ctx context.Context, now, cutoff time.Time, limit int32) (int64, error)
}

// OutboxCleaner 是 outbox 的 30 天保留（§5.4.6 表格第一列）。
//
// 🔴 直接用 domain 既有的 DeleteDoneBefore 而不是 pkg/outbox.CleanupDone：後者要一個
// 簽章不同的 MonitorRepo，而 outbox_worker.go 那份 adapter 的 DeleteDoneBefore 實作好了
// 卻沒有人呼叫（唯一的呼叫端 RefreshMetrics 只做 obs.Refresh，而 Refresh 只 Set gauge、
// 不刪任何列）。多繞一層 adapter 只是把同一個坑換個位置。
type OutboxCleaner interface {
	DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int32) (int64, error)
}

// CrossPeriodCounter 回「發票日期屬已結束期別、且仍有非終態調整 operation」的筆數。
type CrossPeriodCounter interface {
	CountCrossPeriodPending(ctx context.Context, now time.Time) (int64, error)
}

// GovernanceObserver 收下一輪的觀測值。
type GovernanceObserver interface {
	ObserveCrossPeriodPending(window CrossPeriodWindow, count int64)
	// ObserveApprovalRecheckDue 推 invoice_seller_approval_recheck_due（#3261 ③）。
	ObserveApprovalRecheckDue(count int64)
}

// HolidaySource 是行事曆的 owner 端（shipping-service 的 holidays 表）。
//
// 介面放在這裡而不是 domain：它是**出站依賴**不是儲存層，而 usecase 只需要
// 「給我這段區間的假日」這一件事。
type HolidaySource interface {
	GetHolidays(ctx context.Context, from, to time.Time) ([]time.Time, error)
}

// Governance 是 INV.gov 的 usecase。
type Governance struct {
	profiles GovernanceStore
	counter  CrossPeriodCounter
	obs      GovernanceObserver
	// businessDays／holidays 是行事曆鏡像的兩端。**兩個都可以是 nil**：
	// 本地開發起不了 shipping-service 時 wiring 會傳 nil，此時整段同步跳過並記一行
	// warn，其餘治理工作照跑。零值 BusinessCalendar 是安全側（偏晚的硬上限）。
	businessDays domain.BusinessDayRepository
	holidays     HolidaySource
	// inbox／outboxRows 是 §5.4.6 的兩張清理表。**兩個都可以是 nil**，理由同上。
	inbox      InboxPayloadPurger
	outboxRows OutboxCleaner
	tx         database.TxManager
	log        *zap.Logger
	now        func() time.Time
}

// WithRetention 掛上 §5.4.6 的兩支清理（#3261 ①②）。
//
// 用 setter 而不是加建構參數：NewGovernance 已有 8 個位置參數，再加兩個會讓每個呼叫端
// 都得數位置；而這兩支是可選的（nil 即跳過）。
func (u *Governance) WithRetention(inbox InboxPayloadPurger, outboxRows OutboxCleaner) *Governance {
	u.inbox = inbox
	u.outboxRows = outboxRows
	return u
}

// §5.4.6 的兩個保留期。數字逐字取自設計書該節的表格，改這裡就要改那裡。
//
//	invoice_inbox_events   只清 payload，status IN ('PROCESSED','SKIPPED') 且 processed_at < now-90d
//	invoice_outbox_events  整列刪，status='done' 且 completed_at < now-30d
const (
	inboxPayloadRetention = 90 * 24 * time.Hour
	outboxDoneRetention   = 30 * 24 * time.Hour
	// retentionBatch 讓單輪耗時有上界（兩支都是單句 SQL，批次上限即交易大小）。
	retentionBatch int32 = 500
)

// NewGovernance 建 INV.gov usecase。
func NewGovernance(
	profiles GovernanceStore,
	counter CrossPeriodCounter,
	obs GovernanceObserver,
	businessDays domain.BusinessDayRepository,
	holidays HolidaySource,
	tx database.TxManager,
	log *zap.Logger,
	now func() time.Time,
) *Governance {
	return &Governance{
		profiles: profiles, counter: counter, obs: obs,
		businessDays: businessDays, holidays: holidays, tx: tx,
		log: orNop(log), now: now,
	}
}

// RunOnce 跑一輪治理掃描。
func (u *Governance) RunOnce(ctx context.Context) error {
	now := u.now()

	// C4：資格逾期。集合式 UPDATE，無外部副作用 ⇒ 不需要 claim。
	expired, err := u.profiles.ExpirePending(ctx, now, now.Add(reapplyCooldown), govBatch)
	if err != nil {
		u.log.Error("invoice.gov.expire_pending_failed", zap.Error(err))
	}

	// M20：跨期關帳窗口唯讀計數。**不改任何狀態**（§4.5.10）。
	//
	// 🔴 三個 window 每一輪都要 Set，不能只 Set 當下那一個：其餘兩個會留在上一輪的
	// 值，而 Prometheus 分不出「這一格現在是 0」與「這一格沒被更新」。
	count, cErr := u.counter.CountCrossPeriodPending(ctx, now)
	if cErr != nil {
		u.log.Error("invoice.gov.cross_period_count_failed", zap.Error(cErr))
	} else {
		active := crossPeriodWindow(now)
		for _, w := range []CrossPeriodWindow{WindowOpen, WindowClosing, WindowClosed} {
			if w == active {
				u.obs.ObserveCrossPeriodPending(w, count)
			} else {
				u.obs.ObserveCrossPeriodPending(w, 0)
			}
		}
	}

	// #3261 ③：屆期待複審家數。與 M20 同樣是**唯讀計數、不改狀態**。
	//
	// 🔴 每一輪都要 Set（哪怕是 0）：Prometheus 分不出「這一格現在是 0」與
	// 「這一格沒被更新」，理由與上面 M20 的紅字逐字相同。
	if due, dErr := u.profiles.CountApprovalRecheckDue(ctx, now); dErr != nil {
		u.log.Error("invoice.gov.approval_recheck_count_failed", zap.Error(dErr))
	} else {
		u.obs.ObserveApprovalRecheckDue(due)
	}

	// #3261 ①②：§5.4.6 的兩張清理表。**各自獨立、任何一支的錯誤都不得吃掉後面那支**
	// （同 INV.dl 的 K04／K05／K06 編排）。兩者都是集合式 UPDATE／DELETE、無外部副作用
	// ⇒ 不需要 claim（《多副本排程搶佔規範》§一：無副作用的集合式操作不在搶佔範圍）。
	//
	// 🔴 inbox 是**只清 payload 不刪列**：去重鍵 (event_type, business_dedup_key) 一旦被刪，
	// RefundSyncWorker 的重發就變成「全新的事件」⇒ 對同一筆退款開出第二張折讓單，
	// 而折讓單是法定憑證（設計書 §5.4.6 的 I-49 紅字）。這條保證在 SQL 裡，本處只是呼叫。
	if u.inbox != nil {
		n, pErr := u.inbox.PurgeProcessedPayloads(ctx, now, now.Add(-inboxPayloadRetention), retentionBatch)
		switch {
		case pErr != nil:
			u.log.Error("invoice.gov.inbox_payload_purge_failed", zap.Error(pErr))
		case n > 0:
			u.log.Info("invoice.gov.inbox_payloads_purged", zap.Int64("count", n))
		}
	}
	if u.outboxRows != nil {
		n, cErr := u.outboxRows.DeleteDoneBefore(ctx, now.Add(-outboxDoneRetention), retentionBatch)
		switch {
		case cErr != nil:
			u.log.Error("invoice.gov.outbox_cleanup_failed", zap.Error(cErr))
		case n > 0:
			u.log.Info("invoice.gov.outbox_done_deleted", zap.Int64("count", n))
		}
	}

	// 行事曆鏡像每日整段覆蓋（§5.1.14）。放在最後：它會打一次跨服務 RPC，
	// 失敗不得影響前面兩件純 DB 的工作。
	syncErr := u.syncBusinessDayMirror(ctx, now)

	u.log.Info("invoice.gov.scanned",
		zap.Int("expired_profiles", len(expired)),
		zap.String("cross_period_window", string(crossPeriodWindow(now))),
		zap.Int64("cross_period_pending", count))
	if err != nil {
		return err
	}
	if cErr != nil {
		return cErr
	}
	return syncErr
}

// syncBusinessDayMirror 把 shipping-service 的假日整段覆蓋進本地鏡像（§5.1.14）。
//
// 🔴 **整段 DELETE + INSERT 必須在同一個交易裡**，而且 RPC 要在交易**之外**先拿到
// 資料。反過來做（先開交易、交易內打 RPC）會讓一次 8 秒的 RPC 逾時把 DELETE 之後的
// 空表暴露給併發的申訴建立——而空表在 BusinessCalendar 那端是零涵蓋 ⇒ 那幾筆申訴的
// SLA 全部落在 14 天硬上限。
//
// 🔴 **RPC 失敗一律保留舊鏡像**：昨天的行事曆比空表準得多。這一條是本函式存在的
// 全部理由——同步器的價值不在「同步成功」，在「失敗時不要把東西弄壞」。
//
// ⚠️ 這支同步器在 2026-09-04 之前**不存在**（#3089 第 17 項）：DeleteAll／Insert
// 兩個方法零呼叫端，鏡像自建表以來一列都沒有 ⇒ 每一筆申訴的 SLA 都是 14 自然日
// 而不是 3 工作天，而那個退化是**靜默**的（ComplaintSlaDueAt 恆有回傳值）。
func (u *Governance) syncBusinessDayMirror(ctx context.Context, now time.Time) error {
	if u.businessDays == nil || u.holidays == nil || u.tx == nil {
		u.log.Warn("invoice.gov.calendar_sync_skipped",
			zap.Bool("has_repo", u.businessDays != nil),
			zap.Bool("has_source", u.holidays != nil))
		return nil
	}

	from, to := domain.BusinessDayMirrorWindow(now)
	rows, err := u.holidays.GetHolidays(ctx, from, to)
	if err != nil {
		// 保留舊鏡像。這裡回 error 讓 RunOnce 的回傳值帶著它，排程器那層會 log。
		u.log.Error("invoice.gov.calendar_fetch_failed", zap.Error(err))
		return err
	}

	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, dErr := u.businessDays.DeleteAll(ctx); dErr != nil {
			return dErr
		}
		for _, d := range rows {
			if iErr := u.businessDays.Insert(ctx, d, now); iErr != nil {
				return iErr
			}
		}
		return nil
	})
	if err != nil {
		u.log.Error("invoice.gov.calendar_write_failed", zap.Error(err), zap.Int("holidays", len(rows)))
		return err
	}

	u.log.Info("invoice.gov.calendar_synced",
		zap.Int("holidays", len(rows)),
		zap.String("from", from.Format("2006-01-02")),
		zap.String("to", to.Format("2006-01-02")))
	return nil
}
