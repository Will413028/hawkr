// Package actor 推導一次寫入的互動者 (kind, id) 並檢查身分的前提：店家身分要有能力，買家身分要是營運中的買家。
package actor

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Buyers 是買家身分的檢查需要的 buyer 公開 API：在呼叫方的 transaction 內以共享鎖確認買家營運中，鎖持有到 transaction 結束，
// 與刪帳申請（BeginDeletion 的排他鎖）序列化。
type Buyers interface {
	LockActive(ctx context.Context, buyerID int64) error
}

// StoreBlocks 是店家封鎖檢查需要的 seller 公開 API：店家是否封鎖了買家（單向，只擋寫入）。加入呼叫方的 transaction。
type StoreBlocks interface {
	StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
}

// Resolver 推導互動者。
type Resolver struct {
	Buyers Buyers
	Blocks StoreBlocks
}

// ViewerFor 是呼叫者讀取店 ownerStoreID 的內容時的觀看者：呼叫者此刻的店就是這家店時以店家身分讀，不套個人封鎖（零值觀看者）；
// 否則是買家本人。讀取類的寫入前置檢查也用它，讀與寫對「看得到」的定義一致。
func ViewerFor(c model.Caller, ownerStoreID int64) visibility.Viewer {
	if c.OwnsStore(ownerStoreID) {
		return visibility.Anonymous
	}
	return visibility.Viewer{BuyerID: c.BuyerID}
}

// requireBuyer 要求 buyerID 是營運中（ACTIVE）的買家。必須在寫入的 transaction 內呼叫，而且在寫入之前：買家列的共享鎖持有到
// transaction 結束，刪帳申請（BeginDeletion）要等這個寫入提交才能完成，所以清除 job 一定看得到這次寫入的列；申請之後才開始的寫入
// 在這裡被擋下。鎖的順序：買家列先於 social 自己的表（identity → buyer → seller → social）。
func (r Resolver) requireBuyer(ctx context.Context, buyerID int64) error {
	if buyerID <= 0 {
		return apperr.PermissionDenied("%s", text.CallerBuyerRequired(uitext.LocaleFrom(ctx)))
	}
	return r.Buyers.LockActive(ctx, buyerID)
}

// buyerWrite 是買家身分對店 storeID 的寫入前置：買家營運中，而且這家店沒有封鎖他（PermissionDenied）。所有「新增或改寫內容」的買家身分都經這裡，
// 所以店家封鎖的檢查只有這一處：入口不呼叫它就拿不到互動者。
func (r Resolver) buyerWrite(ctx context.Context, buyerID, storeID int64) error {
	if err := r.requireBuyer(ctx, buyerID); err != nil {
		return err
	}
	blocked, err := r.Blocks.StoreBlocksBuyer(ctx, storeID, buyerID)
	if err != nil {
		return fmt.Errorf("social：讀取店家 %d 的封鎖：%w", storeID, err)
	}
	if blocked {
		return apperr.PermissionDenied("%s", text.BlockStoreBlockedBuyer(uitext.LocaleFrom(ctx)))
	}
	return nil
}

// ForWrite 推導寫入的互動者：呼叫者此刻的店擁有資源（ownerStoreID）時是 STORE，要有 storeCap；否則是 BUYER，要是營運中的買家，而且資源所屬的店
// 沒有封鎖他。兩者都不是回 PermissionDenied。店家身分不退回買家身分：停權的店在自己的貼文上寫入就是被擋。
func (r Resolver) ForWrite(ctx context.Context, c model.Caller, ownerStoreID int64, storeCap seller.Capability) (model.Actor, error) {
	if c.OwnsStore(ownerStoreID) {
		if err := seller.Authorize(*c.Store, storeCap); err != nil {
			return model.Actor{}, err
		}
		return model.Actor{Kind: model.ActorStore, ID: c.Store.StoreID}, nil
	}
	if err := r.buyerWrite(ctx, c.BuyerID, ownerStoreID); err != nil {
		return model.Actor{}, err
	}
	return model.Actor{Kind: model.ActorBuyer, ID: c.BuyerID}, nil
}

// ForAuthor 是改寫自己既有內容（編輯）的前置：作者是店家時要有 storeCap；作者是買家時與 ForWrite 的買家身分相同（營運中、店沒有封鎖他）。
func (r Resolver) ForAuthor(ctx context.Context, c model.Caller, author model.Actor, ownerStoreID int64, storeCap seller.Capability) error {
	if author.Kind == model.ActorStore {
		return seller.Authorize(*c.Store, storeCap)
	}
	return r.buyerWrite(ctx, author.ID, ownerStoreID)
}

// ForBuyerWrite 是只有買家能做的新增（貼文讚）的前置：營運中，而且店 storeID 沒有封鎖他。
func (r Resolver) ForBuyerWrite(ctx context.Context, buyerID, storeID int64) error {
	return r.buyerWrite(ctx, buyerID, storeID)
}

// ForSelfService 是不受店家封鎖影響的買家動作的前置：只確認買家營運中。用在分享，以及其他不新增互動內容的動作；刪除自己的內容與取消讚
// 不需要買家身分檢查，店家封鎖不能攔住自清的手。
func (r Resolver) ForSelfService(ctx context.Context, buyerID int64) error {
	return r.requireBuyer(ctx, buyerID)
}

// ForViewer 推導讀取時的互動者（讚的個人化）：不檢查能力與狀態；沒有任何身分時 ok 為假。
func ForViewer(c model.Caller, ownerStoreID int64) (a model.Actor, ok bool) {
	switch {
	case c.OwnsStore(ownerStoreID):
		return model.Actor{Kind: model.ActorStore, ID: c.Store.StoreID}, true
	case c.BuyerID != 0:
		return model.Actor{Kind: model.ActorBuyer, ID: c.BuyerID}, true
	}
	return model.Actor{}, false
}
