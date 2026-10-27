package client

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/im-service/internal/client/buyer"
	"github.com/Will413028/hawkr/im-service/internal/client/media"
	"github.com/Will413028/hawkr/im-service/internal/client/order"
	"github.com/Will413028/hawkr/im-service/internal/client/seller"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"go.uber.org/fx"
)

// Module provides gRPC client dependencies.
var Module = fx.Module("client",
	fx.Provide(buyer.NewClient),
	fx.Provide(newBuyerClientAdapter),
	fx.Provide(seller.NewClient),
	fx.Provide(newSellerClientAdapter),
	fx.Provide(order.NewClient),
	fx.Provide(newOrderClientAdapter),
	fx.Provide(media.NewClient),
	fx.Provide(newMediaClientAdapter),
	fx.Provide(usecase.NewProfileCache),
)

// buyerClientAdapter adapts buyer.Client to usecase.BuyerClient interface.
type buyerClientAdapter struct {
	client *buyer.Client
	cache  *usecase.ProfileCache
}

func newBuyerClientAdapter(c *buyer.Client, cache *usecase.ProfileCache) usecase.BuyerClient {
	return &buyerClientAdapter{client: c, cache: cache}
}

func (a *buyerClientAdapter) GetProfileBatch(ctx context.Context, buyerIDs []int64) (map[int64]usecase.BuyerProfileInfo, error) {
	result := make(map[int64]usecase.BuyerProfileInfo, len(buyerIDs))
	var missing []int64
	for _, id := range buyerIDs {
		if cached, ok := a.cache.GetBuyer(id); ok {
			result[id] = cached
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}
	profiles, err := a.client.GetProfileBatch(ctx, missing)
	if err != nil {
		// 已命中快取的部分照樣回傳；err 讓呼叫端知道 missing 那批是「不知道」
		// 而非「不存在」。此處不再自行吞掉——決定權在呼叫端。
		return result, fmt.Errorf("fetch buyer profiles: %w", err)
	}
	for id, p := range profiles {
		info := usecase.BuyerProfileInfo{Nickname: p.Nickname, Avatar: p.Avatar}
		result[id] = info
		a.cache.SetBuyer(id, info)
	}
	return result, nil
}

// ResolvePrincipal delegates to the underlying buyer-service gRPC client
// (② buyer re-key W2/W3 cross-role guard). No caching: this is a point
// lookup on the self-message send path, not a batch profile fetch.
func (a *buyerClientAdapter) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	return a.client.ResolvePrincipal(ctx, buyerID)
}

// ResolvePrincipalStrict delegates to the underlying buyer-service gRPC client
// and keeps the three-way answer intact (#2789). Used by the room-identity
// guard in SendMessage, which must not treat an echo-back as a resolution.
func (a *buyerClientAdapter) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (usecase.PrincipalResolution, error) {
	accountID, found, err := a.client.ResolvePrincipalStrict(ctx, buyerID)
	if err != nil {
		return usecase.PrincipalResolution{}, err
	}
	return usecase.PrincipalResolution{AccountID: accountID, Found: found}, nil
}

// ListIDsByAccount delegates to the underlying buyer-service gRPC client
// (account.deleted/buyer.deleted PII-purge path, D8). No caching: this is a
// one-off historical-enumeration lookup, not a batch profile fetch.
func (a *buyerClientAdapter) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	return a.client.ListIDsByAccount(ctx, accountID)
}

// sellerClientAdapter adapts seller.Client to usecase.SellerClient interface.
type sellerClientAdapter struct {
	client *seller.Client
	cache  *usecase.ProfileCache
}

func newSellerClientAdapter(c *seller.Client, cache *usecase.ProfileCache) usecase.SellerClient {
	return &sellerClientAdapter{client: c, cache: cache}
}

func (a *sellerClientAdapter) GetProfileBatch(ctx context.Context, storeIDs []int64) (map[int64]usecase.SellerProfileInfo, error) {
	result := make(map[int64]usecase.SellerProfileInfo, len(storeIDs))
	var missing []int64
	for _, id := range storeIDs {
		if cached, ok := a.cache.GetSeller(id); ok {
			result[id] = cached
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}
	profiles, err := a.client.GetProfileBatch(ctx, missing)
	if err != nil {
		return result, fmt.Errorf("fetch seller profiles: %w", err)
	}
	for id, p := range profiles {
		info := usecase.SellerProfileInfo{
			Nickname:    p.Nickname,
			Avatar:      p.Avatar,
			StoreName:   p.StoreName,
			LogoKey:     p.LogoKey,
			StoreCode:   p.StoreCode,
			StoreClosed: p.StoreClosed,
		}
		result[id] = info
		a.cache.SetSeller(id, info)
	}
	return result, nil
}

// ResolvePrincipal delegates to the underlying seller-service gRPC client
// (Plan 2b cross-role guard). No caching: this is a point lookup on the
// self-message send path, not a batch profile fetch.
func (a *sellerClientAdapter) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	return a.client.ResolvePrincipal(ctx, storeID)
}

// ResolvePrincipalStrict delegates to the underlying seller-service gRPC client
// and keeps the three-way answer intact (#2789). Used by the room-identity
// guard in SendMessage, which must not treat an echo-back as a resolution.
func (a *sellerClientAdapter) ResolvePrincipalStrict(ctx context.Context, storeID int64) (usecase.PrincipalResolution, error) {
	accountID, found, err := a.client.ResolvePrincipalStrict(ctx, storeID)
	if err != nil {
		return usecase.PrincipalResolution{}, err
	}
	return usecase.PrincipalResolution{AccountID: accountID, Found: found}, nil
}

// ListIDsByAccount delegates to the underlying seller-service gRPC client
// (account.deleted/buyer.deleted PII-purge path, D8). No caching: this is a
// one-off historical-enumeration lookup, not a batch profile fetch.
func (a *sellerClientAdapter) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	return a.client.ListIDsByAccount(ctx, accountID)
}

// orderClientAdapter adapts order.Client to usecase.OrderClient interface.
type orderClientAdapter struct {
	client *order.Client
}

func newOrderClientAdapter(c *order.Client) usecase.OrderClient {
	return &orderClientAdapter{client: c}
}

func (a *orderClientAdapter) GetOrderParties(ctx context.Context, orderNo string) (usecase.OrderParties, error) {
	p, err := a.client.GetOrderParties(ctx, orderNo)
	if err != nil {
		return usecase.OrderParties{}, err
	}
	return usecase.OrderParties{BuyerID: p.BuyerID, StoreID: p.StoreID}, nil
}

func (a *orderClientAdapter) HasOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, string, error) {
	return a.client.HasOrderBetween(ctx, buyerID, storeID)
}

// mediaClientAdapter adapts media.Client to usecase.MediaClient interface.
type mediaClientAdapter struct {
	client *media.Client
}

func newMediaClientAdapter(c *media.Client) usecase.MediaClient {
	return &mediaClientAdapter{client: c}
}

func (a *mediaClientAdapter) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (usecase.AssetInfo, error) {
	return a.client.FinalizeAndLink(ctx, stagedToken, refType, refID)
}

func (a *mediaClientAdapter) ReleaseChatReferences(ctx context.Context, refIDs []string) error {
	return a.client.ReleaseChatReferences(ctx, refIDs)
}
