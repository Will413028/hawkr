// Package calleridentity 保存單體提供的業務身分快照。
package calleridentity

import "context"

// Identity 僅由單體身分 header 的驗證入口建立；零值業務 ID 表示不存在。
type Identity struct {
	AccountID       int64
	IsAdmin         bool
	BuyerID         int64
	StoreID         int64
	SellerSuspended string
}

type key struct{}

// With 將已驗證的單體身分放入 context。
func With(ctx context.Context, id Identity) context.Context { return context.WithValue(ctx, key{}, id) }

// From 回傳單體身分與來源標記；ws-ticket 與訪客沒有此標記。
func From(ctx context.Context) (Identity, bool) { id, ok := ctx.Value(key{}).(Identity); return id, ok }
