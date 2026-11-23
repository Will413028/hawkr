package charge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	// failedTransferUnresolved 是轉帳「呼叫過、結果不明」超過期限的終局原因（transfers.failure_reason）；failedRefused 同建立付款。
	failedTransferUnresolved = "unresolved_transfer"
	// failedReversalUnresolved 是撤回「呼叫過、結果不明」超過期限的終局原因（refunds.failure_reason）。
	failedReversalUnresolved = "unresolved_reversal"
)

// RequestTransfer 是訂單完成後把這筆付款的款項轉給賣家的入口（D6；步驟 4 的 GrantPayment 呼叫）：在一個 transaction 內鎖付款列、確認已入帳，
// 寫一筆 PENDING 轉帳（金額是付款金額扣掉非失敗退款，賣家收款帳戶的外部 ID 寫成快照；帳戶必須已開通轉帳）並排入轉帳 job；provider 在 transaction 外由 job 呼叫（規則 5）。
// 冪等：已有有效（PENDING 或 SUCCEEDED）轉帳時回傳它，由唯一索引 transfers_active_payment 守，不寫第二筆；只有沒有有效轉帳時，付款未入帳、賣家帳戶未開通
// 或沒有可轉的金額才是 FailedPrecondition（FAILED 的轉帳不佔位，之後再呼叫會建新的一筆、換新鍵）。付款不存在回 NotFound。呼叫方不能在 transaction 內呼叫。
func (s *Service) RequestTransfer(ctx context.Context, orderNo string) (model.Transfer, error) {
	if err := s.requireProvider(); err != nil {
		return model.Transfer{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.Transfer{}, errInTransaction
	}
	orderNo = strings.TrimSpace(orderNo)
	var out store.PaymentTransfer
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		pay, err := q.LockPaymentByOrderNo(ctx, orderNo)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這筆付款")
		}
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		amount, destination, refusal := s.transferTerms(ctx, q, pay)
		if refusal == nil {
			n, err := q.InsertTransfer(ctx, store.InsertTransferParams{ID: s.d.NewID(), PaymentID: pay.ID, StoreID: pay.StoreID, Amount: amount, DestinationAccountID: destination, Now: now})
			if err != nil {
				return err
			}
			if n == 1 {
				t, err := q.ActiveTransferByPayment(ctx, pay.ID)
				if err != nil {
					return err
				}
				out = t
				return enqueueTransfer(ctx, s.d.Jobs, t.ID)
			}
		}
		t, err := q.ActiveTransferByPayment(ctx, pay.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			if refusal != nil {
				return refusal
			}
			return fmt.Errorf("charge: 付款 %s 寫轉帳沒有成功卻也沒有有效的轉帳", pay.ID)
		}
		out = t
		return err
	})
	if err != nil {
		return model.Transfer{}, err
	}
	return toTransfer(out), nil
}

// transferTerms 算出這筆付款現在可以轉的金額與賣家帳戶的外部 ID；不能轉時回 FailedPrecondition。
func (s *Service) transferTerms(ctx context.Context, q *store.Queries, pay store.PaymentPayment) (int64, string, error) {
	if pay.Status != string(model.PaymentSucceeded) {
		return 0, "", apperr.FailedPrecondition("付款尚未入帳，不能轉帳")
	}
	acct, err := q.PaymentAccountByStore(ctx, pay.StoreID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!acct.TransfersActive || acct.ExternalAccountID == nil)) {
		return 0, "", apperr.FailedPrecondition("賣家的收款帳戶尚未開通")
	}
	if err != nil {
		return 0, "", err
	}
	refunded, err := q.NonFailedRefundTotal(ctx, pay.ID)
	if err != nil {
		return 0, "", err
	}
	if pay.Amount-refunded <= 0 {
		return 0, "", apperr.FailedPrecondition("付款已全額退款，沒有可轉的金額")
	}
	return pay.Amount - refunded, *acct.ExternalAccountID, nil
}

func toTransfer(r store.PaymentTransfer) model.Transfer {
	t := model.Transfer{
		ID: r.ID, PaymentID: r.PaymentID, StoreID: r.StoreID, Amount: r.Amount, ReversedAmount: r.ReversedAmount,
		Status: model.TransferStatus(r.Status), CreatedAt: r.CreatedAt,
	}
	if r.FailureReason != nil {
		t.FailureReason = *r.FailureReason
	}
	return t
}

// Transfer 是轉帳 job（payment.transfer_create）的內容：列還是 PENDING 時，呼叫 provider 之前先記 called_at（獨立提交），帶列上的冪等鍵與付款的 charge
// 建立轉帳（source_transaction），成功在新的 transaction 寫外部 ID 與 SUCCEEDED。已不是 PENDING 的列什麼都不做。provider 永久拒絕進終局 FAILED（job 完成，不回錯誤）；
// 呼叫過、沒有外部 ID 超過 UnresolvedDeadline 的列進終局 FAILED，不再重送。平台餘額不足與其他暫時性失敗回錯誤由 River 重試（不是終局，spikes.md #8），
// final 為 true 時只記警示 log 並回 nil，留給週期掃描。provider 回的金額或外部 ID 與列不符回 ErrAnomaly（不寫入、job 取消）。
func (s *Service) Transfer(ctx context.Context, transferID uuid.UUID, final bool) error {
	err := s.transfer(ctx, transferID)
	if err != nil && final && !errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：轉帳用完重試，留給週期掃描", "transfer_id", transferID.String(), "err", err)
		return nil
	}
	return err
}

func (s *Service) transfer(ctx context.Context, transferID uuid.UUID) error {
	if err := s.requireProvider(); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.TransferByID(ctx, transferID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.Status != string(model.TransferPending) {
		return nil
	}
	pay, err := q.PaymentByID(ctx, row.PaymentID)
	if err != nil {
		return err
	}
	if pay.ExternalChargeID == nil {
		return fmt.Errorf("%w：轉帳 %s 的付款沒有 charge", ErrAnomaly, transferID)
	}
	now := s.d.Now().UTC()
	if row.CalledAt != nil && now.Sub(*row.CalledAt) > UnresolvedDeadline {
		s.d.Logger.ErrorContext(ctx, "payment：轉帳呼叫過、結果不明超過期限，進終局", "transfer_id", transferID.String(), "order_no", pay.OrderNo)
		return s.failTransfer(ctx, transferID, failedTransferUnresolved)
	}
	called, err := s.markTransferCalled(ctx, transferID, now)
	if err != nil || called == nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := s.d.Provider.CreateTransfer(callCtx, TransferInput{
		Amount: called.Amount, SourceChargeID: *pay.ExternalChargeID, DestinationAccountID: called.DestinationAccountID,
		TransferGroup: pay.OrderNo, IdempotencyKey: called.IdempotencyKey.String(),
	})
	if errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕建立轉帳", "transfer_id", transferID.String(), "order_no", pay.OrderNo, "err", err)
		return s.failTransfer(ctx, transferID, failedRefused)
	}
	if err != nil {
		return err
	}
	if snap.ExternalID == "" || snap.Amount != called.Amount {
		s.d.Logger.ErrorContext(ctx, "payment：provider 回的轉帳與列不一致，不寫入", "transfer_id", transferID.String(), "order_no", pay.OrderNo, "external_transfer_id", snap.ExternalID)
		return fmt.Errorf("%w：轉帳的外部 ID 或金額與列不同", ErrAnomaly)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPayment(ctx, row.PaymentID); err != nil {
			return err
		}
		_, err := q.SucceedTransfer(ctx, store.SucceedTransferParams{ID: transferID, ExternalTransferID: &snap.ExternalID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

// markTransferCalled 在獨立的 transaction 記下「呼叫過」的時刻（呼叫 provider 之前提交）；列已不是 PENDING 時回 nil。
func (s *Service) markTransferCalled(ctx context.Context, transferID uuid.UUID, now time.Time) (*store.PaymentTransfer, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row *store.PaymentTransfer
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).MarkTransferCalled(ctx, store.MarkTransferCalledParams{ID: transferID, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		row = &r
		return err
	})
	return row, err
}

// failTransfer 讓 PENDING 的轉帳進終局 FAILED（不佔有效轉帳的位置）；列已不是 PENDING 時什麼都不做。
func (s *Service) failTransfer(ctx context.Context, transferID uuid.UUID, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).FailTransfer(ctx, store.FailTransferParams{ID: transferID, FailureReason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}
