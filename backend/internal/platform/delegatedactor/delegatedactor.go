// Package delegatedactor 是代行寫入帶給資源模組的「代行者」：傳輸層的代行元件把它放進 ctx，資源模組在自己的 transaction 內為寫入發帶這份資料的事件，
// 稽核的訂閱者同 transaction 寫 SUCCEEDED（A7-D26）。資源模組只讀它、不 import delegation，所以放在 platform。
package delegatedactor

import "context"

// Actor 是一次代行寫入的代行者與請求資料。欄位全是事實，不含判斷：授權與稽核由訂閱者在同一個 transaction 內檢查與寫入。
type Actor struct {
	SessionID int64
	AdminID   int64
	// StoreID 是代行 session 的目標店家；資源模組的事件帶它，訂閱者核對它與被寫的店家相同。
	StoreID int64
	// Capability、Action、Resource 是被呼叫方法的代行宣告（delegation 的值域，資源模組不解讀）。
	Capability string
	Action     string
	Resource   string
	// RequestID 是 request_id（AIP-155），同一個 session 內去重。
	RequestID    string
	Reason       string
	NotifySeller bool
	// PayloadHash 綁定 RequestID 與請求內容。
	PayloadHash string
	// ResourceVersionBefore 是請求帶的資源版本（寫入前），沒有則為 nil。
	ResourceVersionBefore *int64
	CorrelationID         string
	UserAgent             string
}

type ctxKey struct{}

// With 回傳帶著代行者的 ctx。
func With(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// From 讀 ctx 裡的代行者；沒有（店主自己的寫入）回 false。
func From(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxKey{}).(Actor)
	return a, ok
}
