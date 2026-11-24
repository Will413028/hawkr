package intake

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
)

// deleteAccountTokens 在 ctx 的 transaction 內刪掉帳號的全部裝置 token（兩支 App 都刪）；沒有 token 也成功。
func (s *Service) deleteAccountTokens(ctx context.Context, accountID int64) error {
	if _, err := store.New(s.d.DB.Conn(ctx)).DeleteDeviceTokensByAccount(ctx, accountID); err != nil {
		return fmt.Errorf("刪除帳號 %d 的裝置 token：%w", accountID, err)
	}
	return nil
}
