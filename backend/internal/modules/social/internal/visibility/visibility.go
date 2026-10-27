// Package visibility 是社群內容「公開可讀」「可見」兩類規則的入口（A7-1a，ADR 2026-09-27-module-internal-structure 的修訂）。
// 讀取內容的 query 用 go-jet 建，所有規則都從這裡組：
//
//   - Counted：內容「計入」的旗標（comments.counted、forum_entries.counted）。定義只有一份，在資料庫的 reevaluate_*（db/objects.sql），
//     輸入全是 social 自己的表（deleted_at、hides、inactive_authors）；寫入的 trigger、買家刪帳的訂閱者與每日對帳讓旗標保持正確，讀取與重算直接用旗標，
//     不重新推導（A7-1a，ADR 2026-10-23-social-counts-in-stats-tables 的修訂）。計數與旗標不依觀看者。
//   - 觀看者的規則（個人封鎖，A7-1b）：入口型別必帶的條件，不併進 Counted。觀看者封鎖的店，它的貼文與討論區對觀看者整個不出現
//     （ReadablePosts、ReadableForum）；與觀看者任一方向有個人封鎖的買家，他的留言與討論區留言（含底下的回覆）對觀看者整則不出現，
//     不留佔位項目（Comments、ForumEntries 的 Where）。佔位項目只有刪除與隱藏，那是對所有人一致的狀態。被排除的列不進「有可見回覆」的判斷，
//     回覆的讀取另外要求父列沒被排除（Admits）。店家封鎖買家只擋寫入，不在這裡（seller.StoreBlocksBuyer，social 的寫入檢查）。
//   - Readable*：容器（貼文、店家）的規則。列表對父鍵只檢查一次，不是每個子列一次。
//
// 觀看者是買家（Viewer.BuyerID），零值是未登入：匿名讀取的語句不含任何封鎖子句，不用 `$v IS NULL OR …` 的開關（generic plan 無法最佳化）。
// 店主以店家身分讀自己的店（OwnerPosts、reads 的 ListOwner*）、治理用的讀取（sanction）與全域彙總（hashtag）都以匿名建入口，不套個人封鎖。
// 封鎖來自 buyer.pub_blocked_stores()（觀看者封鎖店）與 buyer.pub_blocked_buyers()（買家之間，任一方向）（pubsql 的生成包裝）：以 EXISTS 內聯進述詞，
// 走 buyer.blocked_stores、buyer.blocked_buyers 的主鍵。
//
// 公開表只能經這裡的入口型別取得（Posts、Comments、ForumEntries）：reads 套件的 AST 測試禁止直接引用 jet 的 table.Posts、table.Comments、
// table.ForumEntries，所以漏掉規則只會是「沒有用入口」而不是「少寫一個條件」，前者由 golden 與資料集測試擋。
//
// 規則中的固定代碼（PUBLISHED、BUYER、STORE）一律是 SQL 常數（NewEnumValue），不是 bind 參數：generic plan 無法用 `$1` 證明 partial index 的述詞，
// 會退成 Seq Scan（量測 59 ms 對 0.7 ms；TestPostPublishedUsesInlineConstants 擋）。
package visibility

import (
	"time"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/jet/table"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/pubsql"
)

// Viewer 是讀取的人：買家（BuyerID）；零值是未登入。有觀看者的讀取多出封鎖的 NOT EXISTS，語句文字不同於匿名讀取。
type Viewer struct{ BuyerID int64 }

// Anonymous 是未登入的觀看者。
var Anonymous = Viewer{}

// StoreExposable 是「店家此刻可以公開曝光」：seller.pub_exposable_stores(at) 含該店。at 是請求的 clock，不用 now()。
func StoreExposable(at time.Time, storeID IntegerExpression) BoolExpression {
	return pubsql.SellerExposableStoresHasStoreID(TimestampzT(at), storeID)
}

// anonymous 回報觀看者是不是未登入：未登入的語句不含封鎖子句。
func (v Viewer) anonymous() bool { return v.BuyerID <= 0 }

// and 把非 nil 的條件用 AND 串起來；全是 nil 時回 TRUE。nil 表示「這個觀看者沒有這條規則」（匿名沒有封鎖）。
func and(parts ...BoolExpression) BoolExpression {
	var out BoolExpression
	for _, p := range parts {
		switch {
		case p == nil:
		case out == nil:
			out = p
		default:
			out = out.AND(p)
		}
	}
	if out == nil {
		return Bool(true)
	}
	return out
}

// storeBlocked 是 blocker 此刻封鎖了店 store：buyer.pub_blocked_stores() 的 EXISTS，走主鍵 (blocker_buyer_id, store_id)。
// 子查詢內的 store_id 是 pub_blocked_stores() 的欄位；blocker 與 store 可以是常數或外層的列。
func storeBlocked(blocker IntegerExpression, store IntegerExpression) BoolExpression {
	return pubsql.BuyerBlockedStoresHasBlockerBuyerID(blocker, IntegerColumn("store_id").EQ(store))
}

// buyerBlocked 是 blocker 此刻封鎖了買家 blocked：buyer.pub_blocked_buyers() 的 EXISTS，走主鍵 (blocker_buyer_id, blocked_buyer_id)。
// 子查詢內的 blocked_buyer_id 是 pub_blocked_buyers() 的欄位。
func buyerBlocked(blocker IntegerExpression, blocked IntegerExpression) BoolExpression {
	return pubsql.BuyerBlockedBuyersHasBlockerBuyerID(blocker, IntegerColumn("blocked_buyer_id").EQ(blocked))
}

// storeNotBlocked 是觀看者沒有封鎖這家店；匿名是 nil（沒有這條規則）。
func storeNotBlocked(v Viewer, storeID IntegerExpression) BoolExpression {
	if v.anonymous() {
		return nil
	}
	return NOT(storeBlocked(Int64(v.BuyerID), storeID))
}

// authorAdmitted 是內容的作者沒被觀看者的封鎖排除：作者是買家時，觀看者與他之間任一方向都沒有個人封鎖；店家作者不在這裡排除
// （觀看者封鎖該店時，容器條件已經擋下整家店）。匿名是 nil。
func authorAdmitted(v Viewer, kind ColumnString, id ColumnInteger) BoolExpression {
	if v.anonymous() {
		return nil
	}
	viewer := Int64(v.BuyerID)
	return kind.NOT_EQ(NewEnumValue("BUYER")).OR(NOT(buyerBlocked(viewer, id)).AND(NOT(buyerBlocked(id, viewer))))
}

// Posts 是貼文的入口：T 是已別名的 posts 表，cond 是這個入口必須帶的條件（用 Where 併進 query 的 WHERE）。
type Posts struct {
	T    *table.PostsTable
	cond BoolExpression
}

// PostPublished 是貼文自己的公開條件：已發布而且沒有刪除。與 posts_feed 這類 partial index 的述詞一致。
func PostPublished(p *table.PostsTable) BoolExpression {
	return p.Status.EQ(NewEnumValue("PUBLISHED")).AND(p.DeletedAt.IS_NULL())
}

// ReadablePosts 是觀看者可讀的貼文：已發布、沒有刪除、店家可曝光，而且觀看者沒有封鎖這家店。
func ReadablePosts(alias string, at time.Time, v Viewer) Posts {
	p := table.Posts.AS(alias)
	return Posts{T: p, cond: and(PostPublished(p), StoreExposable(at, p.StoreID), storeNotBlocked(v, p.StoreID))}
}

// OwnerPosts 是店主讀自己的貼文：任何狀態，不含刪除；店主以店家身分讀，不套個人封鎖。
func OwnerPosts(alias string, storeID int64) Posts {
	p := table.Posts.AS(alias)
	return Posts{T: p, cond: p.StoreID.EQ(Int64(storeID)).AND(p.DeletedAt.IS_NULL())}
}

// Where 是入口的條件加上 extra。
func (s Posts) Where(extra ...BoolExpression) BoolExpression {
	return and(append([]BoolExpression{s.cond}, extra...)...)
}

// Condition 是入口的條件本身（當作旗標投影時用）。
func (s Posts) Condition() BoolExpression { return s.cond }

// ReadableForum 是店家的討論區對觀看者可讀：店家可曝光，而且觀看者沒有封鎖這家店。storeID 可以是常數或外層的列。
func ReadableForum(at time.Time, v Viewer, storeID IntegerExpression) BoolExpression {
	return and(StoreExposable(at, storeID), storeNotBlocked(v, storeID))
}

// hidden 是「有未還原的隱藏」的子查詢；target 選 comment_id 或 forum_entry_id。只給 CommentIsHidden 這類要看隱藏本身的讀取；
// 內容是否可見、是否計入看旗標，不重新求值隱藏。
func hidden(target func(*table.HidesTable) ColumnString, id StringExpression) BoolExpression {
	h := table.Hides.AS("h")
	return EXISTS(SELECT(h.ID).FROM(h).WHERE(target(h).EQ(id).AND(h.RestoredAt.IS_NULL())))
}

// authorInactive：作者是買家而且在 inactive_authors（買家刪帳的事實的複本）。只用在佔位項目的 state（DELETED 或 HIDDEN），
// 只對輸出的頁內的列求值。把「作者是買家」放進子查詢，外層才是 semi-join。
func authorInactive(kind ColumnString, id ColumnInteger) BoolExpression {
	ia := table.InactiveAuthors.AS("ia")
	return EXISTS(SELECT(ia.BuyerID).FROM(ia).WHERE(ia.BuyerID.EQ(id).AND(kind.EQ(NewEnumValue("BUYER")))))
}

// Comments 是貼文的留言的入口：T 是已別名的 comments 表，cond 是觀看者的規則（作者沒被封鎖排除），用 Where 併進 query 的 WHERE。
type Comments struct {
	T    *table.CommentsTable
	v    Viewer
	cond BoolExpression
}

// CommentsOf 回傳別名 alias 的留言入口。
func CommentsOf(alias string, v Viewer) Comments {
	t := table.Comments.AS(alias)
	return Comments{T: t, v: v, cond: authorAdmitted(v, t.ActorKind, t.ActorID)}
}

// Where 是入口的條件（觀看者的規則）加上 extra。
func (c Comments) Where(extra ...BoolExpression) BoolExpression {
	return and(append([]BoolExpression{c.cond}, extra...)...)
}

// Admits 是 id 這則留言（不一定來自入口的列，例如請求帶來的父留言 ID）沒被觀看者的規則排除；匿名是 nil。回覆的讀取用它要求父列沒被排除。
func (c Comments) Admits(id StringExpression) BoolExpression {
	if c.cond == nil {
		return nil
	}
	a := CommentsOf("pc", c.v)
	return EXISTS(SELECT(a.T.ID).FROM(a.T).WHERE(a.Where(a.T.ID.EQ(id))))
}

// ParentAdmitted 是入口這一列若是回覆，它的父留言沒被觀看者的規則排除（頂層沒有父列）；匿名是 nil。讀單列（讚）時用，列表的回覆讀取用 Admits。
func (c Comments) ParentAdmitted() BoolExpression {
	adm := c.Admits(c.T.ParentID)
	if adm == nil {
		return nil
	}
	return c.T.ParentID.IS_NULL().OR(adm)
}

// CommentHiddenBy 是 id 這則留言此刻有未還原的隱藏（id 不一定來自入口的列，例如請求帶來的參數）。
func CommentHiddenBy(id StringExpression) BoolExpression {
	return hidden(func(h *table.HidesTable) ColumnString { return h.CommentID }, id)
}

// ForumEntryHiddenBy 是 id 這則討論區留言此刻有未還原的隱藏。
func ForumEntryHiddenBy(id StringExpression) BoolExpression {
	return hidden(func(h *table.HidesTable) ColumnString { return h.ForumEntryID }, id)
}

// Counted 是內容計入的旗標（定義在資料庫的 reevaluate_comments：沒有刪除、沒有未還原的隱藏、作者不在 inactive_authors）。
// 佔位項目的遮罩也看它：旗標為假而底下有可見的回覆時，項目變成佔位。不依觀看者。
func (c Comments) Counted() BoolExpression { return c.T.Counted }

// Gone 是佔位項目要顯示成 DELETED 的條件：已刪除，或作者是買家而且已刪帳。其餘不可見的佔位項目是 HIDDEN。
func (c Comments) Gone() BoolExpression {
	return c.T.DeletedAt.IS_NOT_NULL().OR(authorInactive(c.T.ActorKind, c.T.ActorID))
}

// ForumEntries 是店家討論區留言的入口，規則同 Comments。
type ForumEntries struct {
	T    *table.ForumEntriesTable
	v    Viewer
	cond BoolExpression
}

// ForumEntriesOf 回傳別名 alias 的討論區留言入口。
func ForumEntriesOf(alias string, v Viewer) ForumEntries {
	t := table.ForumEntries.AS(alias)
	return ForumEntries{T: t, v: v, cond: authorAdmitted(v, t.ActorKind, t.ActorID)}
}

// Where 同 Comments.Where。
func (e ForumEntries) Where(extra ...BoolExpression) BoolExpression {
	return and(append([]BoolExpression{e.cond}, extra...)...)
}

// Admits 同 Comments.Admits。
func (e ForumEntries) Admits(id StringExpression) BoolExpression {
	if e.cond == nil {
		return nil
	}
	a := ForumEntriesOf("pe", e.v)
	return EXISTS(SELECT(a.T.ID).FROM(a.T).WHERE(a.Where(a.T.ID.EQ(id))))
}

// Counted 同 Comments.Counted（定義在 reevaluate_forum_entries）。
func (e ForumEntries) Counted() BoolExpression { return e.T.Counted }

// Gone 同 Comments.Gone。
func (e ForumEntries) Gone() BoolExpression {
	return e.T.DeletedAt.IS_NOT_NULL().OR(authorInactive(e.T.ActorKind, e.T.ActorID))
}
