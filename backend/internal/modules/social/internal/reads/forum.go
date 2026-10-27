package reads

import (
	"time"
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/jet/table"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// EntryRow 是討論區留言列表的一列；佔位項目同 CommentRow。
type EntryRow struct {
	ID         uuid.UUID
	StoreID    int64
	ParentID   *uuid.UUID
	ReplyToID  *uuid.UUID
	State      string
	ActorKind  string
	ActorID    int64
	Body       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ReplyCount int64
}

// ListTopLevelForumEntries 是店家討論區的頂層留言，含佔位項目；店家要可曝光而且觀看者沒有封鎖這家店（對店家鍵只檢查一次）。
// 規則同 topLevelComments：被封鎖排除的作者，頂層整則不出現，回覆不進「有可見回覆」的判斷。
func ListTopLevelForumEntries(storeID int64, now time.Time, v visibility.Viewer, pg CommentPage) SelectStatement {
	c := visibility.ForumEntriesOf("c", v)
	r := visibility.ForumEntriesOf("r", v)
	s := table.ForumEntryStats.AS("s")
	vis := LATERAL(SELECT(c.Counted().AS("visible"))).AS("v")
	visible := BoolColumn("visible").From(vis)
	state := CASE().
		WHEN(visible).THEN(NewEnumValue(stateVisible)).
		WHEN(c.Gone()).THEN(NewEnumValue(stateDeleted)).
		ELSE(NewEnumValue(stateHidden))
	hasVisibleReply := EXISTS(SELECT(r.T.ID).FROM(r.T).WHERE(r.Where(r.T.ParentID.EQ(c.T.ID), r.Counted())))
	return SELECT(
		c.T.ID.AS("id"), c.T.StoreID.AS("store_id"), c.T.ParentID.AS("parent_id"), c.T.ReplyToID.AS("reply_to_id"),
		state.AS("state"),
		masked(visible, c.T.ActorKind, String("")).AS("actor_kind"),
		masked(visible, c.T.ActorID, Int64(0)).AS("actor_id"),
		masked(visible, c.T.Body, String("")).AS("body"),
		c.T.CreatedAt.AS("created_at"), c.T.UpdatedAt.AS("updated_at"), s.ReplyCount.AS("reply_count"),
	).
		FROM(c.T.INNER_JOIN(s, s.ForumEntryID.EQ(c.T.ID)).CROSS_JOIN(vis)).
		WHERE(c.Where(
			c.T.StoreID.EQ(Int64(storeID)),
			c.T.ParentID.IS_NULL(),
			visibility.ReadableForum(now, v, Int64(storeID)),
			visible.OR(hasVisibleReply),
			afterComment(c.T.CreatedAt, c.T.ID, pg),
		)).
		ORDER_BY(c.T.CreatedAt.ASC(), c.T.ID.ASC()).
		LIMIT(pg.Limit)
}

// ListForumReplies 是頂層討論區留言的回覆（父留言可以是佔位項目，但不能被觀看者的封鎖排除）：自己可見、作者沒被排除、店家可曝光而且觀看者沒有封鎖這家店。
func ListForumReplies(parentID uuid.UUID, now time.Time, v visibility.Viewer, pg CommentPage) SelectStatement {
	c := visibility.ForumEntriesOf("c", v)
	s := table.ForumEntryStats.AS("s")
	return SELECT(
		c.T.ID.AS("id"), c.T.StoreID.AS("store_id"), c.T.ParentID.AS("parent_id"), c.T.ReplyToID.AS("reply_to_id"),
		NewEnumValue(stateVisible).AS("state"),
		c.T.ActorKind.AS("actor_kind"), c.T.ActorID.AS("actor_id"), c.T.Body.AS("body"),
		c.T.CreatedAt.AS("created_at"), c.T.UpdatedAt.AS("updated_at"), s.ReplyCount.AS("reply_count"),
	).
		FROM(c.T.INNER_JOIN(s, s.ForumEntryID.EQ(c.T.ID))).
		WHERE(c.Where(
			c.T.ParentID.EQ(UUID(parentID)),
			c.Counted(),
			visibility.ReadableForum(now, v, c.T.StoreID),
			c.Admits(UUID(parentID)),
			afterComment(c.T.CreatedAt, c.T.ID, pg),
		)).
		ORDER_BY(c.T.CreatedAt.ASC(), c.T.ID.ASC()).
		LIMIT(pg.Limit)
}

// ForumReplyTargetIsReadable 同 ReplyTargetIsReadable（含該串頂層沒被排除）：同一個討論區、同一串、觀看者看得到的留言；同一串由 forum_entries_reply_to_fk 保證。
func ForumReplyTargetIsReadable(id uuid.UUID, storeID int64, threadID uuid.UUID, v visibility.Viewer) SelectStatement {
	e := visibility.ForumEntriesOf("e", v)
	return SELECT(EXISTS(SELECT(e.T.ID).FROM(e.T).WHERE(
		e.Where(e.T.ID.EQ(UUID(id)), e.T.StoreID.EQ(Int64(storeID)), e.T.ThreadID.EQ(UUID(threadID)), e.Counted(), e.Admits(UUID(threadID))),
	)).AS("value"))
}

// ForumEntryIsHidden 回討論區留言此刻有沒有未還原的隱藏。
func ForumEntryIsHidden(id uuid.UUID) SelectStatement {
	return SELECT(visibility.ForumEntryHiddenBy(UUID(id)).AS("value"))
}

// ForumIsReadable 回店家的討論區對觀看者此刻可不可以讀：店家可曝光，而且觀看者沒有封鎖這家店。
func ForumIsReadable(storeID int64, now time.Time, v visibility.Viewer) SelectStatement {
	return SELECT(visibility.ReadableForum(now, v, Int64(storeID)).AS("value"))
}
