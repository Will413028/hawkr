package seller

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Client wraps the seller-service gRPC client.
type Client struct {
	client pb.SellerServiceClient
}

// NewClient creates a new seller-service gRPC client.
func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.SellerServiceAddr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to seller-service at %s: %w", cfg.SellerServiceAddr, err)
	}
	return &Client{client: pb.NewSellerServiceClient(conn)}, nil
}

// ProfileInfo holds the seller's display info.
type ProfileInfo struct {
	Nickname  string
	Avatar    string
	StoreName string
	LogoKey   string
	// StoreCode 是賣家的 canonical 路徑短碼（sellers.store_code）。對話列表的
	// target_user_id 在賣家側是 store_id（內部代理鍵），賣家頁 canonical 網址
	// 走 store_code；值與 StoreName/LogoKey 同在這一次 GetSellerBatch 回應裡。
	StoreCode   string
	StoreClosed bool
}

func (c *Client) GetProfileBatch(ctx context.Context, storeIDs []int64) (map[int64]*ProfileInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetSellerBatch(ctx, &pb.GetSellerBatchRequest{StoreIds: storeIDs})
	if err != nil {
		return nil, fmt.Errorf("get seller profiles batch: %w", err)
	}
	// 業務碼要單獨檢查：seller-service 對業務錯誤回 (Code != 0, 空 data, error = nil)
	// —— 少了它，關店守門會把「查不到店家狀態」讀成「這家店沒關」而靜默放行，
	// 那正是 #2830 群組 5 要消滅的「查詢失敗被當成業務結論」。
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("get seller profiles batch: code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}

	result := make(map[int64]*ProfileInfo, len(resp.Data))
	for _, info := range resp.Data {
		nickname := info.Nickname
		if nickname == "" {
			nickname = info.Name
		}
		// 49 §6.6：seller 回 objectKey（avatar_key/logo_key）；批 4 於 im Gateway/前端組 URL
		avatar := info.AvatarKey
		if avatar == "" {
			avatar = info.LogoKey
		}
		storeID, parseErr := strconv.ParseInt(info.GetStoreId(), 10, 64)
		if parseErr != nil || storeID <= 0 {
			return nil, fmt.Errorf("seller-service returned invalid store_id %q", info.GetStoreId())
		}
		result[storeID] = &ProfileInfo{
			Nickname:  nickname,
			Avatar:    avatar,
			StoreName: info.Name,
			LogoKey:   info.LogoKey, // 49 §6.6：裸 objectKey，顯示 URL 由消費端組
			StoreCode: info.StoreCode,
			StoreClosed: info.LifecycleStatus == "closed" ||
				info.LifecycleStatus == "deleted", // reshape 前殘值
		}
	}
	return result, nil
}

func (c *Client) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &pb.ResolvePrincipalsRequest{Ids: []int64{storeID}})
	if err != nil {
		pkglogger.Warn(ctx, "seller ResolvePrincipals RPC failed; falling back to the raw store_id",
			zap.Int64("store_id", storeID), zap.Error(err))
		return storeID
	}
	if resp.GetCode() != 0 {
		pkglogger.Warn(ctx, "seller ResolvePrincipals returned a business error; falling back to the raw store_id",
			zap.Int64("store_id", storeID), zap.Int32("code", resp.GetCode()), zap.String("message", resp.GetMessage()))
		return storeID
	}
	for _, unresolved := range resp.GetUnresolvedIds() {
		if unresolved == storeID {
			// seller-service has no live sellers row for this store_id (soft-deleted
			// stint, or never existed). principals[storeID] carries an UNVERIFIED
			// echo-back of the input, not a real resolution — see
			// seller.proto ResolvePrincipalsResponse.unresolved_ids.
			//
			// The return value is deliberately unchanged (the echo-back and the raw
			// store_id are the same number): this branch used to be completely
			// silent, and making it observable is the whole point. Deciding what a
			// caller should *do* about it is a separate, per-call-site question —
			// see issue #2789.
			pkglogger.Warn(ctx, "seller-service does not recognise this store_id (soft-deleted or unknown); principal is an unverified echo-back, not a real resolution",
				zap.Int64("store_id", storeID))
			return storeID
		}
	}
	if principal, ok := resp.GetPrincipals()[storeID]; ok {
		return principal
	}
	pkglogger.Warn(ctx, "seller ResolvePrincipals omitted the requested store_id from its principals map; falling back to the raw store_id",
		zap.Int64("store_id", storeID))
	return storeID
}

// ResolvePrincipalStrict answers the same question as ResolvePrincipal above,
// but without flattening "seller-service says this is not a live store_id" into
// the echo-back that ResolvePrincipal has to return for read paths.
//
// found=false means seller-service has no live sellers row for storeID (a
// soft-deleted stint, or an id that was never a store_id at all). An error
// means seller-service could not answer at all — transport failure or a
// non-zero business code. The two are deliberately distinct: a WRITE or GATE
// path must be able to tell "not mine" from "I don't know", and both from a
// real resolution (#2789).
//
// Callers that compare two principals for equality MUST use this method:
// with ResolvePrincipal, an unresolved id comes back as itself, so two ids
// belonging to the same human compare as different and the guard silently
// fails open.
func (c *Client) ResolvePrincipalStrict(ctx context.Context, storeID int64) (accountID int64, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &pb.ResolvePrincipalsRequest{Ids: []int64{storeID}})
	if err != nil {
		return 0, false, fmt.Errorf("seller ResolvePrincipals store_id %d: %w", storeID, err)
	}
	if resp.GetCode() != 0 {
		return 0, false, fmt.Errorf("seller ResolvePrincipals store_id %d: code=%d msg=%s", storeID, resp.GetCode(), resp.GetMessage())
	}
	for _, unresolved := range resp.GetUnresolvedIds() {
		if unresolved == storeID {
			return 0, false, nil
		}
	}
	if principal, ok := resp.GetPrincipals()[storeID]; ok {
		return principal, true, nil
	}
	// Not reachable against the real server (its unnest LEFT JOIN always fills
	// the map), but an omitted id is not a resolution either — report it as
	// "not mine" rather than inventing one.
	return 0, false, nil
}

func (c *Client) CurrentStoreID(ctx context.Context, accountID int64) int64 {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetSeller(ctx, &pb.GetSellerRequest{AccountId: accountID})
	if err != nil || resp.GetCode() != 0 {
		return accountID
	}
	if sid, perr := strconv.ParseInt(resp.GetData().GetStoreId(), 10, 64); perr == nil && sid > 0 {
		return sid
	}
	return accountID
}

// ListIDsByAccount returns every store_id ever assigned to accountID (including
// closed stints) via seller-service's internal-only ListStoreIDsByAccount RPC —
// used solely by the account.deleted/buyer.deleted PII-purge consumer (D8).
// Unlike buyer-service's equivalent, seller.proto's StoreIds field is a native
// repeated int64 (matching this proto's existing ResolvePrincipalsResponse
// convention) — no string parsing needed.
func (c *Client) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ListStoreIDsByAccount(ctx, &pb.ListStoreIDsByAccountRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("list store ids by account: %w", err)
	}
	return resp.GetStoreIds(), nil
}
