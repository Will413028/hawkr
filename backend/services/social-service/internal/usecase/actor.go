package usecase

import "github.com/Will413028/hawkr/social-service/internal/domain"

// Caller 是呼叫者此刻的兩個業務身分（0 = 沒有）。互動者是 (kind, id)：buyer_id 與 store_id
// 各自連號，同一個數值在兩邊是不同的人，所以不可只比 id。
type Caller struct {
	BuyerID int64
	StoreID int64
}

// Actor 是一次互動實際使用的身分。零值表示沒有身分（訪客）。
type Actor struct {
	Kind int16
	ID   int64
}

// IsZero 回報這是不是沒有身分。
func (a Actor) IsZero() bool { return a.ID == 0 }

// OwnsStore 回報呼叫者此刻的店是不是 ownerStoreID。
func (c Caller) OwnsStore(ownerStoreID int64) bool {
	return c.StoreID != 0 && c.StoreID == ownerStoreID
}

// IsAuthor 回報 (kind, id) 是不是呼叫者自己的某個身分。
func (c Caller) IsAuthor(kind int16, id int64) bool {
	switch kind {
	case domain.ActorKindBuyer:
		return c.BuyerID != 0 && c.BuyerID == id
	case domain.ActorKindStore:
		return c.StoreID != 0 && c.StoreID == id
	}
	return false
}

// ViewerActor 是讀取時的互動身分（讚的個人化、封鎖過濾）：內容屬於呼叫者的店 → 店家；
// 否則 → 買家；兩者都沒有 → 零值。
func (c Caller) ViewerActor(ownerStoreID int64) Actor {
	if c.OwnsStore(ownerStoreID) {
		return Actor{Kind: domain.ActorKindStore, ID: c.StoreID}
	}
	if c.BuyerID != 0 {
		return Actor{Kind: domain.ActorKindBuyer, ID: c.BuyerID}
	}
	return Actor{}
}

// DeriveActor 是寫入時的互動身分：規則同 ViewerActor，但沒有任何可用身分時回 ErrForbidden。
// 店家身分只用在回應自己內容（ownerStoreID 是呼叫者的店）的動作，其餘一律本人。
func (c Caller) DeriveActor(ownerStoreID int64) (Actor, error) {
	actor := c.ViewerActor(ownerStoreID)
	if actor.IsZero() {
		return Actor{}, ErrForbidden
	}
	return actor, nil
}
