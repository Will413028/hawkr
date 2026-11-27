package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

const govBatch = 500

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

// Governance 是 INV.gov 的 usecase。
type Governance struct {
	profiles   GovernanceStore
	counter    CrossPeriodCounter
	obs        GovernanceObserver
	inbox      InboxPayloadPurger
	outboxRows OutboxCleaner
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

const (
	inboxPayloadRetention       = 90 * 24 * time.Hour
	outboxDoneRetention         = 30 * 24 * time.Hour
	retentionBatch        int32 = 500
)

// NewGovernance 建 INV.gov usecase。
func NewGovernance(
	profiles GovernanceStore,
	counter CrossPeriodCounter,
	obs GovernanceObserver,
	log *zap.Logger,
	now func() time.Time,
) *Governance {
	return &Governance{
		profiles: profiles, counter: counter, obs: obs,
		log: orNop(log), now: now,
	}
}

// RunOnce 跑一輪治理掃描。
func (u *Governance) RunOnce(ctx context.Context) error {
	now := u.now()

	expired, err := u.profiles.ExpirePending(ctx, now, now.Add(reapplyCooldown), govBatch)
	if err != nil {
		u.log.Error("invoice.gov.expire_pending_failed", zap.Error(err))
	}

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

	if due, dErr := u.profiles.CountApprovalRecheckDue(ctx, now); dErr != nil {
		u.log.Error("invoice.gov.approval_recheck_count_failed", zap.Error(dErr))
	} else {
		u.obs.ObserveApprovalRecheckDue(due)
	}

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

	u.log.Info("invoice.gov.scanned",
		zap.Int("expired_profiles", len(expired)),
		zap.String("cross_period_window", string(crossPeriodWindow(now))),
		zap.Int64("cross_period_pending", count))
	if err != nil {
		return err
	}
	return cErr
}
