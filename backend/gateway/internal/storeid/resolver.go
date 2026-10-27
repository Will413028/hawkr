package storeid

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/gateway/internal/calleridentity"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNoCurrentStore 表示該帳號目前沒有現任（active / suspended）store。
// optional-seller 路由（如 ?view=seller）應映射為 403 seller access required。
var ErrNoCurrentStore = errors.New("storeid: account has no current store")

type SellerGetter interface {
	CurrentStoreID(ctx context.Context, accountID int64) (int64, error)
}

// GetterFunc adapts a plain func to SellerGetter（測試 seam）。
type GetterFunc func(ctx context.Context, accountID int64) (int64, error)

// CurrentStoreID implements SellerGetter.
func (f GetterFunc) CurrentStoreID(ctx context.Context, accountID int64) (int64, error) {
	return f(ctx, accountID)
}

type Resolver struct{ seller SellerGetter }

// New 建立業務身分解析器。
func New(seller SellerGetter) *Resolver { return &Resolver{seller: seller} }

// CurrentStoreID resolves accountID to the CURRENT store_id.
// 回傳 ErrNoCurrentStore 表示無現任賣家（無 row / 已刪除 / 已關閉）。
func (r *Resolver) CurrentStoreID(ctx context.Context, accountID identity.AccountID) (identity.StoreID, error) {
	rawAccountID := int64(accountID)

	if caller, ok := calleridentity.From(ctx); ok && !caller.IsAdmin && caller.AccountID == rawAccountID {
		if caller.StoreID == 0 {
			return 0, ErrNoCurrentStore
		}
		return identity.StoreID(caller.StoreID), nil
	}

	storeID, err := r.seller.CurrentStoreID(ctx, rawAccountID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return 0, ErrNoCurrentStore
		}
		return 0, err
	}
	if storeID == 0 {
		return 0, fmt.Errorf("storeid: seller-service returned empty identity for account %d", rawAccountID)
	}
	return identity.StoreID(storeID), nil
}
