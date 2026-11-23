package charge

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
)

// Sync 是同步 job（payment.charge_sync）的內容：在 transaction 外重讀 PaymentIntent（不信事件內容），在 transaction 內推導狀態寫入、狀態改變時發布 PaymentChanged，
// 並經 platform inbox 把這個 PaymentIntent 在讀取開始前收到的事件整批標為已處理（事件只是「重讀」的訊號，handler 不標記，由涵蓋它的同步標記）。
// 同步可能和別的同步或建立並行：寫入只在這次的讀取開始時間不早於已存的 fetched_at 時進行，讀取過期（較新的讀取已寫入並標記）的這次不標記，仍以唯讀的 RoutePending 回答 again；
// 判斷過的快照一律標記，包括與列的終局狀態衝突而保留列的（否則事件永遠未處理，job 每 2 秒 snooze 一次）。
// 沒有外部 ID（還在建立中）的列什麼都不做，由建立 job 負責。again 為 true 表示讀取期間又收到事件：呼叫方稍後再跑一輪。
// 快照與列對不起來回 ErrAnomaly（永久，job 取消，記 error log，列不動，讀取開始前的事件標記已處理）；不認得的狀態回一般錯誤，由 River 重試。
func (s *Service) Sync(ctx context.Context, paymentID uuid.UUID) (again bool, err error) {
	if s.d.Provider == nil {
		return false, nil
	}
	p := s.d.Provider
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.ExternalPaymentID == nil || row.Provider != p.Name() {
		return false, nil
	}
	external := *row.ExternalPaymentID
	startedAt := s.d.Now().UTC()
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := p.GetPayment(callCtx, external)
	if err != nil {
		return false, err
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	err = s.d.DB.InTx(wctx, func(ctx context.Context) error {
		res, err := s.applySnapshot(ctx, paymentID, snap, startedAt)
		if err != nil {
			return err
		}
		if res.judged {
			if _, err := s.d.Inbox.MarkRouteProcessed(ctx, p.Name(), external, startedAt, syncProcessor); err != nil {
				return err
			}
		}
		again, err = s.d.Inbox.RoutePending(ctx, p.Name(), external)
		return err
	})
	if errors.Is(err, ErrAnomaly) {
		s.d.Logger.ErrorContext(ctx, "payment：同步讀到與付款列不一致的快照，不寫入", "payment_id", paymentID.String(), "external_payment_id", external, "err", err)
		// 已告警、列保持原樣可查：把讀取開始前的事件標為已處理，掃描不再為同一批事件反覆排同步直到 inbox 保存期滿。
		if merr := s.markEventsProcessed(ctx, p.Name(), external, startedAt); merr != nil {
			return false, errors.Join(err, merr)
		}
	}
	return again, err
}

// markEventsProcessed 在獨立的 transaction 把這個 PaymentIntent 在 startedAt 之前收到的事件標為已處理。
func (s *Service) markEventsProcessed(ctx context.Context, provider, external string, startedAt time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := s.d.Inbox.MarkRouteProcessed(ctx, provider, external, startedAt, syncProcessor)
		return err
	})
}

// ReceiveEvent 接收 provider 的付款 webhook（HTTP 入口是 httpapi 的 POST /webhooks/stripe-payments，與收款帳戶的入口分開：一個 destination 一個 endpoint 一把簽章密鑰）：
// 驗簽，在一個 transaction 內把事件（原始 payload，路由鍵是 PaymentIntent ID）寫進 platform inbox 並排入派送 job；(provider, event_id) 已存在的重送事件不再寫入、不再派送。
// 驗過簽章但不處理的事件直接回 nil；簽章或格式不合回 InvalidInput，沒有設定 provider 回 FailedPrecondition。payload 不寫 log。
func (s *Service) ReceiveEvent(ctx context.Context, header http.Header, body []byte) error {
	p := s.d.Provider
	if p == nil {
		return apperr.FailedPrecondition("尚未設定付款")
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
	if ev.Kind != EventPayment {
		return nil
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := s.d.Inbox.Record(ctx, inbox.Event{Provider: p.Name(), EventID: ev.ID, RouteKey: ev.ObjectID, Payload: body})
		return err
	})
}
