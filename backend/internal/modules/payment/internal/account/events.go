package account

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
)

func (s *Service) pollable(ctx context.Context, after int64) ([]int64, error) {
	return store.New(s.d.DB.Conn(ctx)).PollablePaymentAccountStoreIDs(ctx, store.PollablePaymentAccountStoreIDsParams{AfterStoreID: after, Batch: sweepBatch})
}

func (s *Service) stale(ctx context.Context, after int64) ([]int64, error) {
	return store.New(s.d.DB.Conn(ctx)).StalePaymentAccountCreateStoreIDs(ctx, store.StalePaymentAccountCreateStoreIDsParams{
		IdleBefore: s.d.Now().UTC().Add(-createIdle), AfterStoreID: after, Batch: sweepBatch,
	})
}

// ReceiveEvent 接收 provider 的 webhook（HTTP 入口是 httpapi 的 POST /webhooks/stripe）：驗簽，在一個 transaction 內把事件（原始 payload，
// 路由鍵是外部帳戶 ID）寫進 platform inbox 並排入派送 job（規則 5、D29）；呼叫方在這裡回傳之後才回 2xx。(provider, event_id) 已存在的重送事件
// 不再寫入、不再派送。驗過簽章但不處理的事件直接回 nil（不寫 inbox）；簽章或格式不合回 InvalidInput，沒有設定 provider 回 FailedPrecondition。
// payload 不寫 log。
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
		_, err := s.d.Inbox.Record(ctx, inbox.Event{Provider: p.Name(), EventID: ev.ID, RouteKey: ev.ExternalAccountID, Payload: body})
		return err
	})
}

// handleEvent 是收款事件的 inbox handler（派送 job 的 transaction 內）：外部帳戶對得到本地帳戶時排入該擁有者的同步 job（以 store_id 為唯一鍵），
// 不標記已處理：事件只是「重讀帳戶」的訊號，由涵蓋它的同步標記（被執行中的同步合併時，那一輪結束後 RoutePending 讓它再跑一輪）。對不到本地帳戶
// 的事件只留紀錄、直接標記已處理（帳戶之後才填上外部 ID 時，建立 job 會排入同步，讀到的是最新狀態）。
func (s *Service) handleEvent(ctx context.Context, e inbox.Event) error {
	acct, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByExternal(ctx, store.PaymentAccountByExternalParams{Provider: e.Provider, ExternalAccountID: &e.RouteKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.d.Inbox.MarkProcessed(ctx, e.Provider, e.EventID, eventProcessor)
	}
	if err != nil {
		return err
	}
	return enqueueSync(ctx, s.d.Jobs, acct.StoreID)
}

// staleEventStores 是 inbox 有 eventGrace 之前收到卻還沒處理的事件的帳戶（事件在同步寫入快照之後、job 標記完成之前到達時，排入被執行中的 job 合併掉；
// inbox 是唯一不會漏的來源），依外部帳戶 ID 分批，回傳下一批的起點。
func (s *Service) staleEventStores(ctx context.Context, provider, after string) (ids []int64, next string, err error) {
	routes, err := s.d.Inbox.StaleRoutes(ctx, provider, s.d.Now().UTC().Add(-eventGrace), after, sweepBatch)
	if err != nil || len(routes) == 0 {
		return nil, "", err
	}
	ids, err = store.New(s.d.DB.Conn(ctx)).PaymentAccountStoreIDsByExternal(ctx, store.PaymentAccountStoreIDsByExternalParams{Provider: provider, ExternalAccountIds: routes})
	if err != nil {
		return nil, "", err
	}
	if len(routes) == sweepBatch {
		next = routes[len(routes)-1]
	}
	return ids, next, nil
}
