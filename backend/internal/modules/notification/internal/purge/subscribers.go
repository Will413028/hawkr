package purge

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 這個檔案只放同步訂閱者：它們在來源的 transaction 內執行，失敗會讓來源 rollback，所以只排清除 job（enqueue），
// 不碰 notification 的表、不讀別的模組（subscribers_test.go 守住）。

// OnAccountDeleted 訂閱 identity.AccountDeleted：清除帳號的全部受眾。
func (s *Service) OnAccountDeleted(ctx context.Context, e identity.AccountDeleted) error {
	return s.enqueue(ctx, RecipientArgs{AccountID: e.AccountID})
}

// OnBuyerDeleted 訂閱 buyer.BuyerDeleted：清除帳號的 BUYER 受眾。
func (s *Service) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return s.enqueue(ctx, RecipientArgs{AccountID: e.AccountID, Audience: string(model.AudienceBuyer)})
}

// OnStoreRetired 訂閱 seller.StoreRetired：清除帳號在這間店的 STORE 受眾。
func (s *Service) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return s.enqueue(ctx, RecipientArgs{AccountID: e.AccountID, Audience: string(model.AudienceStore), StoreID: e.StoreID})
}

func (s *Service) enqueue(ctx context.Context, a RecipientArgs) error {
	opts := a.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, a, &opts)
}
