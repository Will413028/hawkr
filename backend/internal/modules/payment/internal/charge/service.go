package charge

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	// callTimeout 是 job 內單次呼叫 provider 的上限；requestCallTimeout 是請求內建立 PaymentIntent 的上限（逾時回 pending，補做 job 接手）。
	callTimeout        = 20 * time.Second
	requestCallTimeout = 10 * time.Second
	// createJobDelay 是建立付款時補做 job 延後多久才跑：請求內呼叫的上限再加一點餘裕，請求內的建立正常結束之前補做不會開始。
	createJobDelay = requestCallTimeout + 5*time.Second
	// descriptionMax 是付款說明的字數上限（以 rune 計，與 payments_description_check 相同）。
	descriptionMax = 200
	// writeTimeout 是寫結果的 transaction 的上限（job 的 ctx 逾時被取消後仍要能寫入）。
	writeTimeout = 15 * time.Second
	// jobTimeout 是一次 job 的上限：一次 provider 呼叫加幾次寫結果的 transaction。
	jobTimeout = callTimeout + 3*writeTimeout
	// maxAttempts 是 job 暫時性失敗時 River 重試的上限（退避是 River 預設的 attempt^4 秒，約 80 分鐘內用完）。
	maxAttempts = 8
	// sweepBatch 是掃描一次排入的上限。
	sweepBatch = 100
	// eventGrace 是 inbox 事件多久還沒處理之後由掃描補排同步。
	eventGrace = time.Minute
	// createIdle 是「建立中」的列閒置多久之後由掃描補排建立 job（保底，平常 job 一定存在）。
	createIdle = 10 * time.Minute
	// readIdle 是已建立、未結案的付款多久沒讀過之後由掃描補排同步（漏掉 webhook 的保底）。
	readIdle = 30 * time.Minute
	// IdempotencyRetention 是本模組依賴 provider 保存冪等鍵結果的時間（保守取 24 小時）：超過它的鍵從不重送，因為重送可能建出第二個 PaymentIntent。
	IdempotencyRetention = 24 * time.Hour
	// UnresolvedDeadline 是「呼叫過、沒有外部 ID」的付款從 create_called_at 起最久等多久：期限內以同一把鍵重送取回 PaymentIntent，超過進終局 CREATE_FAILED，
	// 鍵不再重送（買家再次建立時換新鍵）。必須小於 IdempotencyRetention，所以任何重送都在保存期內。付款只在訂單的付款期限內有意義，所以比收款帳戶（7 天）短。
	UnresolvedDeadline = 12 * time.Hour
	// EventRetention 是付款事件原始 payload 在 inbox 的保存期。
	EventRetention = 30 * 24 * time.Hour
	// syncProcessor 與 eventProcessor 是 inbox 的處理者名稱。
	syncProcessor  = "payment.charge_sync"
	eventProcessor = "payment.charge_event"
	// failedUnresolved 與 failedRefused 是建立的終局原因（payments.failure_reason）。
	failedUnresolved = "unresolved_creation"
	failedRefused    = "provider_refused"
)

// UnresolvedDeadline 必須小於冪等鍵保存期：差值為負時這行無法編譯。
const _ = uint64(IdempotencyRetention - UnresolvedDeadline - 1)

// Deps 外部依賴；clock 由組裝點注入。Provider 為 nil 表示沒有設定付款：付款的呼叫回 FailedPrecondition，這時不需要 Inbox 與 Orders；設定了 Provider 時兩者必填。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Inbox    *inbox.Inbox
	Provider Provider
	Orders   Orders
	Now      func() time.Time
	Logger   *slog.Logger
	// NewID 產生付款列的 UUIDv7；測試可以注入固定序列，預設是 uuid.NewV7。
	NewID func() uuid.UUID
}

// Service 是付款的 use case。
type Service struct{ d Deps }

// New 建立服務；設定了 provider 時把它的付款事件處理登記到 inbox（以 provider 自己的名稱，必須與收款帳戶的 provider 不同）。
func New(d Deps) *Service {
	if d.DB == nil || d.Events == nil || d.Jobs == nil || d.Now == nil || d.Logger == nil {
		panic("payment/charge: missing dependency")
	}
	if d.Provider != nil && (d.Inbox == nil || d.Orders == nil) {
		panic("payment/charge: Inbox and Orders are required with a provider")
	}
	if d.NewID == nil {
		d.NewID = uuid.NewV7
	}
	s := &Service{d: d}
	if d.Provider != nil {
		d.Inbox.Register(inbox.Source{Provider: d.Provider.Name(), Retention: EventRetention, Handle: s.handleEvent})
	}
	return s
}

// Configured 回傳是否設定了 provider。
func (s *Service) Configured() bool { return s.d.Provider != nil }

func (s *Service) requireProvider() error {
	if s.d.Provider == nil {
		return apperr.FailedPrecondition("尚未設定付款")
	}
	return nil
}

// errInTransaction 標示呼叫方在 DB transaction 內呼叫了會呼叫 provider 的方法（規則 5）。
var errInTransaction = errors.New("charge: 會呼叫 provider 的方法不能在 transaction 內呼叫")

// CreatePayment 是買家對一張訂單的付款建立（D7 修訂、D8）：先在 transaction 外向訂單一方取應付資訊（歸屬、可付、金額），再在一個 transaction 內寫「建立中」的付款列
// （order_no 唯一：同一張訂單已有列時沿用它，不會有第二個 PaymentIntent）並排入補做 job；提交之後在請求內、transaction 外帶列上的冪等鍵建立 PaymentIntent，
// 回傳含 client_secret 的付款。provider 失敗或逾時不是錯誤：回傳狀態是 CREATING 的付款（pending），補做 job 以同一把鍵接手，呼叫端用 GetPayment 重讀。
// 建立的終局（provider 永久拒絕）回傳 CREATE_FAILED。已建立的付款回傳目前狀態（AWAITING_PAYMENT 時含 client_secret）。
// 訂單不存在或不屬於呼叫者回 NotFound；訂單不可付、金額不是正數或內容和已存在的付款不符回 FailedPrecondition。呼叫方不能在 transaction 內呼叫。
func (s *Service) CreatePayment(ctx context.Context, buyerID int64, orderNo string) (model.Payment, error) {
	if err := s.requireProvider(); err != nil {
		return model.Payment{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.Payment{}, errInTransaction
	}
	if err := model.Positive(buyerID); err != nil {
		return model.Payment{}, err
	}
	orderNo = strings.TrimSpace(orderNo)
	if n := utf8.RuneCountInString(orderNo); n < 1 || n > 64 {
		return model.Payment{}, apperr.InvalidInput("訂單編號長度不合")
	}
	orderCtx, cancel := context.WithTimeout(ctx, callTimeout)
	order, err := s.d.Orders.OrderForPayment(orderCtx, orderNo)
	cancel()
	if err != nil {
		return model.Payment{}, err
	}
	if order.OrderNo != orderNo || order.BuyerID != buyerID {
		return model.Payment{}, apperr.NotFound("找不到這張訂單")
	}
	row, err := s.ensureRow(ctx, order)
	if err != nil {
		return model.Payment{}, err
	}
	if row.Status == string(model.PaymentCreating) {
		return s.createInRequest(ctx, row)
	}
	return s.view(ctx, row)
}

// ensureRow 在一個 transaction 內取得這張訂單的付款列：沒有就寫「建立中」的列；有就沿用（建立失敗的終局，或已取消而訂單仍可付的列，同一列換新鍵、清掉舊的外部 ID 重設回建立中）；
// 列還在建立中時同 transaction 排入補做 job（延到請求內建立的上限之後，見 createJobDelay）。
func (s *Service) ensureRow(ctx context.Context, order Order) (store.PaymentPayment, error) {
	var row store.PaymentPayment
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		if order.Payable && order.Amount > 0 {
			if _, err := q.InsertPayment(ctx, store.InsertPaymentParams{
				ID: s.d.NewID(), OrderNo: order.OrderNo, StoreID: order.StoreID, BuyerID: order.BuyerID, Amount: order.Amount, Provider: s.d.Provider.Name(), Now: now,
			}); err != nil {
				return err
			}
		}
		existing, err := q.PaymentByOrderNo(ctx, order.OrderNo)
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRefusal(order)
		}
		if err != nil {
			return err
		}
		row = existing
		if row.Status == string(model.PaymentSucceeded) {
			return nil
		}
		if err := orderRefusal(order); err != nil {
			return err
		}
		if row.Status == string(model.PaymentCreateFailed) || row.Status == string(model.PaymentCanceled) {
			row, err = q.ResetPayment(ctx, store.ResetPaymentParams{
				ID: row.ID, StoreID: order.StoreID, BuyerID: order.BuyerID, Amount: order.Amount, Provider: s.d.Provider.Name(), Description: paymentDescription(order), Now: now,
			})
			if err != nil {
				return err
			}
		}
		if row.StoreID != order.StoreID || row.BuyerID != order.BuyerID || row.Amount != order.Amount {
			return apperr.FailedPrecondition("訂單內容在付款建立之後變動了")
		}
		if row.Status == string(model.PaymentCreating) {
			return enqueueCreateAt(ctx, s.d.Jobs, row.ID, now.Add(createJobDelay))
		}
		return nil
	})
	return row, err
}

// paymentDescription 是送給 provider 的付款說明：訂單給的商品名稱（去頭尾空白、超過上限截斷），沒有就用訂單編號；存在列上，冪等重送時內容不變。
func paymentDescription(order Order) string {
	d := strings.TrimSpace(order.Description)
	if d == "" {
		return order.OrderNo
	}
	if utf8.RuneCountInString(d) > descriptionMax {
		d = strings.TrimSpace(string([]rune(d)[:descriptionMax]))
	}
	return d
}

// orderRefusal 是訂單現在不能建立付款的原因（不可付、金額不是正數）；可以建立時是 nil。
func orderRefusal(order Order) error {
	if !order.Payable {
		return apperr.FailedPrecondition("訂單目前不能付款")
	}
	if order.Amount <= 0 {
		return apperr.FailedPrecondition("零元訂單不經金流商付款")
	}
	return nil
}

// createInRequest 在請求內建立 PaymentIntent（transaction 已提交）：成功回含 client_secret 的付款；暫時性失敗與逾時回 pending（補做 job 已排），永久拒絕回 CREATE_FAILED。
// 結束時列已經是 AWAITING_PAYMENT 卻沒有 secret（補做 job 先寫入、請求的快照被擋下）時改走讀取路徑向 provider 取 secret；取不到就回沒有 secret 的付款，由 client 以 GetPayment 重讀。
func (s *Service) createInRequest(ctx context.Context, row store.PaymentPayment) (model.Payment, error) {
	res, err := s.create(ctx, row.ID, requestCallTimeout)
	if err != nil {
		s.d.Logger.WarnContext(ctx, "payment：請求內建立 PaymentIntent 沒有結果，回 pending 由補做 job 接手", "payment_id", row.ID.String(), "order_no", row.OrderNo, "err", err)
		latest, rerr := store.New(s.d.DB.Conn(ctx)).PaymentByID(ctx, row.ID)
		if rerr != nil {
			return model.Payment{}, rerr
		}
		res = createResult{row: latest}
	}
	if res.clientSecret == "" && res.row.Status == string(model.PaymentAwaitingPayment) {
		out, verr := s.view(ctx, res.row)
		if verr == nil {
			return out, nil
		}
		s.d.Logger.WarnContext(ctx, "payment：請求內建立後讀不到 client_secret，由 client 重讀", "payment_id", row.ID.String(), "order_no", row.OrderNo, "err", verr)
	}
	return toModel(res.row, res.clientSecret), nil
}

// GetPayment 讀買家自己的付款（供 client 在 pending、3DS 之後與重新整理時重讀）；不是呼叫者的付款回 NotFound（與不存在無法區分）。狀態是 AWAITING_PAYMENT 時向 provider 讀 client_secret
// 一併回傳（不存）；provider 那邊的狀態已經往前（買家剛付款、webhook 還沒到）時不回 secret，並排入同步 job。呼叫方不能在 transaction 內呼叫。
func (s *Service) GetPayment(ctx context.Context, buyerID int64, orderNo string) (model.Payment, error) {
	if err := s.requireProvider(); err != nil {
		return model.Payment{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.Payment{}, errInTransaction
	}
	if err := model.Positive(buyerID); err != nil {
		return model.Payment{}, err
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentByOrderNo(ctx, strings.TrimSpace(orderNo))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.BuyerID != buyerID) {
		return model.Payment{}, apperr.NotFound("找不到這筆付款")
	}
	if err != nil {
		return model.Payment{}, err
	}
	return s.view(ctx, row)
}

// view 把列轉成對外的付款；AWAITING_PAYMENT 時讀 client_secret。
func (s *Service) view(ctx context.Context, row store.PaymentPayment) (model.Payment, error) {
	out := toModel(row, "")
	if row.Status != string(model.PaymentAwaitingPayment) || row.ExternalPaymentID == nil || row.Provider != s.d.Provider.Name() {
		return out, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, requestCallTimeout)
	defer cancel()
	snap, err := s.d.Provider.GetPayment(callCtx, *row.ExternalPaymentID)
	if err != nil {
		return model.Payment{}, apperr.WithCause(apperr.Unavailable("暫時無法取得付款資訊"), err)
	}
	if snap.State != StateAwaitingPayment {
		if err := s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueueSync(ctx, s.d.Jobs, row.ID) }); err != nil {
			return model.Payment{}, err
		}
		return out, nil
	}
	out.ClientSecret = snap.ClientSecret
	return out, nil
}

// toModel 把列轉成對外的付款。
func toModel(r store.PaymentPayment, clientSecret string) model.Payment {
	p := model.Payment{
		ID: r.ID, OrderNo: r.OrderNo, StoreID: r.StoreID, BuyerID: r.BuyerID, Amount: r.Amount, Provider: r.Provider,
		Status: model.PaymentStatus(r.Status), PaidAt: r.PaidAt, CreatedAt: r.CreatedAt, ClientSecret: clientSecret,
	}
	if r.LastError != nil {
		p.LastError = *r.LastError
	}
	return p
}

// handleEvent 是付款事件的 inbox handler（派送 job 的 transaction 內，不能呼叫 provider）：PaymentIntent 對得到本地付款時排入該付款的同步 job（以付款 ID 為唯一鍵），
// 不標記已處理，也不讀事件內容：狀態一律由同步重讀得到。對不到本地付款的事件只留紀錄、直接標記已處理（付款列的外部 ID 還沒寫進來時，建立的結果本身就是最新的快照）。
func (s *Service) handleEvent(ctx context.Context, e inbox.Event) error {
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentByExternal(ctx, store.PaymentByExternalParams{Provider: e.Provider, ExternalPaymentID: &e.RouteKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.d.Inbox.MarkProcessed(ctx, e.Provider, e.EventID, eventProcessor)
	}
	if err != nil {
		return err
	}
	return enqueueSync(ctx, s.d.Jobs, row.ID)
}
