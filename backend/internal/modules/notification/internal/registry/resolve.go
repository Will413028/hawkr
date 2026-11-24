package registry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ErrBadParams 表示參數快照或型別的解析結果永遠不合法（解不開、收件人形狀錯）：重試也不會好，worker 取消 job。其餘錯誤（資料庫、逾時）是暫時性，由 River 重試。
var ErrBadParams = errors.New("通知參數永久不合法")

// MaxFollowerPage 是 StoreFollowers 與 fan-out 一頁的上限，等於 buyer.MaxLiveAccountsPage（根套件的測試守住兩者相等）。
const MaxFollowerPage = 1000

// permanent 把依賴模組回的 InvalidInput（呼叫方給的值不合法，重試也不會好）轉成 ErrBadParams，讓 worker 取消 job 而不是重試；其餘錯誤原樣回傳。
func permanent(err error) error {
	if err != nil && !errors.Is(err, ErrBadParams) && apperr.KindOf(err) == apperr.KindInvalidInput {
		return fmt.Errorf("%w：%w", ErrBadParams, err)
	}
	return err
}

// FollowerRef 是追蹤者的買家與所屬帳號，只有鍵。
type FollowerRef struct{ BuyerID, AccountID int64 }

// Resolver 是 worker 端解析收件人與封鎖要問的地方：事件只帶 store_id、buyer_id，沒有 account_id，所以由 worker 向 buyer、seller 讀。
// 同步訂閱者不碰它（只准 Accept）。實作加入呼叫方的 transaction（Process 在鎖定收件人之後做封鎖檢查）。
type Resolver interface {
	// BuyerAccounts 回傳存活（未關閉）買家的 buyer_id 到帳號；已關閉與不存在的買家不在結果裡。
	BuyerAccounts(ctx context.Context, buyerIDs []int64) (map[int64]int64, error)
	// StoreAccounts 回傳存活（未關閉）店家的 store_id 到所屬帳號；已關閉與不存在的店家不在結果裡。
	StoreAccounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error)
	// BuyersBlockedBetween 回報兩位買家之間任一方向是否有封鎖。
	BuyersBlockedBetween(ctx context.Context, buyerA, buyerB int64) (bool, error)
	// BuyerBlocksStore 回報買家是否封鎖了店。
	BuyerBlocksStore(ctx context.Context, buyerID, storeID int64) (bool, error)
	// StoreBlocksBuyer 回報店是否封鎖了買家。
	StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
	// StoreFollowers 列出追蹤店家、未關閉、追蹤時間不晚於 followedBy 的買家：afterBuyerID 之後（不含）依 buyer_id 由小到大最多 limit 筆（1 到 MaxFollowerPage），
	// 不足 limit 筆表示沒有更多。不加鎖。
	StoreFollowers(ctx context.Context, storeID, afterBuyerID int64, limit int, followedBy time.Time) ([]FollowerRef, error)
	// CaseReporters 列出 moderation 案件的檢舉人（買家或店家）：名次（依檢舉時間與檢舉 ID 由小到大，從 1 起）大於 afterRank 的最多 limit 位（1 到 MaxFollowerPage），
	// 依名次由小到大；不足 limit 筆表示沒有更多。檢舉列結案後不再增減，所以名次穩定，可當 fan-out 的游標。不加鎖。
	CaseReporters(ctx context.Context, caseID uuid.UUID, afterRank int64, limit int) ([]ReporterRef, error)
}

// ReporterRef 是案件的一位檢舉人：Rank 是它在案件檢舉人裡的名次，BuyerID 與 StoreID 恰好一個為正。
type ReporterRef struct {
	Rank     int64
	ReportID uuid.UUID
	BuyerID  int64
	StoreID  int64
}

// Principal 是事件裡的一方：Side 是 BUYER 或 STORE，ID 是該身分面的號碼（buyer_id 或 store_id，兩個號碼空間不可混用）。
type Principal struct {
	Side model.Audience
	ID   int64
}

// Actor 是解析過的 Principal：AccountID 是所屬帳號，已關閉或查不到時是 0（仍參與封鎖檢查，不參與自我略過）。
type Actor struct {
	Principal
	AccountID int64
}

// Expansion 是 worker 端解析出來的一位收件人：Recipient 是 (帳號, 店)，BuyerID 只在 BUYER 受眾填（封鎖檢查用），Actors 是觸發這則通知的人。
type Expansion struct {
	Recipient Recipient
	BuyerID   int64
	Actors    []Actor
}

// Expanded 是 Process 要用的收件人解析結果，來源是型別的 Expand（worker 端）或 Recipient（參數裡已有帳號）。
type Expanded struct {
	Resolved
	// Self 是收件人自己的 Principal（BUYER 是 buyer_id、STORE 是 store_id），零值表示不做封鎖檢查。
	Self Principal
	// Actors 是封鎖檢查的對象；沒有 actor 的型別是空的。
	Actors []Actor
}

func (x *Expansion) validate(aud model.Audience) error {
	r := x.Recipient
	if r.AccountID <= 0 {
		return fmt.Errorf("%w：解析結果沒有收件帳號", ErrBadParams)
	}
	switch aud {
	case model.AudienceBuyer:
		if x.BuyerID <= 0 || r.StoreID != 0 {
			return fmt.Errorf("%w：BUYER 收件人要有 buyer_id、不能有 store_id（%d、%d）", ErrBadParams, x.BuyerID, r.StoreID)
		}
	case model.AudienceStore:
		if r.StoreID <= 0 || x.BuyerID != 0 {
			return fmt.Errorf("%w：STORE 收件人要有 store_id、不能有 buyer_id（%d、%d）", ErrBadParams, r.StoreID, x.BuyerID)
		}
	}
	for _, a := range x.Actors {
		if !slices.Contains(model.Audiences(), a.Side) || a.ID <= 0 {
			return fmt.Errorf("%w：actor %v 不合法", ErrBadParams, a.Principal)
		}
	}
	return nil
}

// ResolveActors 把 Principal 解析成 Actor（帳號查不到時是 0）；買家與店各批次查一次。
func ResolveActors(ctx context.Context, r Resolver, ps ...Principal) ([]Actor, error) {
	var buyers, stores []int64
	for _, p := range ps {
		if p.ID <= 0 {
			return nil, fmt.Errorf("%w：actor %v 的號碼不合法", ErrBadParams, p)
		}
		switch p.Side {
		case model.AudienceBuyer:
			buyers = append(buyers, p.ID)
		case model.AudienceStore:
			stores = append(stores, p.ID)
		default:
			return nil, fmt.Errorf("%w：actor 身分面 %q 不合法", ErrBadParams, p.Side)
		}
	}
	var buyerAcc, storeAcc map[int64]int64
	var err error
	if len(buyers) > 0 {
		if buyerAcc, err = r.BuyerAccounts(ctx, buyers); err != nil {
			return nil, fmt.Errorf("解析 actor 買家帳號：%w", err)
		}
	}
	if len(stores) > 0 {
		if storeAcc, err = r.StoreAccounts(ctx, stores); err != nil {
			return nil, fmt.Errorf("解析 actor 店家帳號：%w", err)
		}
	}
	out := make([]Actor, len(ps))
	for i, p := range ps {
		out[i] = Actor{Principal: p}
		if p.Side == model.AudienceBuyer {
			out[i].AccountID = buyerAcc[p.ID]
		} else {
			out[i].AccountID = storeAcc[p.ID]
		}
	}
	return out, nil
}

// BuyerRecipient 是 Expand 的共用寫法：收件人是買家 buyerID，查帳號；買家已關閉或不存在時回 nil（沒有收件人，Process 略過）。
func BuyerRecipient(ctx context.Context, r Resolver, buyerID int64, actors ...Principal) (*Expansion, error) {
	if buyerID <= 0 {
		return nil, fmt.Errorf("%w：收件買家號碼 %d 不合法", ErrBadParams, buyerID)
	}
	accounts, err := r.BuyerAccounts(ctx, []int64{buyerID})
	if err != nil {
		return nil, fmt.Errorf("解析收件買家帳號：%w", err)
	}
	account, ok := accounts[buyerID]
	if !ok {
		return nil, nil
	}
	resolved, err := ResolveActors(ctx, r, actors...)
	if err != nil {
		return nil, err
	}
	return &Expansion{Recipient: Recipient{AccountID: account}, BuyerID: buyerID, Actors: resolved}, nil
}

// StoreRecipient 是 Expand 的共用寫法：收件人是店家 storeID 的所屬帳號；店家已關閉或不存在時回 nil。
func StoreRecipient(ctx context.Context, r Resolver, storeID int64, actors ...Principal) (*Expansion, error) {
	if storeID <= 0 {
		return nil, fmt.Errorf("%w：收件店家號碼 %d 不合法", ErrBadParams, storeID)
	}
	accounts, err := r.StoreAccounts(ctx, []int64{storeID})
	if err != nil {
		return nil, fmt.Errorf("解析收件店家帳號：%w", err)
	}
	account, ok := accounts[storeID]
	if !ok {
		return nil, nil
	}
	resolved, err := ResolveActors(ctx, r, actors...)
	if err != nil {
		return nil, err
	}
	return &Expansion{Recipient: Recipient{AccountID: account, StoreID: storeID}, Actors: resolved}, nil
}

// PageRequest 是 fan-out 一頁的要求：After 是上一頁的游標（0 是從頭），Limit 是這頁最多幾個（1 到 MaxFollowerPage），
// OccurredAt 是母意圖的事件時間（只有事件發生時就在名單上的收件人才收得到，例如事件之後才追蹤的買家不通知）。
type PageRequest struct {
	After      int64
	Limit      int
	OccurredAt time.Time
}

// Target 是 fan-out 的一位收件人：ID 是它在這次派送裡的唯一號碼（依 ID 由小到大分頁，也是子意圖冪等鍵的收件人部分，例如追蹤者的 buyer_id），
// Params 是子型別的參數（內含該收件人，之後由子型別的 Expand 解析）。
type Target struct {
	ID     int64
	Params any
}

// TargetPage 是 fan-out 的一頁：Targets 是這頁要通知的收件人（可以比底層讀到的少，例如濾掉不該通知的人）；Next 是下一頁的游標，
// 是底層已讀到的最後位置，不是 Targets 的最後一個 ID（Targets 為空時照樣往前）；Done 為 true 表示底層已經沒有更多。游標與結束由底層的分頁決定，
// 不能從 Targets 的數量推得：被濾掉的列會讓一頁看起來不滿。
type TargetPage struct {
	Targets []Target
	Next    int64
	Done    bool
}

// FollowersCursor 由 StoreFollowers 這一頁的原始結果得出 TargetPage 的游標與結束：沒有讀滿 limit 筆就是結束，否則游標是最後一位追蹤者的 buyer_id。
func FollowersCursor(page []FollowerRef, after int64, limit int) (next int64, done bool) {
	if len(page) == 0 {
		return after, true
	}
	return page[len(page)-1].BuyerID, len(page) < limit
}

// Broadcast 宣告型別是 fan-out 的母意圖：它自己不產生收件匣列，worker 分頁呼叫 Targets，為每位收件人建一個 Child 型別的子意圖。
type Broadcast[P any] struct {
	// Child 是子型別名稱；子型別要宣告 Spawned。
	Child string
	// Targets 回傳 req.After 之後的一頁收件人（依 ID 由小到大、最多 req.Limit 個）與下一頁的游標，見 TargetPage。
	Targets func(ctx context.Context, r Resolver, p P, req PageRequest) (TargetPage, error)
}

// SpawnedKey 是 fan-out 子意圖的冪等鍵：母意圖的冪等鍵加收件人 ID，再加子型別名稱前綴。同一個母意圖對同一位收件人只會有一個子意圖，
// 對不同收件人一定不同；重跑任何一頁都不會重複。
func SpawnedKey(childName, parentKey string, targetID int64) string {
	return scopedKey(childName, parentKey+"/"+strconv.FormatInt(targetID, 10))
}
