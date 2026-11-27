package intake

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)


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


// PurgeTokensOnSuspended 訂閱 accountsuspension.Suspended。店家停權（store suspension）不是這個事件，token 不動。
func (s *Service) PurgeTokensOnSuspended(ctx context.Context, e accountsuspension.Suspended) error {
	return s.deleteAccountTokens(ctx, e.AccountID)
}

// PurgeTokensOnAccountDeleted 訂閱 identity.AccountDeleted。
func (s *Service) PurgeTokensOnAccountDeleted(ctx context.Context, e identity.AccountDeleted) error {
	return s.deleteAccountTokens(ctx, e.AccountID)
}
