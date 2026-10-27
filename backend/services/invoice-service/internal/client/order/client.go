// Package order 是 invoice-service 對 order-service 的出站 peer client（58 §6.2）。
//
// 只暴露一支 RPC：`order_no → 訂單發票快照`（§7.1.2 步驟 5 ①）。它回答的是
// 「這張發票要開哪些品項、哪些金額、開給誰」——品項與買家發票偏好刻意不上 MQ
// （§6.3.2.6）：前者必須是「開票當下」的不可變快照，而 MQ 的 at-least-once 重送會讓
// 「當前的 X」在重試時漂掉。
//
// 🔴 本 package 另外承擔一件 proto 給不了的事：**載具解析**（carrier.go）。
// `order_invoice_snapshots` 零 PII ⇒ 載具值不在 proto 裡，必須在填 OrderFacts 之前
// 由本層自己解析並驗歸屬（I-47）。
package order

import (
	"context"
	stderrors "errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrOrderSnapshotNotFound 表示 order-service 查無此訂單、或該訂單沒有發票快照列。
//
// 處置與「order-service 掛了」相同：往上拋、讓 order.paid 重試三次後進 DLQ。
// proto 註解逐字寫明「不得回 code=0 ＋ 空 data」，理由是空 data 會被當成「這張訂單
// 沒有品項」而開出一張金額為 0 的發票，而發票不可回收。
var ErrOrderSnapshotNotFound = stderrors.New("invoice: order invoice snapshot not found")

// ErrOrderSnapshotVersionConflict ＝ order-service 回 9308（§6.2.5 的樂觀鎖，或並行撞
// uk_order_invoice_snapshots）。呼叫端**必須重讀再重試**，不得把它當成一般失敗吞掉：
// 吞掉之後 R03 會以為快照已更新，而 order-service 那邊什麼都沒寫。
var ErrOrderSnapshotVersionConflict = stderrors.New("invoice: order invoice snapshot version conflict")

// DefaultDeadline 是本 client 每一次呼叫的 per-call deadline（§6.2.0「一般讀寫」3 s）。
//
// 🔴 不得改成 0 或拿掉：`grpc.NewClient` 對沒有 deadline 的呼叫不會自己補一個。
// 本 client 的呼叫點是 MQ consumer 的 handler，而 pkg/mq 是逐則序列處理 ⇒ 一次無限
// 掛住等於整條 order.paid 訂閱停擺，且沒有任何錯誤訊號。
const DefaultDeadline = 3 * time.Second

// Client 包住 OrderService stub ＋ 載具解析所需的 buyer preference reader。
type Client struct {
	stub     pb.OrderServiceClient
	prefs    BuyerPreferenceReader
	deadline time.Duration
}

// 編譯期斷言：本 client 必須滿足 consumer 宣告的 port。
var _ invoicemq.OrderFactsClient = (*Client)(nil)

// NewClient 撥號 order-service。interceptor 與 lifecycle 的理由見 payment client 同名函式。
func NewClient(lc fx.Lifecycle, addr string, prefs BuyerPreferenceReader) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoice: order peer addr is empty (ORDER_SERVICE_ADDR)")
	}
	if prefs == nil {
		return nil, fmt.Errorf("invoice: order client requires a BuyerPreferenceReader (carrier resolution, 58 §5.4.3)")
	}
	conn, err := grpcclient.Dial(addr,
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("invoice-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("invoice: dial order-service %s: %w", addr, err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return conn.Close() }})
	return NewClientWithStub(pb.NewOrderServiceClient(conn), prefs, DefaultDeadline), nil
}

// NewClientWithStub 讓測試注入 stub（bufconn）與較短的 deadline。
func NewClientWithStub(stub pb.OrderServiceClient, prefs BuyerPreferenceReader, deadline time.Duration) *Client {
	if deadline <= 0 {
		deadline = DefaultDeadline
	}
	return &Client{stub: stub, prefs: prefs, deadline: deadline}
}

// GetOrderInvoiceSnapshot 取訂單發票快照並解析載具。
//
// buyerID **只能取自訂單**（order.paid payload 的 `buyer_id`，其值由 order-service
// 自 `orders.buyer_id` 寫出）——它是 I-47 歸屬驗證的述詞之一，不得取自任何請求欄位。
//
// 🔴 三條 fail-loud（同 payment client 的理由）：下游 error 往上拋、envelope code != 0
// 視為 not found、data == nil 視為 not found。任何一條退化成回零值都會開出一張
// 沒有品項、金額為 0 的發票。
func (c *Client) GetOrderInvoiceSnapshot(ctx context.Context, orderNo string, buyerID int64) (invoicemq.OrderFacts, error) {
	if orderNo == "" {
		return invoicemq.OrderFacts{}, fmt.Errorf("invoice: get order invoice snapshot: empty order_no")
	}

	callCtx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.GetOrderInvoiceSnapshotInternal(callCtx,
		&pb.GetOrderInvoiceSnapshotInternalRequest{OrderNo: orderNo})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return invoicemq.OrderFacts{}, fmt.Errorf("%w (order_no=%s)", ErrOrderSnapshotNotFound, orderNo)
		}
		return invoicemq.OrderFacts{}, fmt.Errorf("invoice: get order invoice snapshot (order_no=%s): %w", orderNo, err)
	}
	if code := resp.GetCode(); code != 0 {
		return invoicemq.OrderFacts{}, fmt.Errorf(
			"invoice: get order invoice snapshot (order_no=%s): downstream code=%d message=%q: %w",
			orderNo, code, resp.GetMessage(), ErrOrderSnapshotNotFound)
	}
	data := resp.GetData()
	if data == nil {
		return invoicemq.OrderFacts{}, fmt.Errorf(
			"invoice: get order invoice snapshot (order_no=%s): code=0 with nil data: %w", orderNo, ErrOrderSnapshotNotFound)
	}

	facts, err := mapSnapshot(data)
	if err != nil {
		return invoicemq.OrderFacts{}, fmt.Errorf("invoice: get order invoice snapshot (order_no=%s): %w", orderNo, err)
	}

	// 載具解析用**外層** ctx（不是 callCtx）：它是一次本地 DB 讀，不該被 gRPC 的
	// per-call deadline 綁住；而 callCtx 在本函式返回時就被 cancel 了。
	prefID, err := parseOptionalID(data.GetBuyerInvoicePreferenceId(), "buyer_invoice_preference_id")
	if err != nil {
		return invoicemq.OrderFacts{}, fmt.Errorf("invoice: get order invoice snapshot (order_no=%s): %w", orderNo, err)
	}
	res, err := resolveCarrier(ctx, c.prefs,
		data.GetCarrierSelectionMode(), prefID, buyerID,
		data.GetBuyerTaxId(), data.GetBuyerTitle(), data.GetDonationCode())
	if err != nil {
		return invoicemq.OrderFacts{}, fmt.Errorf("invoice: get order invoice snapshot (order_no=%s): %w", orderNo, err)
	}

	facts.CarrierType = res.CarrierType
	facts.CarrierID1 = res.CarrierID1
	facts.CarrierID2 = res.CarrierID2
	facts.DonationCode = res.DonationCode
	facts.BuyerTaxID = res.BuyerTaxID
	facts.BuyerTitle = res.BuyerTitle
	facts.PreferenceOwnershipMismatch = res.OwnershipMismatch
	return facts, nil
}

// ReviseOrderInvoiceSnapshot 是 R03（開立前直接改）的跨服務那一步：請 order-service
// 寫入一份新的買家發票選擇，回傳新的 snapshot_version。
//
// 🔴 **本服務沒有、也不得有任何遞增 snapshot_version 的路徑**（58 號 I-50）：
// 這個值只能從本函式的回傳複製。自己 +1 等於憑空造出第二個寫入者，而兩個寫入者對同一個
// 序號各自遞增的結果是「兩份都以為自己是最新版」。
//
// 🔴 **不傳 notify_email**（§6.2.5 定版 7）：snapshot_version 是內容軸，只承載發票上會
// 印出來的東西。通知信箱的變更由本服務就地改 invoice_documents.recipient_email，
// 不經過這裡——讓它遞增內容軸會使「同內容重開一張票」與「內容真的變了」在冪等鍵上
// 無法區分，而那正是 issue_attempt_seq 實體軸存在的理由。
//
// 三條 fail-loud 與 GetOrderInvoiceSnapshot 同構：下游 error 往上拋、9308 轉成可辨識的
// 版本衝突、其餘非 0 code 一律是錯。**成功卻回空版本號也視為錯**——那代表對方回了
// code=0 但沒寫東西，而呼叫端會拿空字串去更新 document。
func (c *Client) ReviseOrderInvoiceSnapshot(ctx context.Context, in domain.ReviseSnapshotInput) (int64, error) {
	if in.OrderNo == "" {
		return 0, fmt.Errorf("invoice: revise order invoice snapshot: empty order_no")
	}
	if in.IdempotencyKey == "" {
		// 空冪等鍵會讓每次重試都變成一次新的更正（order-service 側的 partial unique index
		// 對 NULL 不生效）⇒ 在這裡就擋掉，不要送出去。
		return 0, fmt.Errorf("invoice: revise order invoice snapshot (order_no=%s): empty idempotency_key", in.OrderNo)
	}

	callCtx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.ReviseOrderInvoiceSnapshotInternal(callCtx, &pb.ReviseOrderInvoiceSnapshotInternalRequest{
		OrderNo:                  in.OrderNo,
		ExpectedSnapshotVersion:  strconv.FormatInt(in.ExpectedSnapshotVersion, 10),
		InvoiceKind:              in.InvoiceKind,
		CarrierSelectionMode:     in.CarrierSelectionMode,
		BuyerInvoicePreferenceId: formatOptionalID(in.BuyerInvoicePreferenceID),
		BuyerTaxId:               in.BuyerTaxID,
		BuyerTitle:               in.BuyerTitle,
		DonationCode:             in.DonationCode,
		IdempotencyKey:           in.IdempotencyKey,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, fmt.Errorf("%w (order_no=%s)", ErrOrderSnapshotNotFound, in.OrderNo)
		}
		return 0, fmt.Errorf("invoice: revise order invoice snapshot (order_no=%s): %w", in.OrderNo, err)
	}
	switch code := resp.GetCode(); code {
	case 0:
		// 正常
	case orderSnapshotVersionConflictCode:
		return 0, fmt.Errorf("%w (order_no=%s expected_version=%d)",
			ErrOrderSnapshotVersionConflict, in.OrderNo, in.ExpectedSnapshotVersion)
	default:
		return 0, fmt.Errorf("invoice: revise order invoice snapshot (order_no=%s): downstream code=%d message=%q",
			in.OrderNo, code, resp.GetMessage())
	}

	raw := resp.GetSnapshotVersion()
	if raw == "" {
		return 0, fmt.Errorf(
			"invoice: revise order invoice snapshot (order_no=%s): code=0 with empty snapshot_version", in.OrderNo)
	}
	next, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invoice: revise order invoice snapshot (order_no=%s): bad snapshot_version %q: %w",
			in.OrderNo, raw, err)
	}
	if next <= in.ExpectedSnapshotVersion {
		// 對方宣稱成功，卻回了一個不比舊版新的號碼。這不是可以容忍的邊界情況：
		// 拿它去更新 document 會讓內容軸倒退，而冪等鍵是用它算的。
		return 0, fmt.Errorf(
			"invoice: revise order invoice snapshot (order_no=%s): snapshot_version %d not newer than %d",
			in.OrderNo, next, in.ExpectedSnapshotVersion)
	}
	return next, nil
}

// orderSnapshotVersionConflictCode ＝ CodeMeta INVOICE_SNAPSHOT_VERSION_CONFLICT（9308）。
// 寫成常數而不是 import bizcode：這裡讀的是 **order-service 回傳的** envelope code，
// 與本服務自己丟出的業務碼是兩件事，混用會讓「誰產生了這個碼」在讀 code 時失焦。
const orderSnapshotVersionConflictCode int32 = 9308

// formatOptionalID：0 ⇒ 空字串（proto 沒有 null，空字串就是「未提供」）。
func formatOptionalID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

// mapSnapshot 把 proto 的字串數字攤成 OrderFacts 的原生型別。
//
// 每一次轉換都 fail-loud：`strconv` 失敗回 error 而不是零值。金額欄位吞成 0 的後果
// 是開出一張金額錯誤的發票（不可回收），而不是一次 500。
func mapSnapshot(d *pb.OrderInvoiceSnapshotData) (invoicemq.OrderFacts, error) {
	version, err := parseRequiredInt(d.GetSnapshotVersion(), "snapshot_version")
	if err != nil {
		return invoicemq.OrderFacts{}, err
	}
	if version < 1 {
		// chk_ois_version；I-50 說本服務只能從回傳值複製，不能自己遞增，也就不能自己補值。
		return invoicemq.OrderFacts{}, fmt.Errorf("snapshot_version must be >= 1, got %d", version)
	}
	original, err := parseRequiredInt(d.GetTotalOriginalAmount(), "total_original_amount")
	if err != nil {
		return invoicemq.OrderFacts{}, err
	}
	shipping, err := parseRequiredInt(d.GetShippingFee(), "shipping_fee")
	if err != nil {
		return invoicemq.OrderFacts{}, err
	}
	discount, err := parseRequiredInt(d.GetDiscountAmount(), "discount_amount")
	if err != nil {
		return invoicemq.OrderFacts{}, err
	}
	actual, err := parseRequiredInt(d.GetTotalActualAmount(), "total_actual_amount")
	if err != nil {
		return invoicemq.OrderFacts{}, err
	}

	items, err := mapItems(d.GetItems())
	if err != nil {
		return invoicemq.OrderFacts{}, err
	}

	funding := fundingFromWire(d.GetDiscountFunding())

	return invoicemq.OrderFacts{
		SnapshotVersion: int32(version), //nolint:gosec // 值域由上面的 >= 1 與 chk_ois_version 守
		Snapshot: domain.OrderSnapshot{
			TotalOriginalAmount: original,
			ShippingFee:         shipping,
			DiscountAmount:      discount,
			TotalActualAmount:   actual,
			DiscountFunding:     funding,
			Items:               items,
		},
		NotifyEmail: d.GetNotifyEmail(),
		// 🔴 fail-closed：`discount_funding` 為 "" ＝ order-service 判不出來（U-7），
		// **不得**當成 SELLER。把平台補貼的那一段從賣家銷售額扣掉 ＝ 賣家短漏報銷售額。
		// 折扣為 0 時資金來源無意義，此格才不成立。
		HasPlatformFundedDiscount: discount > 0 && funding != domain.DiscountFundingSeller,
		IsConsolidatedOrder:       d.GetIsConsolidatedOrder(),
	}, nil
}

func mapItems(in []*pb.OrderInvoiceSnapshotItem) ([]domain.OrderItem, error) {
	// 順序即 line_seq（§5A.3）：proto 註解要求依 order_items.id 升冪，本層原樣保留，
	// 不重排也不依賴 map 迭代序。
	out := make([]domain.OrderItem, 0, len(in))
	for i, it := range in {
		itemID, err := parseRequiredInt(it.GetOrderItemId(), fmt.Sprintf("items[%d].order_item_id", i))
		if err != nil {
			return nil, err
		}
		// non-catalog line（RFQ 成交）沒有平台 SKU ⇒ order-service 輸出空字串。
		// 用 0 承載「無此值」與 Line.SkuID 的既有慣例一致，最終由 inp() 寫成 NULL。
		// ⚠️ 只有 sku_id 可缺：其餘數值欄位仍走 parseRequiredInt。
		skuID, err := parseOptionalInt(it.GetSkuId(), fmt.Sprintf("items[%d].sku_id", i))
		if err != nil {
			return nil, err
		}
		qty, err := parseRequiredInt(it.GetQuantity(), fmt.Sprintf("items[%d].quantity", i))
		if err != nil {
			return nil, err
		}
		unit, err := parseRequiredInt(it.GetUnitPrice(), fmt.Sprintf("items[%d].unit_price", i))
		if err != nil {
			return nil, err
		}
		if qty <= 0 || qty > 1<<31-1 {
			return nil, fmt.Errorf("items[%d].quantity out of range: %d", i, qty)
		}
		out = append(out, domain.OrderItem{
			OrderItemID: itemID,
			SkuID:       skuID,
			ProductName: it.GetProductName(),
			SkuName:     it.GetSkuName(),
			Quantity:    int32(qty), //nolint:gosec // 值域由上一行守
			UnitPrice:   unit,
		})
	}
	return out, nil
}

// fundingFromWire 把 proto 的三態字串換成 domain 值。
//
// 🔴 未知值一律回 Unknown（＝ fail-closed），**不得** default 成 Seller：
// A22 的判定是 `DiscountAmount > 0 && funding != Seller`，把未知當成 Seller
// 會讓平台出資的折扣靜默通過那道閘門。
func fundingFromWire(s string) domain.DiscountFunding {
	switch s {
	case "SELLER":
		return domain.DiscountFundingSeller
	case "PLATFORM":
		return domain.DiscountFundingPlatform
	default:
		return domain.DiscountFundingUnknown
	}
}

// parseRequiredInt 解析必填的數字字串。空字串是錯誤不是 0。
func parseRequiredInt(s, field string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("%s is empty (numeric fields are required, not optional)", field)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer: %w", field, s, err)
	}
	return v, nil
}

// parseOptionalInt 解析**允許缺值**的數值欄位：空字串回 0，交由 domain 的
// 「0 表示 NULL」慣例承載（Line.SkuID 的註解、repository 的 inp() 都照這條走）。
// ⚠️ 只對確實可缺的欄位使用。非空但非數字仍然是錯誤——那是上游壞了，不是缺值。
func parseOptionalInt(s, field string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer: %w", field, s, err)
	}
	return v, nil
}

// parseOptionalID 解析「"" ＝ NULL」的 id 欄位。空字串回 0，其餘必須是正整數。
func parseOptionalID(s, field string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer: %w", field, s, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("%s=%q must be positive when present", field, s)
	}
	return v, nil
}
