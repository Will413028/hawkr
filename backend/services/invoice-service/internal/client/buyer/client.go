// Package buyer 是 invoice-service 對 buyer-service 的出站 peer client（58 §6.2）。
//
// 只暴露一支 RPC：`[]buyer_id → 顯示名`（#3017 後半）。它回答的是 A06／A07／S09／S10
// 申訴畫面上「這件申訴是誰提的」那一欄。
//
// 🔴 **鍵是 `buyer_id` 不是 `account_id`**（§6 買賣家身分分離）：`invoice_complaints`
// 存的是 `complainant_buyer_id`（buyers PK、surrogate）。buyer-service 另有一支
// `GetBuyerProfileBatch(account_ids)`，**拿它來用會在 buyer_id ≠ account_id 的買家身上
// 靜默回不到東西**，而首任買家（grandfather，兩者相等）身上完全看不出來。
package buyer

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// DefaultDeadline 是本 client 每一次呼叫的 per-call deadline。
//
// 2 s，同 shipping client：這一跳掛在列表端點的請求路徑上，取的是一個顯示欄位。
//
// 🔴 不得改成 0 或拿掉：`grpc.NewClient` 對沒有 deadline 的呼叫不會自己補一個。
const DefaultDeadline = 2 * time.Second

// MaxBatch 是單次批次的上限，取申訴列表的最大 page_size。
const MaxBatch = 100

// Display 是一位買家的顯示資訊。
//
// `Deleted` 必須帶出來：已註銷 stint 的 `DisplayName` 是 buyer-service 產生的墓碑文案
// （不是個資），遮罩它會得到讀不懂的字串。**判斷「是不是墓碑」讀本欄，不得拿
// DisplayName 去跟墓碑字串做字串比對**——proto 註解明令禁止（那會在每個顯示端
// 硬編碼一份文案）。
type Display struct {
	DisplayName string
	Deleted     bool
}

// Client 包住 BuyerService stub。
type Client struct {
	stub     pb.BuyerServiceClient
	deadline time.Duration
}

// NewClient 撥號 buyer-service。
//
// 兩個 interceptor 缺一不可（§6.2.0 的 52 號 wiring 表）。
// forwardAccountHeaders = false：buyer-service 這支 RPC 用服務身分授權
// （`k8s/values/buyer-service.yaml` 的 `networkPolicy.grpcAllowedCallers` 已列
// invoice-service），不看呼叫端的使用者 header。
func NewClient(lc fx.Lifecycle, addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoice: buyer peer addr is empty (BUYER_SERVICE_ADDR)")
	}
	conn, err := grpcclient.Dial(addr,
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("invoice-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("invoice: dial buyer-service %s: %w", addr, err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return conn.Close() }})
	return NewClientWithStub(pb.NewBuyerServiceClient(conn), DefaultDeadline), nil
}

// NewClientWithStub 讓測試注入 stub（bufconn）與較短的 deadline。
func NewClientWithStub(stub pb.BuyerServiceClient, deadline time.Duration) *Client {
	if deadline <= 0 {
		deadline = DefaultDeadline
	}
	return &Client{stub: stub, deadline: deadline}
}

// BatchGetBuyerDisplay 取一批買家的顯示名。
//
// 回傳 map 只含查得到的 buyer_id；查不到的不進 map（proto 逐字：「查無的 id 不出現」）。
//
// 🔴 錯誤一律往上拋，不得在這一層降級成空 map：降不降級是呼叫端的決定。
func (c *Client) BatchGetBuyerDisplay(ctx context.Context, buyerIDs []int64) (map[int64]Display, error) {
	if len(buyerIDs) == 0 {
		return map[int64]Display{}, nil
	}
	if len(buyerIDs) > MaxBatch {
		return nil, fmt.Errorf("invoice: batch get buyer display: %d buyer_ids exceeds max %d", len(buyerIDs), MaxBatch)
	}

	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.BatchGetBuyerDisplayByBuyerIDs(ctx,
		&pb.BatchGetBuyerDisplayByBuyerIDsRequest{BuyerIds: buyerIDs})
	if err != nil {
		return nil, fmt.Errorf("invoice: batch get buyer display (%d ids): %w", len(buyerIDs), err)
	}
	if code := resp.GetCode(); code != 0 {
		return nil, fmt.Errorf("invoice: batch get buyer display: downstream code=%d message=%q", code, resp.GetMessage())
	}

	out := make(map[int64]Display, len(resp.GetBuyers()))
	for id, v := range resp.GetBuyers() {
		// ⚠️ **`v == nil` 在 wire 上不會發生**：protobuf 的 map<int64, Message> 送出
		// nil 值之後，接收端解出來是一個**空 message** 而不是 nil（實測，
		// client_test.go 的 id 13 那一格就是為此存在）。所以判準必須是「有沒有名字」，
		// 不能是「指標是不是 nil」——後者在單元測試的 in-process 假物件上看起來有效，
		// 一上 wire 就永遠不成立。
		name := v.GetDisplayName()
		if name == "" {
			// 沒有名字的項目不是答案。proto 逐字「查無的 id 不出現」⇒ 出現卻沒名字
			// 屬於異常，讓它留在 map 裡會讓「解析成功但沒名字」與「解析成功」
			// 在呼叫端無從區分。
			continue
		}
		out[id] = Display{DisplayName: name, Deleted: v.GetDeleted()}
	}
	return out, nil
}
