package intake

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

// 這個檔案只放同步訂閱者：它們在來源的 transaction 內執行，失敗會讓來源 rollback，所以只做 Accept（登記意圖並排 job），
// 不讀別的模組、不組文字、不寫收件匣（subscribers_test.go 守住）。

// OnSuspended 訂閱 accountsuspension.Suspended。
func (s *Service) OnSuspended(ctx context.Context, e accountsuspension.Suspended) error {
	return s.Accept(ctx, "account_suspended", registry.AccountSuspendedParams{
		AccountID: e.AccountID, SuspensionID: e.SuspensionID, Category: e.Category, SuspendedAt: e.SuspendedAt,
	})
}

// OnUnsuspended 訂閱 accountsuspension.Unsuspended。
func (s *Service) OnUnsuspended(ctx context.Context, e accountsuspension.Unsuspended) error {
	return s.Accept(ctx, "account_unsuspended", registry.AccountUnsuspendedParams{
		AccountID: e.AccountID, SuspensionID: e.SuspensionID, ReleasedAt: e.ReleasedAt,
	})
}
