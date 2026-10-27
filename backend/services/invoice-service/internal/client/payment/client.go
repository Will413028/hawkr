// Package payment 是 invoice-service 對 payment-service 的出站 peer client（58 §6.2）。
//
// 只暴露一支 RPC：`order_no → 付款事實`（§7.1.2 步驟 5 ②、註 2）。它回答的是
// 「這張訂單的發票日期該取哪一個時間、金額對帳的基準是哪一筆交易」。
package payment

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrPaymentFactsNotFound 表示 payment-service 查無此訂單的 canonical 交易。
//
// 它與「payment-service 掛了」在 log 上必須分得開，但**兩者的處置相同**：往上拋、
// 讓 order.paid 重試三次後進 DLQ。查無交易而照樣開票 ＝ 發票日期退回本地時鐘、
// 金額對帳失去基準（proto 註解逐字寫明），而發票開出去不可回收。
var ErrPaymentFactsNotFound = stderrors.New("invoice: payment facts not found for order_no")

// DefaultDeadline 是本 client 每一次呼叫的 per-call deadline。
//
// 3 s ＝ 58 §6.2.0「一般讀寫（不打 AMEGO）」那一格：payment 側是一次
// `payment_transactions` 的索引查詢，不打 PayUni。
//
// 🔴 不得改成 0 或拿掉：`grpc.NewClient` 對沒有 deadline 的呼叫不會自己補一個，
// payment-service 卡住時這一跳會無限期掛住 MQ consumer 的 goroutine，而 pkg/mq 的
// consumer 是逐則序列處理 ⇒ 一次卡死等於整條 order.paid 訂閱停擺、且沒有錯誤訊號。
const DefaultDeadline = 3 * time.Second

// Client 包住 PaymentService stub。
type Client struct {
	stub     pb.PaymentServiceClient
	deadline time.Duration
}

// 編譯期斷言：本 client 必須滿足 consumer 宣告的 port。
var _ invoicemq.PaymentFactsClient = (*Client)(nil)

// NewClient 撥號 payment-service。
//
// 兩個 interceptor 缺一不可（§6.2.0 的 52 號 wiring 表）：
//   - InternalAuthClientInterceptor 帶 HMAC 四 header。缺它 ⇒ 對端在
//     `INTERNAL_AUTH_MODE=enforcing` 下每次都回 UNAUTHENTICATED，而 permissive
//     期間完全看不出來。
//   - OTelClientStatsHandler 讓這一跳留在 trace 上。
//
// forwardAccountHeaders = false：這是 MQ 觸發的系統呼叫，incoming context 沒有
// 使用者身分（§6.2.0 出站表逐條列明 order／payment 皆為 false）。
func NewClient(lc fx.Lifecycle, addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoice: payment peer addr is empty (PAYMENT_SERVICE_ADDR)")
	}
	conn, err := grpcclient.Dial(addr,
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("invoice-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("invoice: dial payment-service %s: %w", addr, err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return conn.Close() }})
	return NewClientWithStub(pb.NewPaymentServiceClient(conn), DefaultDeadline), nil
}

// NewClientWithStub 讓測試注入 stub（bufconn）與較短的 deadline。
//
// deadline <= 0 時退回 DefaultDeadline，不是「不設 deadline」。
func NewClientWithStub(stub pb.PaymentServiceClient, deadline time.Duration) *Client {
	if deadline <= 0 {
		deadline = DefaultDeadline
	}
	return &Client{stub: stub, deadline: deadline}
}

// GetPaymentFacts 取這張訂單 canonical 交易的付款事實。
//
// 🔴 五條 fail-loud，任何一條退化成「回零值 ＋ nil error」都會開出一張錯誤的發票：
//
//	① 下游 gRPC error ⇒ 原樣往上拋（NotFound 換成 ErrPaymentFactsNotFound）
//	② envelope code != 0 ⇒ error（proto 註解：查無交易一律回非 0，不得回 code=0 ＋ 空 data）
//	③ data == nil ⇒ error，不得當成「這張訂單沒有付款」
//	④ pay_time 解析失敗或為零值 ⇒ error。PaidAt() 的 COALESCE 尾端就是它，
//	   零值會讓 invoice_date 落在 0001-01-01，法定期限與所有告警一起錯
//	⑤ is_duplicate == true ⇒ error。payment 端選錯 canonical 交易（proto 註解逐字要求）
func (c *Client) GetPaymentFacts(ctx context.Context, orderNo string) (invoicemq.PaymentFacts, error) {
	if orderNo == "" {
		return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts: empty order_no")
	}

	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.GetPaymentFactsInternal(ctx, &pb.GetPaymentFactsInternalRequest{OrderNo: orderNo})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return invoicemq.PaymentFacts{}, fmt.Errorf("%w (order_no=%s)", ErrPaymentFactsNotFound, orderNo)
		}
		return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts (order_no=%s): %w", orderNo, err)
	}
	if code := resp.GetCode(); code != 0 {
		return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts (order_no=%s): downstream code=%d message=%q: %w",
			orderNo, code, resp.GetMessage(), ErrPaymentFactsNotFound)
	}
	data := resp.GetData()
	if data == nil {
		return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts (order_no=%s): code=0 with nil data: %w",
			orderNo, ErrPaymentFactsNotFound)
	}
	if data.GetIsDuplicate() {
		return invoicemq.PaymentFacts{}, fmt.Errorf(
			"invoice: get payment facts (order_no=%s): downstream returned a duplicate transaction (trade_no=%s)",
			orderNo, data.GetTradeNo())
	}

	payTime, err := timeutil.ParseISO8601(data.GetPayTime())
	if err != nil {
		return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts (order_no=%s): parse pay_time %q: %w",
			orderNo, data.GetPayTime(), err)
	}
	if payTime.IsZero() {
		return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts (order_no=%s): pay_time is zero", orderNo)
	}

	// 🔴 三態：**不能用 GetProviderPaidAt()**。那個 getter 對 nil 回 ""，於是
	// 「payment 端本來就沒有這個值」（ATM webhook 常態）與「有欄位但內容是空字串」
	// （上游 bug）塌成同一件事。前者是設計上的合法值 ⇒ PaidAt() 走 COALESCE 退到
	// pay_time；後者是壞資料 ⇒ 必須 fail-loud，不得靜默退到本地收單時間。
	var providerPaidAt *time.Time
	if data.ProviderPaidAt != nil {
		raw := *data.ProviderPaidAt
		if raw == "" {
			return invoicemq.PaymentFacts{}, fmt.Errorf(
				"invoice: get payment facts (order_no=%s): provider_paid_at is set but empty (upstream must omit the field instead)",
				orderNo)
		}
		t, perr := timeutil.ParseISO8601(raw)
		if perr != nil {
			return invoicemq.PaymentFacts{}, fmt.Errorf("invoice: get payment facts (order_no=%s): parse provider_paid_at %q: %w",
				orderNo, raw, perr)
		}
		utc := t.UTC()
		providerPaidAt = &utc
	}

	return invoicemq.PaymentFacts{
		TradeNo:        data.GetTradeNo(),
		ProviderPaidAt: providerPaidAt,
		PayTime:        payTime.UTC(),
		IsDuplicate:    false,
	}, nil
}
