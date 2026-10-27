package orderprovisioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/commerce"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/outbox"
)

const (
	// maxAttempts 是重試上限；attempt_count 達到它就轉終態 failed，不再重排。
	// 第一次呼叫是 attempt 1。
	//
	// ⛔ 沒有上限＝無限重試：一筆因設定缺失或參數不合法而永久失敗的 acceptance，會讓
	// worker 每輪重跑它。上限是它唯一的出口（doc 64 §6.4.14）。
	maxAttempts = 5
	// baseBackoff 是 backoff 的單位間隔：延遲 = 2^attempt * baseBackoff。
	baseBackoff = 1 * time.Minute
	// maxBackoff 封頂每次重試的延遲，不論 attempt 多大。
	maxBackoff = 1 * time.Hour
	// fetchBatch 限制每次 RunOnce 撈出的候選列數。
	fetchBatch = 50
	// claimLease 沿用 publicationoutbox 的現值，不另調——兩者的失敗形態相同（跨服務
	// 呼叫的重試）。這條路徑一列是一次 gRPC 建單，上界是 client 自己的 deadline
	// （grpcclient.Timeout5s），3 分鐘留了約一個半數量級的餘裕。
	claimLease = 3 * time.Minute

	// outboxName 是 pkg/outbox.Metrics 的 outbox_name 標籤值。
	outboxName = "rfq_order_provisioning"
)

// order_failure_reason 的值域（doc 64 §6.4.14）。
//
// ⚠️ 它們**還不是** CodeMeta 生成的常數：`ProcurementOrderProvisioningFailureReason`
// 的 producer 要改 `docs/代碼管理（CodeMeta）/`，而該目錄目前被物流線的四組缺失值域
// 卡著（lefthook 的 codemeta-copies glob 會連坐擋下任何碰它的 commit）。字面值逐字取自
// 設計書那張表 ⇒ 值域補上時是一對一映射，既有列不需要改寫。
// ⛔ 不要在這裡自己發明第二套數字碼（doc 57 §11 已否決）。
const (
	// FailureReasonConfigurationMissing：PROCUREMENT_SETTLEMENT_STORE_ID 未設定。
	// ⚠️ 2026-09-21 起只剩這一個：PAYUNI_AGENT_MER_ID 隨 EXTERNAL lane 的受託代銷形狀
	// 一起退場，order-service 不再讀它。補設定後需人工重放，重跑不會自癒。
	FailureReasonConfigurationMissing = "CONFIGURATION_MISSING"
	// FailureReasonUnknownLane：lane 不是 PLATFORM 也不是 EXTERNAL（fail-closed）。
	FailureReasonUnknownLane = "UNKNOWN_LANE"
	// FailureReasonOrderServiceRejected：order-service 回終態拒絕的其餘情形。
	FailureReasonOrderServiceRejected = "ORDER_SERVICE_REJECTED"
	// FailureReasonRetryExhausted：暫時性錯誤累積達 maxAttempts。排除根因後可重放。
	FailureReasonRetryExhausted = "RETRY_EXHAUSTED"
)

// orderServiceInternalErrorCode 是 GrpcResponseCode.INTERNAL_ERROR(500)。
//
// ⚠️ 通用錯誤碼那一段（0／400／500…）不在 pkg/errors/bizcode 的生成範圍內——實查該檔
// 只有各服務的業務碼，沒有通用段 ⇒ 這裡只能自己宣告。⛔ 不要為了它去動 CodeMeta：
// 那會觸發 codemeta-copies 的連坐 gate，而這是一個讀取端常數、不是新值域。
const orderServiceInternalErrorCode int32 = 500

// RfqOrderCommand 是投遞給 order-service 的建單命令。
//
// ⛔ **刻意沒有 SkuID 欄位**：RFQ 明細是 non-catalog line（doc 64 §6.4.9），投遞端不帶
// sku_id。把它做成「結構上帶不了」而不是「記得別填」——這是 Java 那側
// CreateOrderItemParam.catalog 的同一道守門在 Go 這一端的對應物。
type RfqOrderCommand struct {
	AcceptanceID int64
	BuyerID      identity.BuyerID
	// StoreID 是訂單賣方：PLATFORM ＝ 回價那個平台賣家自己的 store，
	// EXTERNAL ＝ 平台結算 store（PROCUREMENT_SETTLEMENT_STORE_ID）。
	StoreID int64
	// TotalAmount ＝ goods + shipping + tax（chk_prbqa_price_is_sum 已在 DB 保證）。
	TotalAmount int64
	ShippingFee int64
	// UnitPrice ＝ goods + tax。order_items 恆為一列且 quantity = 1（§6.4.3）。
	UnitPrice int64
	// SkuName 是真實成交品名，含數量與單位。⛔ 不得為 sentinel。
	SkuName string
	Lane    string
}

// RfqOrderResult 是 order-service 的回應。Code 原樣帶上來讓呼叫端自己分類終態與暫時性
// ——分類是業務判斷，不該埋在轉接層。
type RfqOrderResult struct {
	Code            int32
	Message         string
	OrderNo         string
	PaymentDeadline time.Time
	Replayed        bool
}

// RfqOrderCreator 是 worker 對 order-service 的全部需求。
// 由 internal/client/order.Client 實作，綁定寫在 procurement/module.go。
type RfqOrderCreator interface {
	CreateRfqOrder(ctx context.Context, cmd RfqOrderCommand) (RfqOrderResult, error)
}

// terminalRejection 讓 ProcessFunc 內部把「order-service 終態拒絕」與「暫時性錯誤」分開
// 傳遞。⛔ 它不會被回給 pkg/outbox.Worker——回 error 代表要重試，而終態拒絕重試幾次
// 結果都一樣。
type terminalRejection struct {
	reason string
	detail string
}

func (e *terminalRejection) Error() string { return e.reason + ": " + e.detail }

// Worker 排空 procurement_order_provisioning_outbox：把每一列建單命令送進 order-service
// 的 CreateRfqOrder，並把對應的 acceptance 從 CREATING_ORDER 推進到終態。
// claim 搶佔、backoff、終態處理全部在 pkg/outbox.Worker[*Row] 裡。
type Worker struct {
	inner *outbox.Worker[*Row]
}

// NewWorker 建構 worker。settlementStoreID 為 0 代表 PROCUREMENT_SETTLEMENT_STORE_ID
// 未設定——那不會讓 worker 整個停擺，只有 EXTERNAL lane 的列會終態失敗並記下
// CONFIGURATION_MISSING，PLATFORM lane 照常運作。
//
// ⚠️ 這與 publicationoutbox 對「RabbitMQ URL 沒設定」的處置刻意不同：那一支整個 tick
// 跳過，因為沒有 broker 時**每一列**都送不出去，先 claim 再失敗只會白燒 attempt_count。
// 這裡缺的設定只影響兩條 lane 中的一條，整批跳過會連帶把能做的那條也停掉。
func NewWorker(
	repo *Repository, store AcceptanceStore, orders RfqOrderCreator,
	settlementStoreID int64, metrics *outbox.Metrics, logger *zap.Logger,
) *Worker {
	if metrics == nil {
		panic("orderprovisioning.NewWorker: metrics must not be nil")
	}

	p := &processor{store: store, orders: orders, settlementStoreID: settlementStoreID, logger: logger}

	hooks := outbox.Hooks[*Row]{
		OnAttemptFailed: func(_ context.Context, row *Row, cause string, err error) {
			logger.Warn("rfq-order-provisioning-outbox: attempt failed",
				zap.Int64("id", row.ID), zap.Int64("acceptance_id", row.AcceptanceID),
				zap.String("cause", cause), zap.Error(err))
			metrics.AttemptFailedTotal(cause).Inc()
		},
		OnTerminalFailed: func(_ context.Context, row *Row, newAttempt int, lastErr string, markFailedErr error) {
			if markFailedErr != nil {
				// MarkFailed 沒寫成 ⇒ outbox 列仍是 pending、acceptance 仍是
				// CREATING_ORDER（兩者同一交易，見 Repository.MarkFailed），下一輪重試整段。
				logger.Warn("rfq-order-provisioning-outbox: MarkFailed error (degraded)",
					zap.Int64("id", row.ID), zap.Int64("acceptance_id", row.AcceptanceID),
					zap.Int("attempt", newAttempt), zap.String("last_error", lastErr),
					zap.Error(markFailedErr))
				return
			}
			logger.Warn("rfq-order-provisioning-outbox: row exhausted all retries → failed",
				zap.Int64("id", row.ID), zap.Int64("acceptance_id", row.AcceptanceID),
				zap.Int("attempt", newAttempt), zap.String("last_error", lastErr))
			metrics.FailedTotal().Inc()
		},
	}

	cfg := outbox.Config{
		MaxAttempts: maxAttempts,
		BaseBackoff: baseBackoff,
		MaxBackoff:  maxBackoff,
		FetchBatch:  fetchBatch,
		ClaimLease:  claimLease,
	}

	return &Worker{inner: outbox.New[*Row](repo, p.process, cfg, hooks)}
}

// RunOnce 處理一批 pending+due 的列後返回，由 scheduler 每個 tick 呼叫一次。
func (w *Worker) RunOnce(ctx context.Context) error { return w.inner.RunOnce(ctx) }

type processor struct {
	store             AcceptanceStore
	orders            RfqOrderCreator
	settlementStoreID int64
	logger            *zap.Logger
}

// process 是 doc 64 §6.4.13 的四步。
//
//  1. 讀 acceptance
//  2. 已是終態 ⇒ 直接 MarkDone（不重複建單）
//  3. 呼叫 CreateRfqOrder（帶 lane，⛔ 不帶 sku_id）
//  4. CAS 回寫 ORDER_CREATED + order_no（影響 0 列＝別的副本先寫了，視為成功）
//
// ⛔ **順序固定：先建單、後回寫狀態**，不可對調。先回寫 ORDER_CREATED 再建單，一旦建單
// 失敗就留下「說訂單建好了但沒有」——而 chk_prbqa_order_no_matches_state 強制
// order_no 與該狀態同進退，根本寫不進去。這個順序的最壞後果是「訂單已建、acceptance 仍
// CREATING_ORDER」，下一輪由第 2 步的冪等回查接手，方向是低估而非說謊。
//
// 回 error ＝ 暫時性、要重試；回 nil ＝ 這一列處理完了（含終態失敗，見
// markTerminalFailure）。
func (p *processor) process(ctx context.Context, row *Row) (any, error) {
	acceptance, err := p.loadAcceptance(ctx, row)
	if err != nil {
		var rejection *terminalRejection
		if errors.As(err, &rejection) {
			return nil, p.markTerminalFailureByOutbox(ctx, row, rejection)
		}
		return nil, err
	}

	switch acceptance.OrderProvisioningState {
	case repository.OrderProvisioningCreated:
		// 前一輪建完單但 MarkDone 沒寫成 ⇒ 這一列重跑。訂單已經在了，直接讓框架 MarkDone。
		return nil, nil
	case repository.OrderProvisioningFailed:
		// 終態。⛔ 不得再送一次建單命令——acceptance 的兩個終態都沒有回頭邊（§6.4.13）。
		return nil, nil
	}

	cmd, err := p.buildCommand(ctx, acceptance)
	if err != nil {
		var rejection *terminalRejection
		if errors.As(err, &rejection) {
			return nil, p.markTerminalFailure(ctx, acceptance, rejection)
		}
		return nil, err
	}

	result, err := p.orders.CreateRfqOrder(ctx, cmd)
	if err != nil {
		// 傳輸層錯誤（連不上、deadline）一律當暫時性：order-service 的業務判定都會回
		// 帶 code 的正常回應，走不到這裡。
		return nil, fmt.Errorf("create rfq order: %w", err)
	}
	retryable, rejection := classifyResponse(result)
	if retryable {
		// ⚠️ 回 error 而不是就地寫終態：這一類要走退避重試，耗盡才由 MarkFailed 落
		// RETRY_EXHAUSTED。⛔ 也不可 fall through 到下面的 OrderNo 檢查——失敗回應本來
		// 就沒有訂單號，那會把「系統暫時性錯誤」誤判成「契約被破壞」。
		return nil, fmt.Errorf("order-service returned a retryable error [%d]: %s", result.Code, result.Message)
	}
	if rejection != nil {
		return nil, p.markTerminalFailure(ctx, acceptance, rejection)
	}
	if result.OrderNo == "" || result.PaymentDeadline.IsZero() {
		// code=0 但少了訂單號或付款期限：契約被破壞，重試不會變好，但也不該靜默當成功。
		// ⚠️ 付款期限一起擋是因為它會被寫進 acceptance 給買家看——零值會變成
		// 0001-01-01，那是一個「已經過期」的期限，而寫入端不會有任何錯誤。
		return nil, p.markTerminalFailure(ctx, acceptance, &terminalRejection{
			reason: FailureReasonOrderServiceRejected,
			detail: "order-service returned success without an order number or payment deadline",
		})
	}

	// CAS 影響 0 列不是錯誤：代表另一個副本先寫了（outbox 的 at-least-once 本來就允許
	// 兩個副本同時跑到這一步）。
	_, err = p.store.AttachOrderToAcceptance(ctx, acceptance.ID, acceptance.Version,
		result.OrderNo, result.PaymentDeadline)
	if err != nil && !errors.Is(err, repository.ErrBuyerQuoteAcceptanceStale) {
		return nil, fmt.Errorf("attach order to acceptance: %w", err)
	}
	return nil, nil
}

// loadAcceptance 解出 payload 的身分並讀回 acceptance。
func (p *processor) loadAcceptance(ctx context.Context, row *Row) (repository.BuyerQuoteAcceptance, error) {
	var payload commerce.OrderShellRequestedPayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		// payload 是不可變的已提交 bytes：解不開重試幾次都一樣。
		return repository.BuyerQuoteAcceptance{}, &terminalRejection{
			reason: FailureReasonOrderServiceRejected,
			detail: "unmarshal outbox payload: " + err.Error(),
		}
	}
	buyerID, err := strconv.ParseInt(payload.BuyerID, 10, 64)
	if err != nil {
		return repository.BuyerQuoteAcceptance{}, &terminalRejection{
			reason: FailureReasonOrderServiceRejected,
			detail: "parse buyerId from outbox payload: " + err.Error(),
		}
	}
	// ⚠️ 用 row.AcceptanceID（表上的業務鍵欄位）而不是 payload 裡那份：uq_propo_acceptance_event
	// 綁的是欄位，payload 只是副本。buyerID 才需要從 payload 取——acceptance 的讀取一律
	// 帶買家範圍，沒有第二支「不分買家」的查詢，也不該為 worker 開一支。
	acceptance, err := p.store.FindBuyerQuoteAcceptanceByIDAndBuyer(ctx, row.AcceptanceID, identity.BuyerID(buyerID))
	if err != nil {
		if errors.Is(err, repository.ErrBuyerQuoteAcceptanceNotFound) {
			return repository.BuyerQuoteAcceptance{}, &terminalRejection{
				reason: FailureReasonOrderServiceRejected,
				detail: "acceptance not found for outbox row",
			}
		}
		return repository.BuyerQuoteAcceptance{}, err
	}
	return acceptance, nil
}

// buildCommand 依 lane 決定賣方，並從被接受的那一版報價組出品名與金額。
func (p *processor) buildCommand(ctx context.Context, a repository.BuyerQuoteAcceptance) (RfqOrderCommand, error) {
	storeID, err := p.resolveStoreID(a)
	if err != nil {
		return RfqOrderCommand{}, err
	}

	version, err := p.store.FindSellerResponseVersion(ctx, a.SellerResponseID, a.SellerResponseVersion)
	if err != nil {
		if errors.Is(err, repository.ErrSellerResponseNotFound) {
			return RfqOrderCommand{}, &terminalRejection{
				reason: FailureReasonOrderServiceRejected,
				detail: "accepted seller response version not found",
			}
		}
		return RfqOrderCommand{}, err
	}
	snapshot, err := p.store.FindConfirmedRfqSnapshotByIDAndBuyer(ctx, a.ConfirmedRfqSnapshotID, a.BuyerID)
	if err != nil {
		if errors.Is(err, repository.ErrConfirmedRfqSnapshotNotFound) {
			return RfqOrderCommand{}, &terminalRejection{
				reason: FailureReasonOrderServiceRejected,
				detail: "confirmed rfq snapshot not found",
			}
		}
		return RfqOrderCommand{}, err
	}

	skuName, err := domain.ComposeRfqOrderLineName(snapshot.CanonicalPayloadJSON,
		version.OfferedQuantity, version.OfferedQuantityUnit)
	if err != nil {
		// canonical payload 不可變 ⇒ 組不出品名是終態，不是抖動。
		return RfqOrderCommand{}, &terminalRejection{
			reason: FailureReasonOrderServiceRejected,
			detail: err.Error(),
		}
	}

	return RfqOrderCommand{
		AcceptanceID: a.ID,
		BuyerID:      a.BuyerID,
		StoreID:      storeID,
		TotalAmount:  a.AcceptedDealPrice,
		ShippingFee:  a.ShippingAmount,
		// unit_price ＝ goods + tax；運費另計在 shipping_fee，03 §3.2 的對帳式因此成立。
		UnitPrice: a.GoodsAmount + a.TaxAmount,
		SkuName:   skuName,
		Lane:      a.Lane,
	}, nil
}

// resolveStoreID 是 lane 在投遞端唯一的分歧點（doc 64 §6.4 適用範圍）。
func (p *processor) resolveStoreID(a repository.BuyerQuoteAcceptance) (int64, error) {
	switch a.Lane {
	case repository.SellerResponseLanePlatform:
		if a.StoreID == nil {
			// chk_prbqa_identity_matches_lane 保證 PLATFORM 必有 store_id；真的為 nil
			// 代表約束被繞過，不是可重試的情形。
			return 0, &terminalRejection{
				reason: FailureReasonOrderServiceRejected,
				detail: "PLATFORM lane acceptance has no store_id",
			}
		}
		return *a.StoreID, nil
	case repository.SellerResponseLaneExternal:
		if p.settlementStoreID == 0 {
			return 0, &terminalRejection{
				reason: FailureReasonConfigurationMissing,
				detail: "PROCUREMENT_SETTLEMENT_STORE_ID is not configured",
			}
		}
		return p.settlementStoreID, nil
	default:
		// 🔴 fail-closed，不猜：這一格的輸出會成為撥款與開票的法源（§6.4 適用範圍）。
		return 0, &terminalRejection{
			reason: FailureReasonUnknownLane,
			detail: "unknown lane " + strconv.Quote(a.Lane),
		}
	}
}

// classifyResponse 把 order-service 的回應碼分成**三**類：成功、可重試、終態拒絕。
//
// ⚠️ 三類而不是兩類是刻意的。只分「成功／失敗」時，暫時性錯誤只能歸進其中一邊：歸成功
// 會落進「code=0 卻沒有訂單號」的契約破壞分支、歸失敗會把一次系統抖動變成永久
// ORDER_FAILED。兩種都錯，而且都只在線上才看得到。
//
// ⛔ DuplicateKey **不在**終態那一邊：order-service 對 uk_orders_acceptance_id 的衝突
// 已回查既有訂單當冪等命中（code=0、replayed=true），根本不會回錯誤碼給我們。這條註解
// 留著是因為「把它當失敗」會把 acceptance 標成 ORDER_FAILED 而訂單其實建好了——那是這
// 條路徑最貴的一個錯法（doc 64 §6.4.13）。
func classifyResponse(result RfqOrderResult) (retryable bool, rejection *terminalRejection) {
	switch result.Code {
	case 0:
		return false, nil
	case orderServiceInternalErrorCode:
		// 系統錯誤是抖動的候選 ⇒ 交給 backoff 重試，耗盡才落 RETRY_EXHAUSTED。
		return true, nil
	case bizcode.FulfillmentCommerceModelUnknown:
		return false, &terminalRejection{reason: FailureReasonUnknownLane, detail: result.Message}
	case bizcode.PlatformCollectionNotConfigured:
		return false, &terminalRejection{reason: FailureReasonConfigurationMissing, detail: result.Message}
	default:
		return false, &terminalRejection{reason: FailureReasonOrderServiceRejected, detail: result.Message}
	}
}

// markTerminalFailure 把 acceptance CAS 成 ORDER_FAILED 並記下成因，然後回 nil 讓框架
// MarkDone——這一列已經處理完了，重試不會有不同結果（§6.4.13 的「終態拒絕」分支）。
//
// 影響 0 列（ErrBuyerQuoteAcceptanceStale）代表別的副本先寫了，視為成功。
func (p *processor) markTerminalFailure(ctx context.Context, a repository.BuyerQuoteAcceptance, rejection *terminalRejection) error {
	p.logger.Warn("rfq-order-provisioning-outbox: terminal rejection",
		zap.Int64("acceptance_id", a.ID), zap.String("reason", rejection.reason),
		zap.String("detail", rejection.detail))
	_, err := p.store.MarkAcceptanceOrderFailed(ctx, a.ID, a.Version, rejection.reason)
	if err != nil && !errors.Is(err, repository.ErrBuyerQuoteAcceptanceStale) {
		// 寫不進去就回 error：讓這一列重試，而不是讓 acceptance 永遠停在 CREATING_ORDER。
		return fmt.Errorf("mark acceptance order failed: %w", err)
	}
	return nil
}

// markTerminalFailureByOutbox 是「連 acceptance 都讀不回來」時的同一件事——沒有 version
// 可以 CAS，改用 order_provisioning_state 當述詞。
func (p *processor) markTerminalFailureByOutbox(ctx context.Context, row *Row, rejection *terminalRejection) error {
	p.logger.Warn("rfq-order-provisioning-outbox: terminal rejection before acceptance load",
		zap.Int64("id", row.ID), zap.Int64("acceptance_id", row.AcceptanceID),
		zap.String("reason", rejection.reason), zap.String("detail", rejection.detail))
	if err := p.store.MarkAcceptanceOrderFailedByOutboxID(ctx, row.ID, rejection.reason); err != nil {
		return fmt.Errorf("mark acceptance order failed by outbox id: %w", err)
	}
	return nil
}
