package request

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// AccountsWithOpenRequest 回傳 accountIDs 之中有還沒完成的刪帳申請的帳號，依 ID 排序；只讀，不需要 Service 的其他依賴（Reader 用）。
func AccountsWithOpenRequest(ctx context.Context, d *db.DB, accountIDs []int64) ([]int64, error) {
	return store.New(d.Conn(ctx)).AccountsWithOpenRequest(ctx, accountIDs)
}
