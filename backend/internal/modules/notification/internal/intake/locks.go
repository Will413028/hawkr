package intake

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Locks 是寫給一個收件人之前要鎖的三個擁有者：帳號（identity）、買家（buyer）、店家（seller）。
type Locks struct {
	Accounts Accounts
	Buyers   Buyers
	Stores   Stores
}

// Gone 是收件人不能寫入的原因；GoneNone 表示鎖好了、可以寫。
type Gone string

const (
	GoneNone           Gone = ""
	GoneNoAccount      Gone = "no_account"
	GoneAccountDeleted Gone = "account_deleted"
	GoneNoBuyer        Gone = "no_buyer"
	GoneBuyerClosed    Gone = "buyer_closed"
	GoneStoreRetired   Gone = "store_retired"
)

// LockRecipient 在呼叫方的 transaction 內鎖住收件人（鎖順序：identity 帳號列、buyer 買家列或 seller 店家列，notification 的列最後，platform/lockorder），
// 並回報收件人是否還能寫：帳號不存在或已刪除、買家列不存在或已關閉、店家已退場（含不存在，兩者無法區分）都回對應的 Gone，不寫。
// 帳號只需要在寫入途中不被刪、不被改，所以用共享鎖，多個 worker 不互相等待；買家與店家用 FOR KEY SHARE，BuyerDeleted、StoreRetired 的 FOR UPDATE 要等這個 transaction 提交。
// 鎖持有到 transaction 結束。storeID 只在 STORE 受眾使用。
func LockRecipient(ctx context.Context, l Locks, audience model.Audience, accountID, storeID int64) (Gone, error) {
	deleted, err := l.Accounts.ShareLockAccount(ctx, accountID)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return GoneNoAccount, nil
		}
		return GoneNone, fmt.Errorf("鎖定收件帳號：%w", err)
	}
	if deleted {
		return GoneAccountDeleted, nil
	}
	switch audience {
	case model.AudienceBuyer:
		closed, err := l.Buyers.ShareLockByAccount(ctx, accountID)
		switch {
		case apperr.KindOf(err) == apperr.KindNotFound:
			return GoneNoBuyer, nil
		case err != nil:
			return GoneNone, fmt.Errorf("鎖定收件買家：%w", err)
		case closed:
			return GoneBuyerClosed, nil
		}
		return GoneNone, nil
	case model.AudienceStore:
		err := l.Stores.LockNotRetired(ctx, storeID)
		switch {
		case apperr.KindOf(err) == apperr.KindNotFound:
			return GoneStoreRetired, nil
		case err != nil:
			return GoneNone, fmt.Errorf("鎖定收件店家：%w", err)
		}
		return GoneNone, nil
	default:
		return GoneNone, fmt.Errorf("通知受眾 %q 不合法", audience)
	}
}
