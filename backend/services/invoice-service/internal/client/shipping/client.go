// Package shipping 是 invoice-service 對 shipping-service 的出站 peer client（58 §6.2）。
//
// 只暴露一支 RPC：`[]order_no → order_no ⇒ 物流狀態碼`（#3019）。它回答的是
// S06／A12 列表上「這筆訂單出貨了沒」那一欄。
//
// 🔴 **不做任何狀態合成**：58 §6.1.10 明令 `shipmentStatus` 必須與發票狀態分開顯示，
// 合成之後前端就無法履行「出貨不受影響」這句話。本 client 原樣回傳
// shipping-service 的內部狀態碼（ShippingStatus CodeMeta 的 value，數字字串）。
package shipping

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// DefaultDeadline 是本 client 每一次呼叫的 per-call deadline。
//
// 2 s 而不是 payment client 的 3 s：這一跳掛在**列表端點的請求路徑上**，而它取的是
// 一個顯示欄位。整頁的 p99 預算是 300 ms（58 §9.4.3），賣家寧可看到「暫時取不到」
// 也不要為了一欄出貨狀態多等三秒。
//
// 🔴 不得改成 0 或拿掉：`grpc.NewClient` 對沒有 deadline 的呼叫不會自己補一個，
// shipping-service 卡住時整個 S06／A12 會跟著掛住。
const DefaultDeadline = 2 * time.Second

// MaxBatch 是單次批次的上限。列表最大 pageSize 是 100（58 §6.1.10），
// 這裡取同一個數字當硬上限：超過就截斷並讓呼叫端把整批標成降級，
// 而不是靜默只回前 100 筆——後者會讓「查不到」與「沒出貨」再次塌成同一件事。
const MaxBatch = 100

// Client 包住 ShippingService stub。
type Client struct {
	stub     pb.ShippingServiceClient
	deadline time.Duration
}

// NewClient 撥號 shipping-service。
//
// 兩個 interceptor 缺一不可（§6.2.0 的 52 號 wiring 表）：
//   - InternalAuthClientInterceptor 帶 HMAC 四 header。缺它 ⇒ 對端在
//     `INTERNAL_AUTH_MODE=enforcing` 下每次都回 UNAUTHENTICATED，而 permissive
//     期間完全看不出來。
//   - OTelClientStatsHandler 讓這一跳留在 trace 上。
//
// forwardAccountHeaders = false：shipping-service 這支 RPC 用服務身分授權
// （NetworkPolicy 的 shipping-service-ingress 已列 invoice-service），
// 不看呼叫端的使用者 header。
func NewClient(lc fx.Lifecycle, addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoice: shipping peer addr is empty (SHIPPING_SERVICE_ADDR)")
	}
	conn, err := grpcclient.Dial(addr,
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("invoice-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("invoice: dial shipping-service %s: %w", addr, err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return conn.Close() }})
	return NewClientWithStub(pb.NewShippingServiceClient(conn), DefaultDeadline), nil
}

// NewClientWithStub 讓測試注入 stub（bufconn）與較短的 deadline。
//
// deadline <= 0 時退回 DefaultDeadline，不是「不設 deadline」。
func NewClientWithStub(stub pb.ShippingServiceClient, deadline time.Duration) *Client {
	if deadline <= 0 {
		deadline = DefaultDeadline
	}
	return &Client{stub: stub, deadline: deadline}
}

// BatchGetShipmentStatus 取一批訂單的物流狀態碼。
//
// 回傳 map 只含**查得到物流單**的訂單；查不到的訂單不進 map，呼叫端據此回空字串
// ——那是誠實的「這張訂單還沒有物流單」。
//
// 🔴 錯誤一律往上拋，**不得在這一層降級成空 map**：降不降級是呼叫端的決定，
// 而它必須分得出「整批查不到」與「這幾筆沒有物流單」。這一層把 error 吃掉，
// 上層就再也沒有東西可以據以設 shipment_status_degraded（#3019 的原始症狀
// 就是空值被讀成「還沒出貨」）。
func (c *Client) BatchGetShipmentStatus(ctx context.Context, orderNos []string) (map[string]string, error) {
	if len(orderNos) == 0 {
		return map[string]string{}, nil
	}
	if len(orderNos) > MaxBatch {
		return nil, fmt.Errorf("invoice: batch get shipment status: %d order_nos exceeds max %d", len(orderNos), MaxBatch)
	}

	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.GetShipmentsByOrderNos(ctx, &pb.GetShipmentsByOrderNosRequest{OrderNos: orderNos})
	if err != nil {
		return nil, fmt.Errorf("invoice: batch get shipment status (%d order_nos): %w", len(orderNos), err)
	}
	if code := resp.GetCode(); code != 0 {
		return nil, fmt.Errorf("invoice: batch get shipment status: downstream code=%d message=%q", code, resp.GetMessage())
	}

	out := make(map[string]string, len(resp.GetData()))
	for _, v := range resp.GetData() {
		orderNo := v.GetOrderNo()
		if orderNo == "" {
			continue
		}
		// 一張訂單可能有多筆物流單（退貨便會再開一張）。**取最後一筆**與
		// shipping-service 的回傳順序綁定過緊，所以這裡的規則寫死：
		// 已存在就不覆寫 ⇒ 取第一筆，與 GetShipmentByOrderNo 的單筆語意一致。
		if _, dup := out[orderNo]; dup {
			continue
		}
		out[orderNo] = v.GetStatus()
	}
	return out, nil
}

// HolidaysDeadline 是行事曆同步的 per-call deadline。
//
// 8 s 而不是 DefaultDeadline 的 2 s：這一跳**不在請求路徑上**（呼叫端是 INV.gov
// 每日排程），而它一次要回 430 天區間的假日（約 120 列）。用 2 s 會讓一支
// 每天只跑一次、失敗代價是「行事曆停在昨天」的工作被一個為列表端點訂的預算擋掉。
const HolidaysDeadline = 8 * time.Second

// GetHolidays 取一段閉區間內的國定假日（ISO 8601 日期，升冪）。
//
// 呼叫端是 invoice-service 的行事曆鏡像同步（58 §5.1.14，每日整段覆蓋）。
//
// 🔴 **空清單與錯誤必須分得開**，所以錯誤一律往上拋、絕不降級成空 slice：
// 鏡像是「整段 DELETE + INSERT」，把一次失敗當成「這段區間沒有假日」會直接
// 把整張表清空，而空表在 BusinessCalendar 那端是**零涵蓋**⇒ 每一筆申訴的
// SLA 都退回 14 天硬上限。一次網路抖動換來全站 SLA 失準。
func (c *Client) GetHolidays(ctx context.Context, from, to time.Time) ([]time.Time, error) {
	const layout = "2006-01-02"
	ctx, cancel := context.WithTimeout(ctx, HolidaysDeadline)
	defer cancel()

	resp, err := c.stub.GetHolidaysInternal(ctx, &pb.GetHolidaysInternalRequest{
		FromDate: from.Format(layout),
		ToDate:   to.Format(layout),
	})
	if err != nil {
		return nil, fmt.Errorf("invoice: get holidays [%s, %s]: %w", from.Format(layout), to.Format(layout), err)
	}
	if code := resp.GetCode(); code != 0 {
		return nil, fmt.Errorf("invoice: get holidays: downstream code=%d message=%q", code, resp.GetMessage())
	}

	out := make([]time.Time, 0, len(resp.GetHolidayDates()))
	for _, raw := range resp.GetHolidayDates() {
		// 解析失敗**整批失敗**，不是跳過那一列：少一天假日會讓 SLA 算偏早，
		// 而偏早的方向是「賣家被誤判逾期 ⇒ 違規計數 +1」（domain/bizday.go 紅字）。
		d, perr := time.Parse(layout, raw)
		if perr != nil {
			return nil, fmt.Errorf("invoice: get holidays: bad date %q: %w", raw, perr)
		}
		out = append(out, d)
	}
	return out, nil
}
