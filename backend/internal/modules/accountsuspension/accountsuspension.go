// Package accountsuspension 是 accountsuspension 模組的公開 API：帳號停權與解除停權的唯一入口。
//
// 職責、不變量與鎖順序見同目錄的 AGENTS.md；行為規格見 docs/modules/accountsuspension.md。
package accountsuspension

import (
	"context"
	"log/slog"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Deps 是模組需要的外部依賴，全部必填。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Identity *identity.Module
	Seller   *seller.Module
	Logger   *slog.Logger
}

// Module 是 accountsuspension 模組。
type Module struct{ d Deps }

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Events == nil || d.Identity == nil || d.Seller == nil {
		panic("accountsuspension: missing dependency")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Module{d: d}
}

// Record 是停權的紀錄內容：理由類別、說明與來源參照。
type Record struct {
	// Category 是停權的理由類別，必須是 policy.Values() 的值。
	Category policy.Category
	// Reason 是理由的說明，選填。
	Reason string
	// SourceRef 是來源（例如 moderation 的案件）的不透明參照，選填。
	SourceRef string
}

// Suspension 是一筆停權紀錄。
type Suspension = identity.AccountSuspension

// Suspend 永久停權帳號，在呼叫方的 transaction 內執行（沒有就自己開一個）。順序：鎖帳號列、確認帳號沒有店家、寫停權紀錄與
// 停權狀態、發出 Suspended。帳號擁有店家回 FailedPrecondition 與 ReasonAccountHasStore，什麼都不寫；帳號已停權時回傳原本的
// 紀錄，不改理由、不發事件；帳號不存在或已刪除回 NotFound。
func (m *Module) Suspend(ctx context.Context, accountID int64, rec Record) (Suspension, error) {
	var out Suspension
	err := m.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := m.d.Identity.LockAccount(ctx, accountID); err != nil {
			return err
		}
		if err := m.requireNoStore(ctx, accountID); err != nil {
			return err
		}
		s, created, err := m.d.Identity.SuspendAccount(ctx, identity.SuspendInput{
			AccountID: accountID, Category: rec.Category, Reason: rec.Reason, SourceRef: rec.SourceRef,
		})
		if err != nil {
			return err
		}
		out = s
		if !created {
			return nil
		}
		return events.Publish(ctx, m.d.Events, Suspended{
			AccountID: accountID, SuspensionID: s.ID, Category: s.Category, SuspendedAt: s.SuspendedAt,
		})
	})
	return out, err
}

// Unsuspend 手動解除停權，在呼叫方的 transaction 內執行。reason 必填。帳號沒有停權時成功、不發事件，回傳 released=false。
func (m *Module) Unsuspend(ctx context.Context, accountID int64, reason string) (released bool, err error) {
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := m.d.Identity.LockAccount(ctx, accountID); err != nil {
			return err
		}
		s, ok, err := m.d.Identity.UnsuspendAccount(ctx, accountID, reason)
		if err != nil || !ok {
			return err
		}
		released = true
		return events.Publish(ctx, m.d.Events, Unsuspended{
			AccountID: accountID, SuspensionID: s.ID, Category: s.Category, ReleasedAt: *s.ReleasedAt,
		})
	})
	if err != nil {
		return false, err
	}
	return released, nil
}

// requireNoStore 擁有店家的帳號不能停權：停權對店家、未結訂單與撥款的效果由賣家條款決定，條款定出之前不停權。
// 呼叫方已持有帳號列鎖，所以與開店（同一把鎖）序列化：不會停權之後才冒出店家。
func (m *Module) requireNoStore(ctx context.Context, accountID int64) error {
	_, err := m.d.Seller.CurrentStore(ctx, accountID)
	switch {
	case err == nil:
		return apperr.FailedPrecondition("帳號擁有商店，不能停權").WithReason(apperr.ReasonAccountHasStore)
	case apperr.KindOf(err) == apperr.KindNotFound:
		return nil
	default:
		return err
	}
}
