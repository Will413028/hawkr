// Package blocks 是封鎖入口共用的 handler 規則（A7-2e，A7-D23 O4）：跨身分的「同一個人」規則、從內容參照解析被封鎖的作者，與名單的顯示資料。
// buyer 與 seller 模組互不依賴，模組的 API 擋不了「買家封鎖自己的店」「店家封鎖店主本人的買家身分」，所以規則放在依賴兩邊的這一層。
// A6 的聊天、評價出現第二個封鎖入口時，這個套件升成編排模組（決定簡報 §1.3）。
package blocks

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// BuyerAccounts 是買家身分所屬帳號的來源（*buyer.Module）。
type BuyerAccounts interface {
	AccountIDByBuyerID(ctx context.Context, buyerID int64) (int64, error)
}

// StoreReader 是店家（含已關閉的）與它的擁有者帳號的來源（*seller.Module）。
type StoreReader interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Identities 是兩種封鎖身分各自屬於哪個帳號的來源：同一個帳號就是同一個人。
type Identities struct {
	Buyers BuyerAccounts
	Stores StoreReader
}

// RejectSamePerson 是所有封鎖入口共用的跨身分規則：買家封鎖自己的店、店家封鎖店主本人的買家身分，都回 invalid_argument。
// 請求本身永遠不成立（不是等某個狀態被修正），所以不是 failed_precondition（API 設計規範（Connect）第八節：要等狀態被修正才用 failed_precondition；
// 同樣的先例是 FollowStore 追蹤自己的店）。店家不存在回 not_found；買家身分查不到帳號時回 not_found。
func (i Identities) RejectSamePerson(ctx context.Context, buyerID, storeID int64) error {
	store, err := i.Stores.Store(ctx, storeID)
	if err != nil {
		return err
	}
	owner, err := i.Buyers.AccountIDByBuyerID(ctx, buyerID)
	if err != nil {
		return err
	}
	if owner == store.AccountID {
		return apperr.InvalidInput("不能封鎖自己")
	}
	return nil
}

// Authors 是從內容參照解析作者的來源（*social.Module）：只回呼叫者讀得到的內容的作者。
type Authors interface {
	ContentAuthorFor(ctx context.Context, c social.Caller, t social.Target) (social.ContentAuthor, error)
}

// Sources 是封鎖入口的 handler 規則的來源（buyer 與 seller 的 App 各收一份）：內容參照解析與跨身分規則的身分來源。
type Sources struct {
	Authors    Authors
	Identities Identities
}

// Complete 回報四個來源都有（組裝點漏接時 NewApp 回錯誤，不讓請求才 nil panic）。
func (s Sources) Complete() bool {
	return s.Authors != nil && s.Identities.Buyers != nil && s.Identities.Stores != nil
}

// Ref 是請求裡的內容參照：留言或討論區留言，恰好設一個。
type Ref struct {
	CommentID    string
	ForumEntryID string
}

// Target 解析參照；兩個都沒設或都設了回 invalid_argument，ID 格式錯誤回 invalid_argument。
func (r Ref) Target() (social.Target, error) {
	switch {
	case r.CommentID != "" && r.ForumEntryID != "":
		return social.Target{}, rpc.RequiredField("source", "comment_id 與 forum_entry_id 只能設一個")
	case r.CommentID != "":
		id, err := rpc.ParseUUID("comment_id", r.CommentID)
		return social.Target{Kind: social.TargetComment, ID: id}, err
	case r.ForumEntryID != "":
		id, err := rpc.ParseUUID("forum_entry_id", r.ForumEntryID)
		return social.Target{Kind: social.TargetForumEntry, ID: id}, err
	}
	return social.Target{}, rpc.RequiredField("source", "必須設 comment_id 或 forum_entry_id")
}

// ResolveAuthor 把參照解析成作者；呼叫者讀不到（不存在、刪除、隱藏、被封鎖排除、貼文不公開）一律 not_found，不區分原因。
func ResolveAuthor(ctx context.Context, authors Authors, c social.Caller, ref Ref) (social.ContentAuthor, error) {
	t, err := ref.Target()
	if err != nil {
		return social.ContentAuthor{}, err
	}
	got, err := authors.ContentAuthorFor(ctx, c, t)
	if err != nil {
		return social.ContentAuthor{}, err
	}
	if !got.Found {
		return social.ContentAuthor{}, apperr.NotFound("找不到這則內容")
	}
	return got, nil
}

// BuyerView 是名單裡一位買家的顯示資料：Deleted 為真時沒有名稱與頭像。
type BuyerView struct {
	ID          int64
	Deleted     bool
	DisplayName string
	Avatar      *string
}

// BuyerDisplays 是買家顯示資料的批次來源（*buyer.Module）；最多 buyer.MaxBatchSize 個。
type BuyerDisplays interface {
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
}

// BuyerViews 以一次批次查詢組出名單的買家：已關閉的帳號顯示成已刪除（同 social 的 Actor），查不到的 ID 也是已刪除，不讓名單的列消失，
// 使用者才能解除。
func BuyerViews(ctx context.Context, d BuyerDisplays, ids []int64) (map[int64]BuyerView, error) {
	out := make(map[int64]BuyerView, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := d.BatchDisplay(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		v := BuyerView{ID: r.BuyerID, Deleted: r.Deleted}
		if !r.Deleted {
			if r.DisplayName != nil {
				v.DisplayName = *r.DisplayName
			}
			v.Avatar = r.Avatar
		}
		out[r.BuyerID] = v
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = BuyerView{ID: id, Deleted: true}
		}
	}
	return out, nil
}

// LiveBuyer 回報買家是不是存在而且沒有關閉；封鎖的對象必須是（seller 的封鎖不檢查買家是否存在）。不是回 not_found。
func LiveBuyer(ctx context.Context, d BuyerDisplays, buyerID int64) error {
	views, err := BuyerViews(ctx, d, []int64{buyerID})
	if err != nil {
		return err
	}
	if views[buyerID].Deleted {
		return apperr.NotFound("找不到這位買家")
	}
	return nil
}
