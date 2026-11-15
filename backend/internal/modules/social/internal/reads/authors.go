package reads

import (
	"time"

	. "github.com/go-jet/jet/v2/postgres"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// AuthorRow 是「這位讀取者讀得到的一則內容」的作者與所屬店家（A7-2e：從留言、討論區封鎖作者）。
type AuthorRow struct {
	ActorKind string `db:"actor_kind"`
	ActorID   int64  `db:"actor_id"`
	StoreID   int64  `db:"store_id"`
}

// ReadableCommentAuthor 是觀看者（買家 buyerID，0 是未登入）讀得到的留言的作者：入口的觀看者規則（觀看者自己發起的封鎖不排除，讓重複封鎖仍解析得到作者；
// 對方發起的封鎖照樣排除，父列條件同樣套用）、計入（沒刪、沒隱藏）、貼文公開可讀、父留言沒被排除（同 ReadableCommentPost）；
// 讀不到沒有列。
func ReadableCommentAuthor(id uuid.UUID, now time.Time, buyerID int64) SelectStatement {
	v := visibility.Viewer{BuyerID: buyerID, SkipOwnBlocks: true}
	c := visibility.CommentsOf("c", v)
	p := visibility.ReadablePosts("p", now, v)
	return SELECT(c.T.ActorKind.AS("actor_kind"), c.T.ActorID.AS("actor_id"), p.T.StoreID.AS("store_id")).
		FROM(c.T.INNER_JOIN(p.T, p.T.ID.EQ(c.T.PostID))).
		WHERE(c.Where(c.T.ID.EQ(UUID(id)), c.Counted(), p.Where(), c.ParentAdmitted()))
}

// OwnerCommentAuthor 是店主讀自己的貼文下的留言的作者：貼文不必公開，不套個人封鎖（同 ListOwnerReplies），留言要計入。
func OwnerCommentAuthor(id uuid.UUID, storeID int64) SelectStatement {
	c := visibility.CommentsOf("c", visibility.Anonymous)
	p := visibility.OwnerPosts("p", storeID)
	return SELECT(c.T.ActorKind.AS("actor_kind"), c.T.ActorID.AS("actor_id"), p.T.StoreID.AS("store_id")).
		FROM(c.T.INNER_JOIN(p.T, p.T.ID.EQ(c.T.PostID))).
		WHERE(c.Where(c.T.ID.EQ(UUID(id)), c.Counted(), p.Where()))
}

// ReadableForumEntryAuthor 是觀看者讀得到的討論區留言的作者：入口的觀看者規則、計入、討論區可讀（店家可曝光、沒被觀看者封鎖）、父留言沒被排除。
func ReadableForumEntryAuthor(id uuid.UUID, now time.Time, buyerID int64) SelectStatement {
	v := visibility.Viewer{BuyerID: buyerID, SkipOwnBlocks: true}
	e := visibility.ForumEntriesOf("e", v)
	return SELECT(e.T.ActorKind.AS("actor_kind"), e.T.ActorID.AS("actor_id"), e.T.StoreID.AS("store_id")).
		FROM(e.T).
		WHERE(e.Where(e.T.ID.EQ(UUID(id)), e.Counted(), visibility.ReadableForum(now, v, e.T.StoreID), e.ParentAdmitted()))
}

// OwnerForumEntryAuthor 是店主讀自己討論區的留言的作者：不套個人封鎖，留言要計入。
func OwnerForumEntryAuthor(id uuid.UUID, storeID int64) SelectStatement {
	e := visibility.ForumEntriesOf("e", visibility.Anonymous)
	return SELECT(e.T.ActorKind.AS("actor_kind"), e.T.ActorID.AS("actor_id"), e.T.StoreID.AS("store_id")).
		FROM(e.T).
		WHERE(e.Where(e.T.ID.EQ(UUID(id)), e.T.StoreID.EQ(Int64(storeID)), e.Counted()))
}
