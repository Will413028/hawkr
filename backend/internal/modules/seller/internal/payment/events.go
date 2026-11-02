package payment

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (s *Service) pollable(ctx context.Context, after int64) ([]int64, error) {
	return store.New(s.d.DB.Conn(ctx)).PollablePaymentAccountStoreIDs(ctx, store.PollablePaymentAccountStoreIDsParams{
		EventBefore: s.d.Now().UTC().Add(-eventGrace), AfterStoreID: after, Batch: sweepBatch,
	})
}

func (s *Service) stale(ctx context.Context, after int64) ([]int64, error) {
	return store.New(s.d.DB.Conn(ctx)).StalePaymentAccountCreateStoreIDs(ctx, store.StalePaymentAccountCreateStoreIDsParams{
		IdleBefore: s.d.Now().UTC().Add(-createIdle), AfterStoreID: after, Batch: sweepBatch,
	})
}

// ReceiveEvent 接收 provider 的 webhook（HTTP 入口是 httpapi 的 POST /webhooks/stripe）：驗簽，先寫 inbox 再處理（規則 5）。(provider, event_id) 已存在的重送事件不再排 job；
// 外部帳戶對得到本地帳戶時排入該店家的同步 job（以 store_id 為唯一鍵），對不到的只留紀錄。驗過簽章但不處理的事件直接回 nil（不寫 inbox、不排 job）；簽章或格式不合回 InvalidInput，沒有設定 provider 回
// FailedPrecondition。
func (s *Service) ReceiveEvent(ctx context.Context, header http.Header, body []byte) error {
	p := s.d.Provider
	if p == nil {
		return apperr.FailedPrecondition("尚未設定收款開通")
	}
	ev, err := p.ParseEvent(header, body)
	if errors.Is(err, ErrIgnoredEvent) {
		return nil
	}
	if errors.Is(err, ErrInvalidEvent) {
		return apperr.InvalidInput("事件的簽章或格式不合")
	}
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		inserted, err := q.InsertPaymentProviderEvent(ctx, store.InsertPaymentProviderEventParams{
			Provider: p.Name(), EventID: ev.ID, ExternalAccountID: ev.ExternalAccountID, ReceivedAt: s.d.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if inserted == 0 {
			return nil
		}
		acct, err := q.PaymentAccountByExternal(ctx, store.PaymentAccountByExternalParams{Provider: p.Name(), ExternalAccountID: &ev.ExternalAccountID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return enqueueSync(ctx, s.d.Jobs, acct.StoreID)
	})
}
