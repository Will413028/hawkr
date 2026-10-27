package buyer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.BuyerServiceClient
}

func NewClient(cfg *config.Config) (usecase.BuyerClient, error) {
	conn, err := grpcclient.Dial(cfg.Buyer.Addr(),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("buyer grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewBuyerServiceClient(conn)}, nil
}

func (c *Client) GetBuyerProfile(ctx context.Context, buyerID int64) (*usecase.AccountInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetBuyerProfileByBuyerID(ctx, &pb.GetBuyerProfileByBuyerIDRequest{BuyerId: buyerID})
	if err != nil || resp.GetCode() != 0 || resp.GetData() == nil {
		pkglogger.Debug(ctx, "buyer GetBuyerProfileByBuyerID failed", zap.Int64("buyerID", buyerID), zap.Error(err), zap.Int32("code", resp.GetCode()))
		// nil = 查不到（見 usecase.AccountInfo 的契約說明）。理由同 seller client。
		return nil, nil
	}
	data := resp.GetData()
	return &usecase.AccountInfo{
		Nickname: data.GetNickname(),
		Avatar:   data.GetAvatarKey(),
	}, nil
}

func (c *Client) GetSubscriberIDs(ctx context.Context, sellerID int64) ([]int64, error) {
	var ids []int64
	page := int32(1)
	const requestedPageSize = int32(100)

	for {
		ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout10s)
		resp, err := c.client.GetSubscribersList(ctx, &pb.GetSubscribersListRequest{
			StoreId:  sellerID,
			Page:     page,
			PageSize: requestedPageSize,
		})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("GetSubscribersList page %d: %w", page, err)
		}
		for _, s := range resp.Data {
			ids = append(ids, s.BuyerId)
		}
		// buyer-service clamps PageSize server-side (currently to a lower max
		// than requestedPageSize=100 here) — this loop must compare against
		// what the server actually used (resp.PageSize), not the value it
		// requested, or a seller with more subscribers than the server's max
		// silently loses every subscriber past the first page (found
		// 2026-08-15: requestedPageSize=100 vs. server max=50 meant
		// len(resp.Data)==50 < 100 was always true, so this always stopped
		// after page 1 regardless of how many more subscribers existed).
		actualPageSize, perr := strconv.ParseInt(resp.PageSize, 10, 32)
		if perr != nil || actualPageSize <= 0 || int64(len(resp.Data)) < actualPageSize {
			break
		}
		page++
	}
	return ids, nil
}

// GetBuyerMetaBatch 查 buyer_id（PK）——理由同 GetBuyerProfile。
func (c *Client) GetBuyerMetaBatch(ctx context.Context, buyerIDs []int64) (map[int64]*usecase.AccountInfo, error) {
	if len(buyerIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.BatchGetBuyerMetaByBuyerIDs(ctx, &pb.BatchGetBuyerMetaByBuyerIDsRequest{BuyerIds: buyerIDs})
	if err != nil || resp.GetCode() != 0 {
		return make(map[int64]*usecase.AccountInfo), nil
	}
	result := make(map[int64]*usecase.AccountInfo, len(resp.GetBuyers()))
	for id, meta := range resp.GetBuyers() {
		result[id] = &usecase.AccountInfo{
			Nickname: meta.GetNickname(),
			Avatar:   meta.GetAvatarKey(), // 49 §6.6：buyer 回 objectKey；批 4 組 URL
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

// ResolvePrincipalStrict, unlike ResolvePrincipal above, must genuinely
// distinguish "this id is a known buyer_id" from "buyer-service doesn't
// recognize it" — see dispatcher.go's BanUser doc comment for why a false
// positive here (treating an echoed-back id as Found) matters: it would skip
// trying the id against SellerClient too. Plain `map[buyerID]` presence
// can't do this: buyer-service's ResolvePrincipals always returns a fully
// populated map (unknown ids echo back unchanged, never omitted) — the same
// idiom this campaign's aftersales-service fix proved is a no-op (see
// ReturnStuckAlertScheduler.resolveNotifyBuyerAccountId). This uses the real
// signal buyer.proto's ResolvePrincipalsResponse.unresolved_ids now carries
// instead (2026-08-05, added by the same campaign's buyer-service contract
// fix): Found is true only when buyerID is NOT in that list.
func (c *Client) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (usecase.PrincipalResolution, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &pb.ResolvePrincipalsRequest{Ids: []int64{buyerID}})
	if err != nil {
		return usecase.PrincipalResolution{}, fmt.Errorf("buyer ResolvePrincipals: %w", err)
	}
	if resp.GetCode() != 0 {
		return usecase.PrincipalResolution{}, fmt.Errorf("buyer ResolvePrincipals: code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}
	for _, unresolved := range resp.GetUnresolvedIds() {
		if unresolved == buyerID {
			return usecase.PrincipalResolution{}, nil
		}
	}
	if principal, ok := resp.GetPrincipals()[buyerID]; ok {
		return usecase.PrincipalResolution{AccountID: principal, Found: true}, nil
	}
	return usecase.PrincipalResolution{}, nil
}
