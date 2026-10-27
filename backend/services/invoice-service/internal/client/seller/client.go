// Package seller 是 invoice-service 對 seller-service 的出站 peer client（58 §6.2）。
//
// 只暴露一支 RPC：`store_id → account_id`。多一支就是「賣家是誰」這個問題的第二個
// 答案來源，而 §6 的身分分離規則只允許一個。
//
// 🔴 身分語意（§6）：`store_id` 是賣家的**業務鍵**（sellers PK surrogate），
// `account_id` 是**登入 principal**（accounts PK）。兩者首任 grandfather 相等但
// **禁止假設恆等**——本 client 存在的唯一理由就是那個假設不成立。
package seller

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrSellerNotFound 表示 seller-service 查無此 store_id（或該列已軟刪）。
//
// 兩者對「這個 store 的登入 principal 是誰」是同一個答案：沒有。呼叫端
// （`INV.ob` 的 withNotifyRouting）看到任何 error 都會填 `"0"` 並繼續派送，
// 這個具名 error 的用途是讓「查無此人」與「seller-service 掛了」在 log 上分得開。
var ErrSellerNotFound = stderrors.New("invoice: seller not found for store_id")

// DefaultDeadline 是本 client 每一次呼叫的 per-call deadline。
//
// 3 s ＝ 58 §6.2.0「一般讀寫（不打 AMEGO）」那一格的同一個數字：這是一次
// 純索引查詢（sellers PK → account_id），p99 預算遠小於它。
//
// ⚠️ 它是常數而不是 env：`internal/config` 的 PeerConfig 目前只有位址、沒有
// per-peer deadline 欄位，而新增設定欄位不在本次可寫範圍內。要調它就是加一個
// `INVOICE_GRPC_DEADLINE_PEER_MS` 進 config.PeerConfig，然後由 NewClient 帶進來。
//
// 🔴 不得改成 0 或拿掉：`grpc.NewClient` 對沒有 deadline 的呼叫不會自己補一個，
// seller-service 卡住時這一跳會無限期掛住 outbox 派送 goroutine，而 outbox 的
// tick 是 5 s ⇒ 一次卡死等於整批出向事件停擺，且沒有任何錯誤訊號。
const DefaultDeadline = 3 * time.Second

// Client 包住 SellerService stub。
type Client struct {
	stub     pb.SellerServiceClient
	deadline time.Duration
}

// NewClient 撥號 seller-service。
//
// 兩個 interceptor 缺一不可（§5.2）：
//   - InternalAuthClientInterceptor 帶 52 號 HMAC 四 header。缺它 ⇒ 對端在
//     `INTERNAL_AUTH_MODE=enforcing` 下每次都回 UNAUTHENTICATED，而 permissive
//     期間完全看不出來。
//   - OTelClientStatsHandler 讓這一跳留在 trace 上。
//
// forwardAccountHeaders = false：這是 outbox 派送時的系統呼叫，incoming context
// 沒有使用者身分（58 §6.2.0 的出站表逐條列明）。
//
// 連線註冊進 fx lifecycle：在 usecase 裡自己撥號的連線沒有 owner，關不掉。
func NewClient(lc fx.Lifecycle, addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoice: seller peer addr is empty (SELLER_SERVICE_ADDR)")
	}
	conn, err := grpcclient.Dial(addr,
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("invoice-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("invoice: dial seller-service %s: %w", addr, err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return conn.Close() }})
	return NewClientWithStub(pb.NewSellerServiceClient(conn), DefaultDeadline), nil
}

// NewClientWithStub 讓測試注入 stub（bufconn）與較短的 deadline。
//
// deadline <= 0 時退回 DefaultDeadline，不是「不設 deadline」——後者是本檔開頭
// 註解說明的那個失效模式。
func NewClientWithStub(stub pb.SellerServiceClient, deadline time.Duration) *Client {
	if deadline <= 0 {
		deadline = DefaultDeadline
	}
	return &Client{stub: stub, deadline: deadline}
}

// ResolveSellerAccountID 把賣家業務鍵解析成登入 principal。
//
// 🔴 三條 fail-loud，任何一條退化成「回 0 ＋ nil error」都會讓呼叫端把
// `notify_seller_account_id` 寫成一個**看起來解析成功的 0**，而下游 notification
// 收到 0 就 skip ⇒ 賣家通知靜默不送，log 上卻沒有任何 Warn：
//
//	① 下游回 gRPC error ⇒ 原樣往上拋（NotFound 換成 ErrSellerNotFound）
//	② storeID <= 0 ⇒ 連打都不打，直接 error
//	③ 下游回 account_id <= 0 ⇒ error，不得當成合法解析結果
func (c *Client) ResolveSellerAccountID(ctx context.Context, storeID int64) (int64, error) {
	if storeID <= 0 {
		return 0, fmt.Errorf("invoice: resolve seller account: invalid store_id %d", storeID)
	}

	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	// StoreId 是 int64 而非 string：§4.1 例外① —— 它是服務內部注入的身分欄位、
	// 前端不可控（Request 端；Response 同語意欄位仍必須是 string）。
	resp, err := c.stub.ResolveSellerAccountID(ctx, &pb.ResolveSellerAccountIDRequest{StoreId: storeID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, fmt.Errorf("%w (store_id=%d)", ErrSellerNotFound, storeID)
		}
		return 0, fmt.Errorf("invoice: resolve seller account (store_id=%d): %w", storeID, err)
	}

	// 本 RPC 的 Response 沒有 code/message envelope（錯誤走真實 gRPC status，
	// seller.proto 的註解逐字寫明）⇒ 這裡不得去讀 GetCode()。
	accountID := resp.GetAccountId()
	if accountID <= 0 {
		return 0, fmt.Errorf("invoice: seller-service returned non-positive account_id %d for store_id %d",
			accountID, storeID)
	}
	return accountID, nil
}

// BatchGetSellerMeta 批次取店名（58 §6.1.12／§6.1.13 的 ADM 工作台識別欄）。
//
// 🔴 **失敗不是錯誤**：店名是 ADM 工作台上的「識別對象」欄，取不到就退回 store_id
// ——那正是修這件事之前的畫面。把它做成 fail-loud 會讓 seller-service 一次抖動
// 就讓整個稅籍審核佇列打不開，而佇列本身的資料完全健康。
// ⇒ 回 (nil, err) 由呼叫端自行決定退回空字串，呼叫端**不得**因此中止整個列表。
//
// 回傳 map 的鍵是 store_id（store-stint surrogate），與傳入的 storeIDs 同一個軸。
func (c *Client) BatchGetSellerMeta(ctx context.Context, storeIDs []int64) (map[int64]string, error) {
	if len(storeIDs) == 0 {
		return map[int64]string{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.BatchGetSellerMeta(ctx, &pb.BatchGetSellerMetaRequest{StoreIds: storeIDs})
	if err != nil {
		return nil, fmt.Errorf("invoice: batch get seller meta: %w", err)
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("invoice: batch get seller meta code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}
	out := make(map[int64]string, len(resp.GetSellers()))
	for storeID, meta := range resp.GetSellers() {
		// 空店名照樣放進 map：那是 seller-service 的事實，不是「查不到」。
		// 兩者對呼叫端的處置相同（顯示 store_id），但混在一起會讓下次查不到時
		// 以為是店名真的沒設。
		out[storeID] = meta.GetSellerName()
	}
	return out, nil
}
