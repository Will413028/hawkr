package accountdeletion

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/request"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Reader 是只讀刪帳申請的 accountdeletion：只需要資料庫，不需要 identity、buyer、seller 與訂單的 port。給不該為了讀一列申請而備齊整個
// 模組的組裝使用（本機種子 dev-seed 的前提檢查）。
type Reader struct{ d *db.DB }

// NewReader 建立唯讀的 accountdeletion；d 必填。
func NewReader(d *db.DB) *Reader {
	if d == nil {
		panic("accountdeletion: NewReader missing dependency")
	}
	return &Reader{d: d}
}

// AccountsWithOpenRequest 回傳 accountIDs 之中有還沒完成的刪帳申請（`PENDING_ORDERS`）的帳號，依 ID 排序。
func (r *Reader) AccountsWithOpenRequest(ctx context.Context, accountIDs []int64) ([]int64, error) {
	return request.AccountsWithOpenRequest(ctx, r.d, accountIDs)
}
