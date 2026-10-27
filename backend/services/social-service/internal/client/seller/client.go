package seller

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.SellerServiceClient
}

func NewClient(cfg *config.Config) (usecase.SellerClient, error) {
	conn, err := grpcclient.Dial(cfg.Seller.Addr(),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("seller grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewSellerServiceClient(conn)}, nil
}

func (c *Client) GetSellerInfo(ctx context.Context, storeID int64) (*usecase.AccountInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetSeller(ctx, &pb.GetSellerRequest{StoreId: storeID})
	if err != nil || resp.GetCode() != 0 || resp.GetData() == nil {
		pkglogger.Debug(ctx, "seller GetSeller failed", zap.Int64("storeID", storeID), zap.Error(err), zap.Int32("code", resp.GetCode()))
		// nil = 查不到（見 usecase.AccountInfo 的契約說明）。不可回 &AccountInfo{}：
		// 那會讓「以存在與否判斷身分」的 caller 永遠得到肯定答案。
		return nil, nil
	}
	data := resp.GetData()
	return &usecase.AccountInfo{
		Nickname: data.GetName(),
		Avatar:   data.GetLogoKey(), // 49 §6.6：seller 回 objectKey；批 4 組 URL
	}, nil
}

func (c *Client) GetSellerMetaBatch(ctx context.Context, storeIDs []int64) (map[int64]*usecase.AccountInfo, error) {
	if len(storeIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.BatchGetSellerMeta(ctx, &pb.BatchGetSellerMetaRequest{StoreIds: storeIDs})
	if err != nil || resp.GetCode() != 0 {
		return make(map[int64]*usecase.AccountInfo), nil
	}
	result := make(map[int64]*usecase.AccountInfo, len(resp.GetSellers()))
	for id, meta := range resp.GetSellers() {
		result[id] = &usecase.AccountInfo{
			Nickname: meta.GetSellerName(),
			Avatar:   meta.GetLogoKey(), // 49 §6.6：seller 回 objectKey；批 4 組 URL
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

// ResolvePrincipalStrict, unlike ResolvePrincipal above, must genuinely
// distinguish "this id is a known store_id" from "seller-service doesn't
// recognize it" — see dispatcher.go's BanUser for why a false positive here
// (treating an echoed-back id as Found) matters: it would skip trying the id
// against BuyerClient too. Plain `map[storeID]` presence cannot do this:
// seller-service's ResolvePrincipals returns a fully populated map (unknown
// and soft-deleted ids echo back unchanged, never omitted), so Found was
// unconditionally true — the same idiom the aftersales-service fix proved is
// a no-op (see ReturnStuckAlertScheduler.resolveNotifyBuyerAccountId).
//
// This now uses the real signal seller.proto's ResolvePrincipalsResponse
// .unresolved_ids carries (issue #2789), mirroring the buyer-side sibling
// client/buyer/client.go (fixed 2026-08-05): Found is true only when storeID
// is NOT in that list.
func (c *Client) ResolvePrincipalStrict(ctx context.Context, storeID int64) (usecase.PrincipalResolution, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &pb.ResolvePrincipalsRequest{Ids: []int64{storeID}})
	if err != nil {
		return usecase.PrincipalResolution{}, fmt.Errorf("seller ResolvePrincipals: %w", err)
	}
	if resp.GetCode() != 0 {
		return usecase.PrincipalResolution{}, fmt.Errorf("seller ResolvePrincipals: code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}
	for _, unresolved := range resp.GetUnresolvedIds() {
		if unresolved == storeID {
			return usecase.PrincipalResolution{}, nil
		}
	}
	if principal, ok := resp.GetPrincipals()[storeID]; ok {
		return usecase.PrincipalResolution{AccountID: principal, Found: true}, nil
	}
	return usecase.PrincipalResolution{}, nil
}
