package buyer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"google.golang.org/grpc"
)

// Client wraps the buyer-service gRPC client.
type Client struct {
	client pb.BuyerServiceClient
}

// NewClient creates a new buyer-service gRPC client.
func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.BuyerServiceAddr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to buyer-service at %s: %w", cfg.BuyerServiceAddr, err)
	}
	return &Client{client: pb.NewBuyerServiceClient(conn)}, nil
}

// ProfileInfo holds the buyer's display info.
type ProfileInfo struct {
	Nickname string
	Avatar   string
}

func (c *Client) GetProfileBatch(ctx context.Context, buyerIDs []int64) (map[int64]*ProfileInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	// #2830：用 Display 而非 Meta。Meta 底層帶 `deleted_at IS NULL`（刻意的 PII
	// 邊界），買家註銷後解不出來 ⇒ 賣家的對話列表作者名整片空白，而
	// RedactDeletedAccount 刻意**保留對話列**（只塗銷訊息內容），所以這個 surface
	// 結構上必然含已註銷買家。Display 讓它們解得出來但只回墓碑，真名不外流。
	resp, err := c.client.BatchGetBuyerDisplayByBuyerIDs(ctx, &pb.BatchGetBuyerDisplayByBuyerIDsRequest{BuyerIds: buyerIDs})
	if err != nil {
		return nil, fmt.Errorf("get buyer display batch: %w", err)
	}
	// 業務碼要單獨檢查：buyer-service 對業務錯誤回 (Code != 0, 空 map, error = nil)
	// —— 只看 err 會把「查詢失敗」讀成「這批人都查無此人」，正是本 RPC 要消滅的
	// 混淆。同檔 ResolvePrincipals 一直都有這道檢查，這支缺了。
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("get buyer display batch: code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}

	result := make(map[int64]*ProfileInfo, len(resp.Buyers))
	for buyerID, meta := range resp.Buyers {
		result[buyerID] = &ProfileInfo{
			Nickname: meta.DisplayName, // 已註銷者已是墓碑字串，可直接顯示
			Avatar:   meta.AvatarKey,   // 已註銷者恆為空
		}
	}
	return result, nil
}

func (c *Client) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &pb.ResolvePrincipalsRequest{Ids: []int64{buyerID}})
	if err != nil || resp.GetCode() != 0 {
		return buyerID
	}
	if principal, ok := resp.GetPrincipals()[buyerID]; ok {
		return principal
	}
	return buyerID
}

// ResolvePrincipalStrict answers the same question as ResolvePrincipal above,
// but without flattening "buyer-service says this is not a live buyer_id" into
// the echo-back that ResolvePrincipal has to return for read paths.
//
// found=false means buyer-service has no live buyers row for buyerID (a
// soft-deleted stint, or an id that was never a buyer_id at all). An error
// means buyer-service could not answer at all — transport failure or a
// non-zero business code. The two are deliberately distinct: a WRITE or GATE
// path must be able to tell "not mine" from "I don't know", and both from a
// real resolution (#2789).
//
// Callers that compare two principals for equality MUST use this method:
// with ResolvePrincipal, an unresolved id comes back as itself, so two ids
// belonging to the same human compare as different and the guard silently
// fails open.
func (c *Client) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (accountID int64, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &pb.ResolvePrincipalsRequest{Ids: []int64{buyerID}})
	if err != nil {
		return 0, false, fmt.Errorf("buyer ResolvePrincipals buyer_id %d: %w", buyerID, err)
	}
	if resp.GetCode() != 0 {
		return 0, false, fmt.Errorf("buyer ResolvePrincipals buyer_id %d: code=%d msg=%s", buyerID, resp.GetCode(), resp.GetMessage())
	}
	for _, unresolved := range resp.GetUnresolvedIds() {
		if unresolved == buyerID {
			return 0, false, nil
		}
	}
	if principal, ok := resp.GetPrincipals()[buyerID]; ok {
		return principal, true, nil
	}
	// Not reachable against the real server (its unnest LEFT JOIN always fills
	// the map), but an omitted id is not a resolution either — report it as
	// "not mine" rather than inventing one.
	return 0, false, nil
}

// ListIDsByAccount returns every buyer_id ever assigned to accountID (including
// closed stints) via buyer-service's internal-only ListBuyerIDsByAccount RPC —
// used solely by the account.deleted/buyer.deleted PII-purge consumer (D8).
func (c *Client) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ListBuyerIDsByAccount(ctx, &pb.ListBuyerIDsByAccountRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("list buyer ids by account: %w", err)
	}
	ids := make([]int64, 0, len(resp.GetBuyerIds()))
	for _, s := range resp.GetBuyerIds() {
		id, perr := strconv.ParseInt(s, 10, 64)
		if perr != nil {
			continue // defensive: buyer-service's TSID-as-string field should always parse
		}
		ids = append(ids, id)
	}
	return ids, nil
}
