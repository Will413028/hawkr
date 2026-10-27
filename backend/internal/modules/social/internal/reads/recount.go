package reads

import (
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/jet/table"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// 計數的批次重算（counts-brief.md §4 第 8 點，DR-A1）：一批是 stats 主鍵的 keyset 範圍 (after, upper]。呼叫方在同一個 transaction 內先依 ID 由小到大
// SELECT ... FOR UPDATE 鎖住這個範圍的 stats 列（LockPostStatsRange 等），再執行這裡的語句：一個語句用分組的 COUNT 算出範圍內每一列的新計數，
// 只覆寫與現值不同的列，RETURNING 舊值與新值（漂移報告）。先鎖再算：算數是鎖之後的新 statement（READ COMMITTED 下看得到別的 transaction 已提交的
// 加減），不蓋掉並行的寫入。計數是旗標的加總：「計入」用 visibility 的 Counted（儲存的旗標 counted），和讀取同一個來源，不重新推導；旗標本身由 reevaluate_*（trigger、買家刪帳的訂閱者、對帳的第一步）維護。
// 重算期間買家被停用也不會讓計數與旗標不一致：停用的事實先經訂閱者在同一個 transaction 修好旗標，再有人改計數（R1）。瀏覽數是只增的事件計數，沒有可重算的來源，不在這裡。

// PostDriftRow 是一篇貼文的計數被覆寫前後的值。
type PostDriftRow struct {
	ID         uuid.UUID `db:"id"`
	OldLike    int64     `db:"old_like"`
	OldComment int64     `db:"old_comment"`
	OldShare   int64     `db:"old_share"`
	NewLike    int64     `db:"new_like"`
	NewComment int64     `db:"new_comment"`
	NewShare   int64     `db:"new_share"`
}

// CommentDriftRow 是一則留言的計數被覆寫前後的值。
type CommentDriftRow struct {
	ID       uuid.UUID `db:"id"`
	OldLike  int64     `db:"old_like"`
	OldReply int64     `db:"old_reply"`
	NewLike  int64     `db:"new_like"`
	NewReply int64     `db:"new_reply"`
}

// EntryDriftRow 是一則討論區留言的回覆數被覆寫前後的值。
type EntryDriftRow struct {
	ID       uuid.UUID `db:"id"`
	OldReply int64     `db:"old_reply"`
	NewReply int64     `db:"new_reply"`
}

// within 是 keyset 範圍 (after, upper]。
func within(col ColumnString, after, upper uuid.UUID) BoolExpression {
	return col.GT(UUID(after)).AND(col.LT_EQ(UUID(upper)))
}

// grouped 是 "key, COUNT(*)" 依 key 分組的子查詢，key 與 n 兩欄供外層 LEFT JOIN。
func grouped(key ColumnString, from ReadableTable, where BoolExpression, alias string) (SelectTable, ColumnString, ColumnInteger) {
	t := SELECT(key.AS("key"), COUNT(STAR).AS("n")).FROM(from).WHERE(where).GROUP_BY(key).AsTable(alias)
	return t, StringColumn("key").From(t), IntegerColumn("n").From(t)
}

// RecountPostStatsBatch 重算 (after, upper] 範圍內貼文的讚、分享與留言數（含回覆），只覆寫不同的列並回傳前後值。
func RecountPostStatsBatch(after, upper uuid.UUID) UpdateStatement {
	st, ps := table.PostStats, table.PostStats.AS("ps")
	l, sh := table.PostLikes.AS("l"), table.PostShares.AS("sh")
	c := visibility.CommentsOf("c", visibility.Anonymous)
	lk, lkKey, lkN := grouped(l.PostID, l, within(l.PostID, after, upper), "lk")
	shs, shKey, shN := grouped(sh.PostID, sh, within(sh.PostID, after, upper), "shs")
	cc, ccKey, ccN := grouped(c.T.PostID, c.T, within(c.T.PostID, after, upper).AND(c.Counted()), "cc")
	v := SELECT(
		ps.PostID.AS("id"),
		ps.LikeCount.AS("old_like"), ps.CommentCount.AS("old_comment"), ps.ShareCount.AS("old_share"),
		COALESCE(lkN, Int(0)).AS("new_like"), COALESCE(ccN, Int(0)).AS("new_comment"), COALESCE(shN, Int(0)).AS("new_share"),
	).FROM(ps.
		LEFT_JOIN(lk, lkKey.EQ(ps.PostID)).
		LEFT_JOIN(shs, shKey.EQ(ps.PostID)).
		LEFT_JOIN(cc, ccKey.EQ(ps.PostID))).
		WHERE(within(ps.PostID, after, upper)).AsTable("v")
	vID := StringColumn("id").From(v)
	oldL, oldC, oldS := IntegerColumn("old_like").From(v), IntegerColumn("old_comment").From(v), IntegerColumn("old_share").From(v)
	newL, newC, newS := IntegerColumn("new_like").From(v), IntegerColumn("new_comment").From(v), IntegerColumn("new_share").From(v)
	return st.UPDATE(st.LikeCount, st.CommentCount, st.ShareCount).
		SET(newL, newC, newS).
		FROM(v).
		WHERE(st.PostID.EQ(vID).AND(oldL.NOT_EQ(newL).OR(oldC.NOT_EQ(newC)).OR(oldS.NOT_EQ(newS)))).
		RETURNING(
			st.PostID.AS("id"), oldL.AS("old_like"), oldC.AS("old_comment"), oldS.AS("old_share"),
			st.LikeCount.AS("new_like"), st.CommentCount.AS("new_comment"), st.ShareCount.AS("new_share"),
		)
}

// RecountCommentStatsBatch 重算 (after, upper] 範圍內留言的讚與回覆數。
func RecountCommentStatsBatch(after, upper uuid.UUID) UpdateStatement {
	st, cs := table.CommentStats, table.CommentStats.AS("cs")
	l := table.CommentLikes.AS("l")
	c := visibility.CommentsOf("c", visibility.Anonymous)
	lk, lkKey, lkN := grouped(l.CommentID, l, within(l.CommentID, after, upper), "lk")
	rp, rpKey, rpN := grouped(c.T.ParentID, c.T, within(c.T.ParentID, after, upper).AND(c.Counted()), "rp")
	v := SELECT(
		cs.CommentID.AS("id"),
		cs.LikeCount.AS("old_like"), cs.ReplyCount.AS("old_reply"),
		COALESCE(lkN, Int(0)).AS("new_like"), COALESCE(rpN, Int(0)).AS("new_reply"),
	).FROM(cs.
		LEFT_JOIN(lk, lkKey.EQ(cs.CommentID)).
		LEFT_JOIN(rp, rpKey.EQ(cs.CommentID))).
		WHERE(within(cs.CommentID, after, upper)).AsTable("v")
	vID := StringColumn("id").From(v)
	oldL, oldR := IntegerColumn("old_like").From(v), IntegerColumn("old_reply").From(v)
	newL, newR := IntegerColumn("new_like").From(v), IntegerColumn("new_reply").From(v)
	return st.UPDATE(st.LikeCount, st.ReplyCount).
		SET(newL, newR).
		FROM(v).
		WHERE(st.CommentID.EQ(vID).AND(oldL.NOT_EQ(newL).OR(oldR.NOT_EQ(newR)))).
		RETURNING(
			st.CommentID.AS("id"), oldL.AS("old_like"), oldR.AS("old_reply"),
			st.LikeCount.AS("new_like"), st.ReplyCount.AS("new_reply"),
		)
}

// RecountForumEntryStatsBatch 重算 (after, upper] 範圍內討論區留言的回覆數。
func RecountForumEntryStatsBatch(after, upper uuid.UUID) UpdateStatement {
	st, es := table.ForumEntryStats, table.ForumEntryStats.AS("es")
	e := visibility.ForumEntriesOf("e", visibility.Anonymous)
	rp, rpKey, rpN := grouped(e.T.ParentID, e.T, within(e.T.ParentID, after, upper).AND(e.Counted()), "rp")
	v := SELECT(
		es.ForumEntryID.AS("id"), es.ReplyCount.AS("old_reply"), COALESCE(rpN, Int(0)).AS("new_reply"),
	).FROM(es.LEFT_JOIN(rp, rpKey.EQ(es.ForumEntryID))).
		WHERE(within(es.ForumEntryID, after, upper)).AsTable("v")
	vID := StringColumn("id").From(v)
	oldR, newR := IntegerColumn("old_reply").From(v), IntegerColumn("new_reply").From(v)
	return st.UPDATE(st.ReplyCount).
		SET(newR).
		FROM(v).
		WHERE(st.ForumEntryID.EQ(vID).AND(oldR.NOT_EQ(newR))).
		RETURNING(st.ForumEntryID.AS("id"), oldR.AS("old_reply"), st.ReplyCount.AS("new_reply"))
}
