package reads

import (
	"time"
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// OwnerRow 是給 moderation 驗證內容歸屬的一列：作者、所屬店家、內容還在不在、是不是公開可讀。貼文沒有作者欄位（作者是店家）。
type OwnerRow struct {
	ActorKind string `db:"actor_kind"`
	ActorID   int64  `db:"actor_id"`
	StoreID   int64  `db:"store_id"`
	ExistsNow bool   `db:"exists_now"`
	Readable  bool   `db:"readable"`
}

// PostOwnerRow 是貼文的歸屬。
type PostOwnerRow struct {
	StoreID   int64 `db:"store_id"`
	ExistsNow bool  `db:"exists_now"`
	Readable  bool  `db:"readable"`
}

// 歸屬的讀取是治理用的（moderation 驗證內容、sanction 的隱藏與還原），以匿名建入口：不套任何觀看者的個人封鎖。

// GetCommentOwner 是留言的歸屬：readable 是留言可見且所屬貼文公開可讀。
func GetCommentOwner(id uuid.UUID, now time.Time) SelectStatement {
	c := visibility.CommentsOf("c", visibility.Anonymous)
	p := visibility.ReadablePosts("p", now, visibility.Anonymous)
	return SELECT(
		c.T.ActorKind.AS("actor_kind"), c.T.ActorID.AS("actor_id"), p.T.StoreID.AS("store_id"),
		c.T.DeletedAt.IS_NULL().AS("exists_now"),
		c.Counted().AND(p.Condition()).AS("readable"),
	).
		FROM(c.T.INNER_JOIN(p.T, p.T.ID.EQ(c.T.PostID))).
		WHERE(c.T.ID.EQ(UUID(id)))
}

// GetForumEntryOwner 是討論區留言的歸屬：readable 是留言可見且店家可曝光。
func GetForumEntryOwner(id uuid.UUID, now time.Time) SelectStatement {
	e := visibility.ForumEntriesOf("e", visibility.Anonymous)
	return SELECT(
		e.T.ActorKind.AS("actor_kind"), e.T.ActorID.AS("actor_id"), e.T.StoreID.AS("store_id"),
		e.T.DeletedAt.IS_NULL().AS("exists_now"),
		e.Counted().AND(visibility.StoreExposable(now, e.T.StoreID)).AS("readable"),
	).
		FROM(e.T).
		WHERE(e.T.ID.EQ(UUID(id)))
}

// GetPostOwner 是貼文的歸屬：readable 是貼文公開可讀。
func GetPostOwner(id uuid.UUID, now time.Time) SelectStatement {
	p := visibility.ReadablePosts("p", now, visibility.Anonymous)
	return SELECT(
		p.T.StoreID.AS("store_id"),
		p.T.DeletedAt.IS_NULL().AS("exists_now"),
		p.Condition().AS("readable"),
	).
		FROM(p.T).
		WHERE(p.T.ID.EQ(UUID(id)))
}
