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
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
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
	// createJobDelay 是建立付款時補做 job 延後多久才跑：請求內的全部上限（記呼叫時刻的寫入、provider 呼叫、寫結果的寫入各自的上限）再加一點餘裕，
	// 請求內的建立最慢也在它之前結束，補做不會與請求搶同一把冪等鍵。
	createJobDelay = writeTimeout + requestCallTimeout + writeTimeout + 5*time.Second
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
	// IdempotencyRetention 是本模組依賴 provider 保存冪等鍵結果的時間（Stripe 至少 24 小時，取下限）：超過它的鍵從不重送，因為重送可能建出第二個物件。
	IdempotencyRetention = 24 * time.Hour
	// ResendWindow 是結果不明的呼叫從這把鍵的第一次呼叫起以同一把鍵重送的上限：必須小於 IdempotencyRetention，所以任何重送都在保存期內；
	// 超過之後只查回（Find*），不再重送。
	ResendWindow = 23 * time.Hour
	// NotFoundWait 是查回「查無」要再等多久才算確定不存在：最後一次重送在 ResendWindow 之前，provider 的列表可能晚一點才看得到它，
	// 所以從第一次呼叫起滿 ResendWindow + NotFoundWait 之後的查無才算數，之後才可以換新鍵重建（FAILED）。
	NotFoundWait = time.Hour
	// NotFoundAfter 是從第一次呼叫起，查無才算確定不存在的時間。
	NotFoundAfter = ResendWindow + NotFoundWait
	// LookupFailureLimit 是連續查回失敗幾次之後進 ANOMALY。
	LookupFailureLimit = 5
	// EventRetention 是付款事件原始 payload 在 inbox 的保存期。
	EventRetention = 30 * 24 * time.Hour
	// syncProcessor 與 eventProcessor 是 inbox 的處理者名稱。
	syncProcessor  = "payment.charge_sync"
	eventProcessor = "payment.charge_event"
	// failedRefused 與 failedNotFound 是終局失敗的原因（failure_reason）：provider 拒絕；查回證明 provider 沒有這筆。
	failedRefused  = "provider_refused"
	failedNotFound = "provider_not_found"
	// failedFullyRefunded 是轉帳在送出前算定金額時，付款已全額退款（沒有可轉的金額）而終局的原因。
	failedFullyRefunded = "fully_refunded"
)

// ResendWindow 必須小於冪等鍵保存期：差值為負時這行無法編譯。
const _ = uint64(IdempotencyRetention - ResendWindow - 1)

// Deps 外部依賴；clock 由組裝點注入。Provider 為 nil 表示沒有設定付款：付款的呼叫回 FailedPrecondition，這時不需要 Inbox、Orders 與 Audit；設定了 Provider 時三者必填。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Inbox    *inbox.Inbox
	Provider Provider
	Orders   Orders
	// Audit 是 platform 的後台動作紀錄：管理員的人工結案在領域變更的同一個 transaction 內寫一筆；設定了 Provider 時必填。
	Audit  *adminaudit.Log
	Now    func() time.Time
	Logger *slog.Logger
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
	if d.Provider != nil && (d.Inbox == nil || d.Orders == nil || d.Audit == nil) {
		panic("payment/charge: Inbox, Orders and Audit are required with a provider")
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

// CreatePayment 是買家一次結帳的付款建立（D7 修訂、D8、D13）：orderNos 是這次結帳的訂單（過渡期 Java 一次一張，B8 之後 N 張）。先在 transaction 外向訂單一方逐張取應付資訊（歸屬、可付、金額），
// 再在一個 transaction 內寫「建立中」的付款群組列與每張訂單應收一列（order_no 唯一：訂單已屬於一筆付款時沿用它，不會有第二個 PaymentIntent；訂單集合必須與既有付款完全相同）並排入補做 job；
// 提交之後在請求內、transaction 外帶列上的冪等鍵建立一個 PaymentIntent（金額是各訂單應收的總額），回傳含 client_secret 的付款。
// provider 失敗或逾時不是錯誤：回傳狀態是 CREATING 的付款（pending），補做 job 以同一把鍵接手，呼叫端用 GetPayment 重讀。建立的終局（provider 永久拒絕）回傳 CREATE_FAILED。
// 已建立的付款回傳目前狀態（AWAITING_PAYMENT 時含 client_secret）。訂單不存在或不屬於呼叫者回 NotFound；訂單不可付、金額不是正數、訂單集合或內容和已存在的付款不符回 FailedPrecondition。
// 呼叫方不能在 transaction 內呼叫。
func (s *Service) CreatePayment(ctx context.Context, buyerID int64, orderNos []string) (model.Payment, error) {
	if err := s.requireProvider(); err != nil {
		return model.Payment{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.Payment{}, errInTransaction
	}
	if err := model.Positive(buyerID); err != nil {
		return model.Payment{}, err
	}
	nos, err := normalizeOrderNos(orderNos)
	if err != nil {
		return model.Payment{}, err
	}
	orders := make([]Order, len(nos))
	for i, orderNo := range nos {
		orderCtx, cancel := context.WithTimeout(ctx, callTimeout)
		order, err := s.d.Orders.OrderForPayment(orderCtx, orderNo)
		cancel()
		if err != nil {
			return model.Payment{}, err
		}
		if order.OrderNo != orderNo || order.BuyerID != buyerID {
			return model.Payment{}, apperr.NotFound("找不到這張訂單")
		}
		orders[i] = order
	}
	row, err := s.ensureRow(ctx, buyerID, orders)
	if err != nil {
		return model.Payment{}, err
	}
	if row.Status == string(model.PaymentCreating) {
		return s.createInRequest(ctx, row)
	}
	return s.view(ctx, row)
}

// normalizeOrderNos 去掉頭尾空白、檢查長度，至少一張，重複的訂單編號回 InvalidInput（同一張訂單不能在同一次結帳出現兩次）。
func normalizeOrderNos(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, apperr.InvalidInput("至少要有一張訂單")
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, len(in))
	for i, orderNo := range in {
		orderNo = strings.TrimSpace(orderNo)
		if n := utf8.RuneCountInString(orderNo); n < 1 || n > 64 {
			return nil, apperr.InvalidInput("訂單編號長度不合")
		}
		if _, dup := seen[orderNo]; dup {
			return nil, apperr.InvalidInput("訂單編號重複")
		}
		seen[orderNo] = struct{}{}
		out[i] = orderNo
	}
	return out, nil
}

// errOrderTaken 標示寫訂單應收時訂單已被別的付款搶先佔住（並行的建立）：回滾整個 transaction 再讀既有的付款。
var errOrderTaken = errors.New("charge: 訂單已屬於另一筆付款")

// ensureRow 取得這次結帳的付款列：訂單都還沒有付款就寫「建立中」的群組列與每張訂單應收一列；有就沿用（訂單集合必須與既有付款完全相同；建立失敗的終局，
// 或已取消而訂單仍可付的列，同一列換新鍵、清掉舊的外部 ID 重設回建立中，訂單應收的店家與金額以這次讀到的為準）；列還在建立中時同 transaction 排入補做 job（延到請求內建立的上限之後，見 createJobDelay）。
// 並行的建立搶同一張訂單時，輸的一方回滾後再讀一次既有的付款。
func (s *Service) ensureRow(ctx context.Context, buyerID int64, orders []Order) (store.PaymentPayment, error) {
	for attempt := 0; ; attempt++ {
		row, err := s.ensureRowOnce(ctx, buyerID, orders)
		if errors.Is(err, errOrderTaken) && attempt == 0 {
			continue
		}
		return row, err
	}
}

func (s *Service) ensureRowOnce(ctx context.Context, buyerID int64, orders []Order) (store.PaymentPayment, error) {
	var row store.PaymentPayment
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		existing, err := existingGroup(ctx, q, orders)
		if err != nil {
			return err
		}
		if existing == nil {
			for _, order := range orders {
				if err := orderRefusal(order); err != nil {
					return err
				}
			}
			paymentID := s.d.NewID()
			if err := q.InsertPayment(ctx, newPaymentParams(paymentID, buyerID, orders, s.d.Provider.Name(), now)); err != nil {
				return err
			}
			for _, order := range orders {
				n, err := q.InsertPaymentOrder(ctx, store.InsertPaymentOrderParams{ID: s.d.NewID(), PaymentID: paymentID, OrderNo: order.OrderNo, StoreID: order.StoreID, Amount: order.Amount, Now: now})
				if err != nil {
					return err
				}
				if n == 0 {
					return errOrderTaken
				}
			}
			existing = &existingPayment{paymentID: paymentID}
		}
		row, err = q.PaymentByID(ctx, existing.paymentID)
		if err != nil {
			return err
		}
		if row.Status == string(model.PaymentSucceeded) {
			return nil
		}
		for _, order := range orders {
			if err := orderRefusal(order); err != nil {
				return err
			}
		}
		stored, err := q.PaymentOrdersOfPayment(ctx, row.ID)
		if err != nil {
			return err
		}
		if row.Status == string(model.PaymentCreateFailed) || row.Status == string(model.PaymentCanceled) {
			if _, err := q.SupersedeEffectiveAttempt(ctx, store.SupersedeEffectiveAttemptParams{PaymentID: row.ID, Now: &now}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			row, err = q.ResetPayment(ctx, store.ResetPaymentParams{
				ID: row.ID, BuyerID: buyerID, Amount: totalOf(orders), Provider: s.d.Provider.Name(), Description: paymentDescription(orders[0]), Now: now,
			})
			if err != nil {
				return err
			}
			byNo := make(map[string]store.PaymentPaymentOrder, len(stored))
			for _, o := range stored {
				byNo[o.OrderNo] = o
			}
			for _, order := range orders {
				if _, err := q.UpdatePaymentOrderTerms(ctx, store.UpdatePaymentOrderTermsParams{ID: byNo[order.OrderNo].ID, StoreID: order.StoreID, Amount: order.Amount}); err != nil {
					return err
				}
			}
			stored, err = q.PaymentOrdersOfPayment(ctx, row.ID)
			if err != nil {
				return err
			}
		}
		if row.BuyerID != buyerID || !sameTerms(stored, orders) {
			return apperr.FailedPrecondition("訂單內容在付款建立之後變動了")
		}
		if row.Status == string(model.PaymentCreating) {
			if err := q.EnsureEffectiveAttempt(ctx, store.EnsureEffectiveAttemptParams{ID: s.d.NewID(), PaymentID: row.ID, Now: now}); err != nil {
				return err
			}
			return enqueueCreateAt(ctx, s.d.Jobs, row.ID, now.Add(createJobDelay))
		}
		return nil
	})
	return row, err
}

type existingPayment struct{ paymentID uuid.UUID }

// existingGroup 回傳這些訂單已屬於的付款：都還沒有付款回 nil；訂單集合與一筆既有付款不完全相同（部分有付款、分屬不同付款、付款底下還有別的訂單）回 FailedPrecondition。
func existingGroup(ctx context.Context, q *store.Queries, orders []Order) (*existingPayment, error) {
	var paymentID uuid.UUID
	found := 0
	for _, order := range orders {
		o, err := q.PaymentOrderByOrderNo(ctx, order.OrderNo)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if found > 0 && o.PaymentID != paymentID {
			return nil, apperr.FailedPrecondition("這些訂單已分屬不同的付款")
		}
		paymentID = o.PaymentID
		found++
	}
	if found == 0 {
		return nil, nil
	}
	stored, err := q.PaymentOrdersOfPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if found != len(orders) || len(stored) != len(orders) {
		return nil, apperr.FailedPrecondition("訂單已屬於另一筆付款，訂單集合不符")
	}
	return &existingPayment{paymentID: paymentID}, nil
}

// sameTerms 回傳已存的訂單應收與這次讀到的訂單是否逐張相同（店家與金額）。
func sameTerms(stored []store.PaymentPaymentOrder, orders []Order) bool {
	byNo := make(map[string]store.PaymentPaymentOrder, len(stored))
	for _, o := range stored {
		byNo[o.OrderNo] = o
	}
	for _, order := range orders {
		o, ok := byNo[order.OrderNo]
		if !ok || o.StoreID != order.StoreID || o.Amount != order.Amount {
			return false
		}
	}
	return len(stored) == len(orders)
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
		s.d.Logger.WarnContext(ctx, "payment：請求內建立 PaymentIntent 沒有結果，回 pending 由補做 job 接手", "payment_id", row.ID.String(), "err", err)
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
		s.d.Logger.WarnContext(ctx, "payment：請求內建立後讀不到 client_secret，由 client 重讀", "payment_id", row.ID.String(), "err", verr)
	}
	return s.toModelWithOrders(ctx, res.row, res.clientSecret)
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
	out, err := s.toModelWithOrders(ctx, row, "")
	if err != nil {
		return model.Payment{}, err
	}
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

// toModelWithOrders 讀付款底下的訂單應收並把列轉成對外的付款。
func (s *Service) toModelWithOrders(ctx context.Context, r store.PaymentPayment, clientSecret string) (model.Payment, error) {
	orders, err := store.New(s.d.DB.Conn(ctx)).PaymentOrdersOfPayment(ctx, r.ID)
	if err != nil {
		return model.Payment{}, err
	}
	return toModel(r, orders, clientSecret), nil
}

// toPaymentOrders 把訂單應收列轉成對外的視圖。
func toPaymentOrders(rows []store.PaymentPaymentOrder) []model.PaymentOrder {
	out := make([]model.PaymentOrder, len(rows))
	for i, o := range rows {
		out[i] = model.PaymentOrder{ID: o.ID, OrderNo: o.OrderNo, StoreID: o.StoreID, Amount: o.Amount}
	}
	return out
}

// toModel 把列與它底下的訂單應收轉成對外的付款。
func toModel(r store.PaymentPayment, orders []store.PaymentPaymentOrder, clientSecret string) model.Payment {
	p := model.Payment{
		ID: r.ID, BuyerID: r.BuyerID, Amount: r.Amount, Provider: r.Provider,
		Status: model.PaymentStatus(r.Status), PaidAt: r.PaidAt, CreatedAt: r.CreatedAt, ClientSecret: clientSecret, Orders: toPaymentOrders(orders),
	}
	if r.LastError != nil {
		p.LastError = *r.LastError
	}
	return p
}

// handleEvent 是付款事件的 inbox handler（派送 job 的 transaction 內，不能呼叫 provider）：PaymentIntent 對得到本地付款時排入該付款的同步 job（以付款 ID 為唯一鍵），退款 ID 對得到本地退款時排入該退款的同步 job（以退款 ID 為唯一鍵），
// 不標記已處理，也不讀事件內容：狀態一律由同步重讀得到。對不到本地付款的事件只留紀錄、直接標記已處理（付款列的外部 ID 還沒寫進來時，建立的結果本身就是最新的快照）。
func (s *Service) handleEvent(ctx context.Context, e inbox.Event) error {
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentByExternal(ctx, store.PaymentByExternalParams{Provider: e.Provider, ExternalPaymentID: &e.RouteKey})
	if errors.Is(err, pgx.ErrNoRows) {
		refund, rerr := store.New(s.d.DB.Conn(ctx)).RefundByExternal(ctx, store.RefundByExternalParams{Provider: e.Provider, ExternalRefundID: &e.RouteKey})
		if errors.Is(rerr, pgx.ErrNoRows) {
			return s.d.Inbox.MarkProcessed(ctx, e.Provider, e.EventID, eventProcessor)
		}
		if rerr != nil {
			return rerr
		}
		return enqueueRefundSync(ctx, s.d.Jobs, refund.ID)
	}
	if err != nil {
		return err
	}
	return enqueueSync(ctx, s.d.Jobs, row.ID)
}
