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

// RequestTransfer 是訂單完成後把這筆付款的款項轉給賣家的入口（D6；訂單一方的 GrantPayment 呼叫）：在一個 transaction 內鎖付款列、確認已入帳，
// 寫一筆 PENDING 轉帳（金額是付款金額扣掉非失敗退款，賣家收款帳戶的外部 ID 寫成快照；帳戶必須已開通轉帳）並排入轉帳 job；provider 在 transaction 外由 job 呼叫（規則 5）。
// 冪等：已有有效（PENDING、SUCCEEDED 或 ANOMALY）轉帳時回傳它，由唯一索引 transfers_active_payment 守，不寫第二筆；只有沒有有效轉帳時，付款未入帳、賣家帳戶未開通
// 或沒有可轉的金額（退款全部 SUCCEEDED 而涵蓋整筆付款；涵蓋整筆但還有進行中的退款時仍寫列，暫定金額是進行中退款的加總，claim 會重算）才是 FailedPrecondition（FAILED 的轉帳不佔位，之後再呼叫會建新的一筆、換新鍵）。付款不存在回 NotFound。呼叫方不能在 transaction 內呼叫。
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
		terms, refusal, err := s.transferTerms(ctx, q, pay)
		if err != nil {
			return err
		}
		if refusal == nil {
			n, err := q.InsertTransfer(ctx, store.InsertTransferParams{ID: s.d.NewID(), PaymentID: pay.ID, StoreID: pay.StoreID, Amount: terms.amount, DestinationAccountID: terms.destination, Now: now})
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

type transferTermsResult struct {
	amount      int64
	destination string
}

// transferTerms 算出這筆付款現在可以轉的金額與賣家帳戶的外部 ID。現在不能轉是 refusal（FailedPrecondition），DB 錯誤是 err：呼叫方在 err 時立刻返回，
// 因為失敗的查詢已讓 transaction 進入錯誤狀態，不能再下 SQL。
func (s *Service) transferTerms(ctx context.Context, q *store.Queries, pay store.PaymentPayment) (terms transferTermsResult, refusal error, err error) {
	if pay.Status != string(model.PaymentSucceeded) {
		return terms, apperr.FailedPrecondition("付款尚未入帳，不能轉帳"), nil
	}
	acct, err := q.PaymentAccountByStore(ctx, pay.StoreID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return terms, nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) || !acct.TransfersActive || acct.ExternalAccountID == nil {
		return terms, apperr.FailedPrecondition("賣家的收款帳戶尚未開通"), nil
	}
	refunded, err := q.NonFailedRefundTotal(ctx, pay.ID)
	if err != nil {
		return terms, nil, err
	}
	amount := pay.Amount - refunded
	if amount <= 0 {
		inFlight, err := q.InFlightRefundTotal(ctx, pay.ID)
		if err != nil {
			return terms, nil, err
		}
		if inFlight <= 0 {
			return terms, apperr.FailedPrecondition("付款已全額退款，沒有可轉的金額"), nil
		}
		amount = inFlight
	}
	return transferTermsResult{amount: amount, destination: *acct.ExternalAccountID}, nil, nil
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

// Transfer 是轉帳 job（payment.transfer_create）的內容：列還是 PENDING 時依結果三分收斂（ADR D68）。
//   - 呼叫前先 claim（claimTransfer）：這把鍵的第一次送出在一個獨立的 transaction 內鎖付款列、以「付款金額減非失敗退款」算定金額並與 called_at 一起寫入，算出不大於 0 就不呼叫 provider、
//     轉帳進 FAILED（fully_refunded）；之後帶列上的冪等鍵與金額、付款的 charge 建立轉帳（source_transaction），成功在新的 transaction 寫外部 ID 與 SUCCEEDED，並讓等待中的退款撤回開始。
//   - provider 終局拒絕（ErrPermanent）進 FAILED（job 完成，不回錯誤）；可重試拒絕（ErrRetryableRefusal，例如平台餘額不足）換新冪等鍵、清掉 called_at（下一次送出重新 claim、重算金額）並回錯誤由 River 重試，不計任何期限。
//   - 結果不明回錯誤，River 與週期掃描以同一把鍵重送，直到 ResendWindow；之後不再呼叫 provider，只以 FindTransfer 依嘗試 ID 查回：找到就收下，
//     查無要滿 NotFoundAfter 才進 FAILED（之後 RequestTransfer 才建得出新的一筆、換新鍵），未滿回錯誤稍後再查；查回連續失敗 LookupFailureLimit 次進 ANOMALY。
//   - 付款沒有 charge、provider 回的外部 ID 或金額與列不符進 ANOMALY（佔住唯一索引、補掃不再重試、發 MoneyAnomaly；job 完成）。
//
// 已不是 PENDING 的列什麼都不做。退款涵蓋整筆付款但還有進行中的（errRefundsInFlight）時記警示 log（轉帳與擋住它的退款 ID）並回該錯誤，不受 final 影響，job 以 snooze 等它們終局。
// final 為 true 時其他暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
func (s *Service) Transfer(ctx context.Context, transferID uuid.UUID, final bool) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
	err := s.transfer(ctx, transferID)
	var waiting *refundsInFlightError
	if errors.As(err, &waiting) {
		ids := make([]string, len(waiting.refundIDs))
		for i, id := range waiting.refundIDs {
			ids[i] = id.String()
		}
		s.d.Logger.WarnContext(ctx, "payment：轉帳等進行中的退款終局", "transfer_id", transferID.String(), "refund_ids", ids)
		return err
	}
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
		return s.anomalyTransfer(ctx, row, "payment_without_charge", "")
	}
	now := s.d.Now().UTC()
	if !inResendWindow(row.CalledAt, now) {
		return s.lookupTransfer(ctx, row, pay, now)
	}
	called, err := s.claimTransfer(ctx, transferID, now)
	if err != nil || called == nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := s.d.Provider.CreateTransfer(callCtx, TransferInput{
		Amount: called.Amount, SourceChargeID: *pay.ExternalChargeID, DestinationAccountID: called.DestinationAccountID,
		TransferGroup: pay.OrderNo, IdempotencyKey: called.IdempotencyKey.String(),
	})
	switch classifyCall(err) {
	case callRefused:
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕建立轉帳", "transfer_id", transferID.String(), "order_no", pay.OrderNo, "err", err)
		return s.failTransfer(ctx, transferID, failedRefused)
	case callRetryable:
		if !firstCall(row.CalledAt, called.CalledAt, now) {
			s.d.Logger.WarnContext(ctx, "payment：重送轉帳被拒絕，但這把鍵之前的呼叫結果不明，不換鍵", "transfer_id", transferID.String(), "order_no", pay.OrderNo, "err", err)
			return err
		}
		s.d.Logger.WarnContext(ctx, "payment：provider 拒絕建立轉帳、沒有執行，換新冪等鍵重試", "transfer_id", transferID.String(), "order_no", pay.OrderNo, "err", err)
		if rerr := s.rotateTransferKey(ctx, transferID, called.IdempotencyKey, now); rerr != nil {
			return errors.Join(err, rerr)
		}
		return err
	case callUnknown:
		return err
	case callSucceeded:
	}
	return s.acceptTransfer(ctx, *called, snap)
}

// lookupTransfer 是過了 ResendWindow 的轉帳：不再呼叫 provider，只依嘗試 ID 查回。
func (s *Service) lookupTransfer(ctx context.Context, row store.PaymentTransfer, pay store.PaymentPayment, now time.Time) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, found, err := s.d.Provider.FindTransfer(callCtx, pay.OrderNo, row.IdempotencyKey.String())
	if err != nil {
		return s.transferLookupFailed(ctx, row, err)
	}
	if err := s.clearTransferLookupFailures(ctx, row); err != nil {
		return err
	}
	if found {
		return s.acceptTransfer(ctx, row, snap)
	}
	if now.Sub(*row.CalledAt) < NotFoundAfter {
		return fmt.Errorf("%w：轉帳 %s", errNotFoundYet, row.ID)
	}
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這筆轉帳，進 FAILED", "transfer_id", row.ID.String(), "order_no", pay.OrderNo)
	return s.failTransfer(ctx, row.ID, failedNotFound)
}

// acceptTransfer 驗證 provider 回的轉帳（建立的回應或查回找到的）與列相符後記成功；不符進 ANOMALY。
func (s *Service) acceptTransfer(ctx context.Context, row store.PaymentTransfer, snap TransferSnapshot) error {
	if snap.ExternalID == "" || snap.Amount != row.Amount {
		return s.anomalyTransfer(ctx, row, "reply_mismatch", snap.ExternalID)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	orphan := false
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPayment(ctx, row.PaymentID); err != nil {
			return err
		}
		_, err := q.SucceedTransfer(ctx, store.SucceedTransferParams{ID: row.ID, ExternalTransferID: &snap.ExternalID, Now: s.d.Now().UTC()})
		if err == nil {
			if err := s.startReversals(ctx, q, row.PaymentID); err != nil {
				return err
			}
			return s.startTopups(ctx, q, row.PaymentID)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			cur, rerr := q.TransferByID(ctx, row.ID)
			if rerr != nil {
				return rerr
			}
			if cur.Status == string(model.TransferSucceeded) && cur.ExternalTransferID != nil && *cur.ExternalTransferID == snap.ExternalID {
				return nil
			}
			orphan = true
			return s.publishAnomaly(ctx, model.AnomalyTransfer, row.ID, row.PaymentID, withDetail("transferred_but_transfer_"+cur.Status, snap.ExternalID))
		}
		return err
	})
	if err == nil && orphan {
		s.d.Logger.ErrorContext(ctx, "payment：provider 已建立轉帳，轉帳列卻已不在 PENDING，待人工核對",
			"transfer_id", row.ID.String(), "payment_id", row.PaymentID.String(), "transfer_external_id", snap.ExternalID)
	}
	return err
}

// claimTransfer 是這把冪等鍵第一次送出前的承諾（D11、ADR D69）：在獨立的 transaction 內依序鎖付款列、轉帳列，called_at 為空時（這把鍵還沒送過）以「付款金額減非失敗退款」算定金額，
// 與 called_at 一起寫入；called_at 已有值（同一把鍵的重送）金額不變。算定金額時等待中的預留都釋出（退款已從金額扣掉，不必再撤回）；算出不大於 0 時不呼叫 provider：退款全部終局（SUCCEEDED）才進 FAILED（fully_refunded，回 nil）；
// 還有進行中的退款（PENDING、REFUNDING、ANOMALY）時回 errRefundsInFlight、什麼都不寫，轉帳保持 PENDING 等它們終局，它們之後失敗金額就大於 0、賣家照常收到。列已不是 PENDING 時回 nil。呼叫 provider 之前提交，之後進來的退款看得到「金額已承諾」。
func (s *Service) claimTransfer(ctx context.Context, transferID uuid.UUID, now time.Time) (*store.PaymentTransfer, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row *store.PaymentTransfer
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		cur, err := q.TransferByID(ctx, transferID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if cur.Status != string(model.TransferPending) {
			return nil
		}
		pay, err := q.LockPayment(ctx, cur.PaymentID)
		if err != nil {
			return err
		}
		locked, err := q.LockTransfer(ctx, transferID)
		if err != nil {
			return err
		}
		if locked.Status != string(model.TransferPending) {
			return nil
		}
		amount := locked.Amount
		if locked.CalledAt == nil {
			refunded, err := q.NonFailedRefundTotal(ctx, pay.ID)
			if err != nil {
				return err
			}
			amount = pay.Amount - refunded
			if amount <= 0 {
				inFlight, err := q.InFlightRefundTotal(ctx, pay.ID)
				if err != nil {
					return err
				}
				if inFlight > 0 {
					blocking, err := q.InFlightRefundIDs(ctx, pay.ID)
					if err != nil {
						return err
					}
					return &refundsInFlightError{transferID: transferID, refundIDs: blocking}
				}
			}
			if err := s.releaseWaitingReversals(ctx, q, pay.ID, reversalDeducted); err != nil {
				return err
			}
			if amount <= 0 {
				reason := failedFullyRefunded
				if _, err := q.FailTransfer(ctx, store.FailTransferParams{ID: transferID, FailureReason: &reason, Now: now}); err != nil {
					return err
				}
				return s.settleTopupsWithoutTransfer(ctx, q, pay.ID)
			}
			if err := s.releaseWaitingTopups(ctx, q, pay.ID, topupCountedInNewAmount); err != nil {
				return err
			}
			if err := q.MarkRefundsDeducted(ctx, pay.ID); err != nil {
				return err
			}
		}
		r, err := q.ClaimTransfer(ctx, store.ClaimTransferParams{ID: transferID, Amount: amount, Now: now})
		if err != nil {
			return err
		}
		row = &r
		return nil
	})
	return row, err
}

// rotateTransferKey 在 provider 已知拒絕之後換新冪等鍵；已被別的呼叫換過或列已不是 PENDING 時什麼都不做。
func (s *Service) rotateTransferKey(ctx context.Context, transferID, oldKey uuid.UUID, now time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).RotateTransferKey(ctx, store.RotateTransferKeyParams{ID: transferID, OldKey: oldKey, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

// transferLookupFailed 記一次查回失敗；連續失敗到上限進 ANOMALY（回 nil），否則回錯誤由 River 重試。
func (s *Service) transferLookupFailed(ctx context.Context, row store.PaymentTransfer, cause error) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var n int32
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		var err error
		n, err = store.New(s.d.DB.Conn(ctx)).BumpTransferLookupFailures(ctx, store.BumpTransferLookupFailuresParams{ID: row.ID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			n = 0
			return nil
		}
		return err
	})
	if err != nil {
		return errors.Join(cause, err)
	}
	if n >= LookupFailureLimit {
		return s.anomalyTransfer(ctx, row, "lookup_failed", "")
	}
	return fmt.Errorf("charge: 轉帳 %s 查回失敗：%w", row.ID, cause)
}

func (s *Service) clearTransferLookupFailures(ctx context.Context, row store.PaymentTransfer) error {
	if row.LookupFailures == 0 {
		return nil
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		return store.New(s.d.DB.Conn(ctx)).ClearTransferLookupFailures(ctx, row.ID)
	})
}

// anomalyTransfer 讓 PENDING 的轉帳進 ANOMALY 並在同一個 transaction 發 MoneyAnomaly；列已不是 PENDING 時什麼都不做。
// 列上只存原因碼 reason；provider 的外部 ID（detail）只進 log 與事件（failure_reason 上限 200 字）。
func (s *Service) anomalyTransfer(ctx context.Context, row store.PaymentTransfer, reason, detail string) error {
	s.d.Logger.ErrorContext(ctx, "payment：轉帳進 ANOMALY，待人工核對 provider", "transfer_id", row.ID.String(), "payment_id", row.PaymentID.String(), "reason", reason, "external_id", detail)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		now := s.d.Now().UTC()
		t, err := store.New(s.d.DB.Conn(ctx)).AnomalyTransfer(ctx, store.AnomalyTransferParams{ID: row.ID, Reason: &reason, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.publishAnomaly(ctx, model.AnomalyTransfer, t.ID, t.PaymentID, withDetail(reason, detail))
	})
}

// failTransfer 讓 PENDING 的轉帳進終局 FAILED（不佔有效轉帳的位置），並在同一個 transaction 釋出等待中的撤回預留（錢本來就在平台手上）；列已不是 PENDING 時什麼都不做。鎖順序：付款列、轉帳列、退款列。
func (s *Service) failTransfer(ctx context.Context, transferID uuid.UUID, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		cur, err := q.TransferByID(ctx, transferID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := q.LockPayment(ctx, cur.PaymentID); err != nil {
			return err
		}
		_, err = q.FailTransfer(ctx, store.FailTransferParams{ID: transferID, FailureReason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.releaseWaitingReversals(ctx, q, cur.PaymentID, reversalTransferFailed); err != nil {
			return err
		}
		return s.settleTopupsWithoutTransfer(ctx, q, cur.PaymentID)
	})
}
