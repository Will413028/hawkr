package agent

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// Prospects 是 prospect 聯絡方式的 port，由 seller 模組實作（邀請的 email 快照從這裡讀；沒有 email 時 Email 是 nil）。
type Prospects interface {
	ProspectContact(ctx context.Context, id int64) (seller.ProspectContact, error)
}

var _ Prospects = (*seller.Module)(nil)
