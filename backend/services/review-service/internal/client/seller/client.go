package seller

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Client struct {
	client sellerpb.SellerServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.SellerService.Addr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to seller-service at %s: %w", cfg.SellerService.Addr, err)
	}
	return &Client{client: sellerpb.NewSellerServiceClient(conn)}, nil
}

// ResolvePrincipal maps a store_id to the seller's login account_id, and
// returns 0 — NOT the input — whenever it could not do so.
//
// ⚠️ The semantics here are the OPPOSITE of what this method did before
// #2789 stage B2a, and the opposite of the identically named method in
// im-service / social-service / buyer-service. That is deliberate, not drift:
// this client has exactly one call site (usecase/review_usecase.go, the
// notifySellerAccountID argument of PublishReviewCreated), the value's only
// destination is review.created's notify_seller_account_id, and the only
// consumer that reads that field is notification-service's template
// target_user_field.
//
// Returning the raw store_id there does not "degrade gracefully" — it writes a
// store-space id into a field the recipient lookup reads as an account_id.
// Under a re-keyed stint that addresses nobody, and it does so in total
// silence. Returning 0 hands the failure to a guard that already exists:
// notification-service's extractUserID rejects id <= 0, the worker skips the
// whole notification and logs "cannot extract user_id" at WARN. Delivery is
// not worse (an id that resolves to nobody was never delivered either) — the
// failure just stops being invisible.
//
// Do NOT copy this method into a gate or a self-comparison. There, 0 is
// actively dangerous: two unresolved ids both become 0 and compare EQUAL, which
// turns a silent fail-open into a false positive. Gates use the unresolved_ids
// signal directly and refuse (see im-service / social-service).
func (c *Client) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ResolvePrincipals(ctx, &sellerpb.ResolvePrincipalsRequest{Ids: []int64{storeID}})
	if err != nil {
		pkglogger.Warn(ctx, "seller ResolvePrincipals RPC failed; no seller principal for this notification",
			zap.Int64("store_id", storeID), zap.Error(err))
		return 0
	}
	if resp.GetCode() != 0 {
		pkglogger.Warn(ctx, "seller ResolvePrincipals returned a business error; no seller principal for this notification",
			zap.Int64("store_id", storeID), zap.Int32("code", resp.GetCode()), zap.String("message", resp.GetMessage()))
		return 0
	}
	for _, unresolved := range resp.GetUnresolvedIds() {
		if unresolved == storeID {
			// seller-service has no live sellers row for this store_id (soft-deleted
			// stint, or never existed). principals[storeID] carries an UNVERIFIED
			// echo-back of the input, not a real resolution — see
			// seller.proto ResolvePrincipalsResponse.unresolved_ids. Publishing that
			// echo-back would put a store-space id in an account-space field.
			pkglogger.Warn(ctx, "seller-service does not recognise this store_id (soft-deleted or unknown); no seller principal for this notification",
				zap.Int64("store_id", storeID))
			return 0
		}
	}
	if principal, ok := resp.GetPrincipals()[storeID]; ok {
		return principal
	}
	pkglogger.Warn(ctx, "seller ResolvePrincipals omitted the requested store_id from its principals map; no seller principal for this notification",
		zap.Int64("store_id", storeID))
	return 0
}

func (c *Client) ResolveStoreCode(ctx context.Context, storeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.BatchGetSellerMeta(ctx, &sellerpb.BatchGetSellerMetaRequest{StoreIds: []int64{storeID}})
	if err != nil {
		return "", err
	}
	if resp.GetCode() != 0 {
		return "", fmt.Errorf("BatchGetSellerMeta store_id %d: code=%d message=%s", storeID, resp.GetCode(), resp.GetMessage())
	}
	meta, ok := resp.GetSellers()[storeID]
	if !ok {
		return "", fmt.Errorf("BatchGetSellerMeta store_id %d: no meta returned", storeID)
	}
	return meta.GetStoreCode(), nil
}
