package reads

import (
	"time"
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/jet/table"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// 列表項目的狀態（內部的 SQL 常數，對外的型別是 model.CommentState）。
const (
	stateVisible = "VISIBLE"
	stateDeleted = "DELETED"
	stateHidden  = "HIDDEN"
)

// CommentRow 是留言列表的一列。State 不是 VISIBLE 的佔位項目，ActorKind 是空字串、ActorID 是 0、Body 是空字串，呼叫方不得使用。
type CommentRow struct {
	ID         uuid.UUID
	PostID     uuid.UUID
	ParentID   *uuid.UUID
	ReplyToID  *uuid.UUID
	State      string
	ActorKind  string
	ActorID    int64
	Body       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LikeCount  int64
	ReplyCount int64
}

// CommentPage 是留言列表的游標與筆數：(created_at, id) 由小到大。
type CommentPage struct {
	AfterCreated time.Time
	AfterID      uuid.UUID
	Limit        int64
}

func afterComment(created ColumnTimestampz, id ColumnString, pg CommentPage) BoolExpression {
	return ROW(created, id).GT(ROW(TimestampzT(pg.AfterCreated), UUID(pg.AfterID)))
}

// placeholderComment 的內文與作者只在 visible 時給，否則是空值。
func masked(visible BoolExpression, col Expression, empty Expression) Expression {
	return COALESCE(CASE().WHEN(visible).THEN(col), empty)
}

// topLevelComments 是貼文的頂層留言，含佔位項目：自己可見，或不可見但底下有可見的回覆（state 是 DELETED 或 HIDDEN）。
// 容器的規則（貼文公開可讀，或店主讀自己的貼文）對父鍵 postID 只檢查一次，不是每個留言一次。
// 觀看者的規則（被封鎖排除的作者）在入口的 Where：被排除的頂層整則不出現，不變成佔位項目；回覆裡被排除的列不進「有可見回覆」的判斷。
func topLevelComments(postID uuid.UUID, v visibility.Viewer, pg CommentPage, container BoolExpression) SelectStatement {
	c := visibility.CommentsOf("c", v)
	r := visibility.CommentsOf("r", v)
	s := table.CommentStats.AS("s")
	vis := LATERAL(SELECT(c.Counted().AS("visible"))).AS("v")
	visible := BoolColumn("visible").From(vis)
	state := CASE().
		WHEN(visible).THEN(NewEnumValue(stateVisible)).
		WHEN(c.Gone()).THEN(NewEnumValue(stateDeleted)).
		ELSE(NewEnumValue(stateHidden))
	hasVisibleReply := EXISTS(SELECT(r.T.ID).FROM(r.T).WHERE(r.Where(r.T.ParentID.EQ(c.T.ID), r.Counted())))
	return SELECT(
		c.T.ID.AS("id"), c.T.PostID.AS("post_id"), c.T.ParentID.AS("parent_id"), c.T.ReplyToID.AS("reply_to_id"),
		state.AS("state"),
		masked(visible, c.T.ActorKind, String("")).AS("actor_kind"),
		masked(visible, c.T.ActorID, Int64(0)).AS("actor_id"),
		masked(visible, c.T.Body, String("")).AS("body"),
		c.T.CreatedAt.AS("created_at"), c.T.UpdatedAt.AS("updated_at"), s.LikeCount.AS("like_count"), s.ReplyCount.AS("reply_count"),
	).
		FROM(c.T.INNER_JOIN(s, s.CommentID.EQ(c.T.ID)).CROSS_JOIN(vis)).
		WHERE(c.Where(
			c.T.PostID.EQ(UUID(postID)),
			c.T.ParentID.IS_NULL(),
			container,
			visible.OR(hasVisibleReply),
			afterComment(c.T.CreatedAt, c.T.ID, pg),
		)).
		ORDER_BY(c.T.CreatedAt.ASC(), c.T.ID.ASC()).
		LIMIT(pg.Limit)
}

func postContainer(src visibility.Posts, postID uuid.UUID) BoolExpression {
	return EXISTS(SELECT(src.T.ID).FROM(src.T).WHERE(src.Where(src.T.ID.EQ(UUID(postID)))))
}

// ListTopLevelComments 是公開的頂層留言：貼文要公開可讀。
func ListTopLevelComments(postID uuid.UUID, now time.Time, v visibility.Viewer, pg CommentPage) SelectStatement {
	return topLevelComments(postID, v, pg, postContainer(visibility.ReadablePosts("cp", now, v), postID))
}

// ListOwnerTopLevelComments 是店主讀自己的貼文的頂層留言：貼文不必公開（草稿、下架與店家被停權時照樣讀得到），其餘同 ListTopLevelComments。
// 店主以店家身分讀，不套個人封鎖（沒有觀看者參數）。
func ListOwnerTopLevelComments(postID uuid.UUID, storeID int64, pg CommentPage) SelectStatement {
	return topLevelComments(postID, visibility.Anonymous, pg, postContainer(visibility.OwnerPosts("cp", storeID), postID))
}

// replies 是頂層留言的回覆（父留言可以是佔位項目，但不能被觀看者的封鎖排除：直接帶被封鎖者的頂層 ID 讀不到回覆）：每一列自己可見
// 而且作者沒被排除；容器 container 由呼叫方給（對 post 的別名 p）。
func replies(parentID uuid.UUID, v visibility.Viewer, pg CommentPage, post visibility.Posts) SelectStatement {
	c := visibility.CommentsOf("c", v)
	s := table.CommentStats.AS("s")
	return SELECT(
		c.T.ID.AS("id"), c.T.PostID.AS("post_id"), c.T.ParentID.AS("parent_id"), c.T.ReplyToID.AS("reply_to_id"),
		NewEnumValue(stateVisible).AS("state"),
		c.T.ActorKind.AS("actor_kind"), c.T.ActorID.AS("actor_id"), c.T.Body.AS("body"),
		c.T.CreatedAt.AS("created_at"), c.T.UpdatedAt.AS("updated_at"), s.LikeCount.AS("like_count"), s.ReplyCount.AS("reply_count"),
	).
		FROM(c.T.INNER_JOIN(post.T, post.T.ID.EQ(c.T.PostID)).INNER_JOIN(s, s.CommentID.EQ(c.T.ID))).
		WHERE(c.Where(
			c.T.ParentID.EQ(UUID(parentID)),
			c.Counted(),
			post.Where(),
			c.Admits(UUID(parentID)),
			afterComment(c.T.CreatedAt, c.T.ID, pg),
		)).
		ORDER_BY(c.T.CreatedAt.ASC(), c.T.ID.ASC()).
		LIMIT(pg.Limit)
}

// ListReplies 是公開的回覆：貼文要公開可讀。
func ListReplies(parentID uuid.UUID, now time.Time, v visibility.Viewer, pg CommentPage) SelectStatement {
	return replies(parentID, v, pg, visibility.ReadablePosts("p", now, v))
}

// ListOwnerReplies 是店主讀自己的貼文下的回覆：貼文不必公開，其餘同 ListReplies；不套個人封鎖。
func ListOwnerReplies(parentID uuid.UUID, storeID int64, pg CommentPage) SelectStatement {
	return replies(parentID, visibility.Anonymous, pg, visibility.OwnerPosts("p", storeID))
}

// StoreIDRow 是只回店家 ID 的 query 的列。
type StoreIDRow struct {
	StoreID int64 `db:"store_id"`
}

// ReadableCommentPost 回留言對觀看者是不是公開可讀（讚只對讀得到的內容按）：回所屬貼文的店家；不可讀沒有列。
func ReadableCommentPost(id uuid.UUID, now time.Time, v visibility.Viewer) SelectStatement {
	c := visibility.CommentsOf("c", v)
	p := visibility.ReadablePosts("p", now, v)
	return SELECT(p.T.StoreID.AS("store_id")).
		FROM(c.T.INNER_JOIN(p.T, p.T.ID.EQ(c.T.PostID))).
		WHERE(c.Where(c.T.ID.EQ(UUID(id)), c.Counted(), p.Where(), c.ParentAdmitted()))
}

// ReplyTargetIsReadable 回回覆的 reply_to 是不是可以回覆的對象：同篇、同一串（頂層留言本身，或它底下的回覆）、觀看者看得到的留言
// （作者被封鎖排除的比照已刪除），而且該串的頂層沒被觀看者的封鎖排除（同 replies 的父列條件）。
// 同一串由 DB 的複合 FK 保證（comments_reply_to_fk），這裡先查是為了回有意義的錯誤。
func ReplyTargetIsReadable(id, postID, threadID uuid.UUID, v visibility.Viewer) SelectStatement {
	c := visibility.CommentsOf("c", v)
	return SELECT(EXISTS(SELECT(c.T.ID).FROM(c.T).WHERE(
		c.Where(c.T.ID.EQ(UUID(id)), c.T.PostID.EQ(UUID(postID)), c.T.ThreadID.EQ(UUID(threadID)), c.Counted(), c.Admits(UUID(threadID))),
	)).AS("value"))
}

// CommentIsHidden 回留言此刻有沒有未還原的隱藏。
func CommentIsHidden(id uuid.UUID) SelectStatement {
	return SELECT(visibility.CommentHiddenBy(UUID(id)).AS("value"))
}
