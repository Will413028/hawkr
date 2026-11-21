// Package project 是消費統計的投影（過渡：ordering 搬進單體後改讀 ordering 的 pub_*，這個套件與 order_nets、ledger 退場）。
// 兩個事件（order.completed、aftersales.refund.completed）各一個 transaction：守門（買家 LockNotClosed、店家 LockNotRetired，擁有者列的 FOR KEY SHARE）→
// 鎖住該單的 order_nets 列 → ledger 去重 → 更新淨額 → 把這張單貢獻的變化量加到客戶。
// 兩個 Record 方法的 recorded 為 true 表示事件是新的（ledger 新增一列），即使它沒有改變任何統計（同單號的第二個 order.completed、已歸零後的全額退款）。
package project

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	prefixOrder  = "order:"
	prefixRefund = "refund:"
)

// Buyers 是投影的買家守門（buyer 公開 API）：transaction 內以 FOR KEY SHARE 確認買家尚未關閉，已關閉與不存在回 NotFound。
type Buyers interface {
	LockNotClosed(ctx context.Context, buyerID int64) error
}

// Stores 是投影的店家守門（seller 公開 API）：transaction 內以 FOR KEY SHARE 確認店家尚未退場，已退場與不存在回 NotFound。
type Stores interface {
	LockNotRetired(ctx context.Context, storeID int64) error
}

// Deps 是投影的依賴。
type Deps struct {
	DB     *db.DB
	Buyers Buyers
	Stores Stores
	Now    func() time.Time
	Logger *slog.Logger
}

// Service 執行投影。
type Service struct{ d Deps }

// New 建立投影。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w：%s", model.ErrInvalidEvent, fmt.Sprintf(format, args...))
}

func checkCommon(key, orderNo string, storeID, buyerID int64) error {
	switch {
	case key == "" || len(key) > model.MaxEventKeyLen:
		return invalid("冪等鍵為空或超過 %d", model.MaxEventKeyLen)
	case orderNo == "" || utf8.RuneCountInString(orderNo) > model.MaxOrderNoLen:
		return invalid("單號為空或超過 %d", model.MaxOrderNoLen)
	case storeID <= 0 || buyerID <= 0:
		return invalid("店家 %d 或買家 %d 不是正整數", storeID, buyerID)
	}
	return nil
}

// RecordOrderCompleted 記一筆完成的訂單：以 event_id 去重（重放回 recorded=false）。買家或店家已刪帳、退場時不寫任何東西（recorded=false）。
func (s *Service) RecordOrderCompleted(ctx context.Context, e model.OrderCompleted) (bool, error) {
	if err := checkCommon(e.EventID, e.OrderNo, e.StoreID, e.BuyerID); err != nil {
		return false, err
	}
	if e.Amount < 0 {
		return false, invalid("訂單金額 %d 為負", e.Amount)
	}
	return s.record(ctx, prefixOrder+e.EventID, e.OrderNo, e.StoreID, e.BuyerID, func(old state) step {
		return onOrderCompleted(old, e.Amount)
	}, true)
}

// RecordRefund 記一筆完成的退款：以 Refund.Key 去重。退款不建立客戶列（客戶只在首筆完成訂單時建立）；退款先到時只記在這單的淨額，
// 訂單到時一併算進去。
func (s *Service) RecordRefund(ctx context.Context, r model.Refund) (bool, error) {
	if err := checkCommon(r.Key, r.OrderNo, r.StoreID, r.BuyerID); err != nil {
		return false, err
	}
	if !r.Outcome.Valid() {
		return false, invalid("未知的退款結果 %q", r.Outcome)
	}
	if r.Outcome == model.RefundPartial && r.Amount <= 0 {
		return false, invalid("部分退款金額 %d 必須為正", r.Amount)
	}
	return s.record(ctx, prefixRefund+r.Key, r.OrderNo, r.StoreID, r.BuyerID, func(old state) step {
		return onRefund(old, r.Outcome, r.Amount)
	}, false)
}

// skip 記下守門回 NotFound 的那一方：transaction 內只記這個、提交空的 transaction，分辨原因的讀取在 transaction 之後。
type skip struct {
	party string
	id    int64
	err   error
}

// isNotFound 判斷守門的錯誤是不是 NotFound（買家已關閉、店家已退場，或 ID 從未存在）：是就「這筆事件不寫」，其他錯誤照常回傳（fail-closed）。
func isNotFound(err error) bool { return apperr.KindOf(err) == apperr.KindNotFound }

// logSkip 在 transaction 提交之後（不在其內：分辨查詢失敗會讓 Postgres 中止 transaction，使空提交失敗而事件被重試）多讀一次 pub 函式分辨原因，
// 只在略過路徑執行（正常路徑沒有額外讀取）：已關閉、已退場是正常的重放，記 info；不存在只可能是上游的壞資料（買家與店家列關閉、退場後仍保留），
// 記 error 讓它看得見。分辨查詢失敗時記 error（帶該錯誤）並照樣略過，不讓事件失敗。
func (s *Service) logSkip(ctx context.Context, k skip, orderNo string) {
	q := store.New(s.d.DB.Conn(ctx))
	var inactive bool
	var qerr error
	if k.party == "買家" {
		inactive, qerr = q.BuyerClosed(ctx, k.id)
	} else {
		inactive, qerr = q.StoreRetired(ctx, k.id)
	}
	switch {
	case qerr != nil:
		s.d.Logger.ErrorContext(ctx, "crm：投影事件的"+k.party+"守門回 NotFound，分辨已停用或不存在的查詢失敗，略過", "party_id", k.id, "order_no", orderNo, "error", qerr)
	case inactive:
		s.d.Logger.InfoContext(ctx, "crm：投影事件的"+k.party+"已停用，略過", "party_id", k.id, "order_no", orderNo)
	default:
		s.d.Logger.ErrorContext(ctx, "crm：投影事件的"+k.party+"不存在，略過", "party_id", k.id, "order_no", orderNo, "error", k.err)
	}
}

func (s *Service) record(ctx context.Context, dedupKey, orderNo string, storeID, buyerID int64, apply func(old state) step, createCustomer bool) (bool, error) {
	recorded := false
	var skipped *skip
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		recorded, skipped = false, nil
		q := s.q(ctx)
		// 守門鎖是 transaction 的第一組語句（買家先於店家，列的 FOR KEY SHARE，持有到 transaction 結束）：擁有者轉移（關閉買家、店家退場）的
		// FOR UPDATE 要等這個 transaction 提交，所以抹除 job 看不到之後才提交的投影；已關閉或已退場時事件不寫任何東西。
		// 守門鎖在 crm 自己的列（order_nets、customers）之前取，lockorder 檢查器機械驗證順序。
		if gerr := s.d.Buyers.LockNotClosed(ctx, buyerID); gerr != nil {
			if !isNotFound(gerr) {
				return gerr
			}
			skipped = &skip{party: "買家", id: buyerID, err: gerr}
			return nil
		}
		if gerr := s.d.Stores.LockNotRetired(ctx, storeID); gerr != nil {
			if !isNotFound(gerr) {
				return gerr
			}
			skipped = &skip{party: "店家", id: storeID, err: gerr}
			return nil
		}
		now := s.d.Now().UTC()
		old, err := q.LockOrderNet(ctx, store.LockOrderNetParams{OrderNo: orderNo, StoreID: storeID, BuyerID: buyerID, Now: now})
		if err != nil {
			return err
		}
		if old.StoreID != storeID || old.BuyerID != buyerID {
			return fmt.Errorf("%w：單 %s 已記錄買家 %d、店家 %d，事件是買家 %d、店家 %d", model.ErrPartiesMismatch, orderNo, old.BuyerID, old.StoreID, buyerID, storeID)
		}
		n, err := q.InsertLedger(ctx, store.InsertLedgerParams{DedupKey: dedupKey, OrderNo: orderNo})
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		recorded = true
		st := apply(state{Completed: old.Completed, FullRefunded: old.FullRefunded, Net: old.NetAmount})
		if !st.Changed {
			return nil
		}
		if err := q.SetOrderNet(ctx, store.SetOrderNetParams{Completed: st.Next.Completed, FullRefunded: st.Next.FullRefunded, NetAmount: st.Next.Net, Now: now, OrderNo: orderNo}); err != nil {
			return err
		}
		if createCustomer {
			if err := q.UpsertCustomerOnOrder(ctx, store.UpsertCustomerOnOrderParams{StoreID: storeID, BuyerID: buyerID, Delta: st.Delta, Now: now}); err != nil {
				return err
			}
		} else if st.Delta != 0 {
			rows, err := q.AddCustomerSpent(ctx, store.AddCustomerSpentParams{Delta: st.Delta, Now: now, StoreID: storeID, BuyerID: buyerID})
			if err != nil {
				return err
			}
			if rows == 0 {
				return errors.New("crm：訂單已完成但客戶列不存在")
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if skipped != nil {
		s.logSkip(ctx, *skipped, orderNo)
		return false, nil
	}
	return recorded, nil
}
