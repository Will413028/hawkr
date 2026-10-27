-- 計數重算的鎖與分頁（counts-brief.md §4 第 8 點）：先 SELECT ... FOR UPDATE 鎖 stats 列，再用 internal/reads 的批次重算語句（go-jet，「計入」的
-- 定義在 internal/visibility）計數覆寫，不蓋掉並行的加減。瀏覽數是只增的事件計數，沒有可重算的來源，不在這裡。
-- 單一對象的 Lock* 給清除 job 在多個對象之間固定鎖順序用；全量重算用 *Range（一批一個語句，依 ID 由小到大鎖）。

-- name: LockPostStats :one
SELECT * FROM social.post_stats WHERE post_id = sqlc.arg(post_id) FOR UPDATE;

-- name: LockCommentStats :one
SELECT * FROM social.comment_stats WHERE comment_id = sqlc.arg(comment_id) FOR UPDATE;

-- name: LockForumEntryStats :one
SELECT * FROM social.forum_entry_stats WHERE forum_entry_id = sqlc.arg(forum_entry_id) FOR UPDATE;

-- name: LockPostStatsRange :exec
-- 鎖住 (after_id, upper_id] 範圍內的貼文 stats，依 post_id 由小到大（批次重算的第一步）。
SELECT post_id FROM social.post_stats WHERE post_id > sqlc.arg(after_id)::uuid AND post_id <= sqlc.arg(upper_id)::uuid ORDER BY post_id FOR UPDATE;

-- name: LockCommentStatsRange :exec
SELECT comment_id FROM social.comment_stats WHERE comment_id > sqlc.arg(after_id)::uuid AND comment_id <= sqlc.arg(upper_id)::uuid ORDER BY comment_id FOR UPDATE;

-- name: LockForumEntryStatsRange :exec
SELECT forum_entry_id FROM social.forum_entry_stats WHERE forum_entry_id > sqlc.arg(after_id)::uuid AND forum_entry_id <= sqlc.arg(upper_id)::uuid ORDER BY forum_entry_id FOR UPDATE;

-- name: PostStatsPage :many
-- 全量重算的分頁：依 post_id 的 keyset。
SELECT post_id FROM social.post_stats WHERE post_id > sqlc.arg(after_id)::uuid ORDER BY post_id LIMIT sqlc.arg(max_rows);

-- name: CommentStatsPage :many
SELECT comment_id FROM social.comment_stats WHERE comment_id > sqlc.arg(after_id)::uuid ORDER BY comment_id LIMIT sqlc.arg(max_rows);

-- name: ForumEntryStatsPage :many
SELECT forum_entry_id FROM social.forum_entry_stats WHERE forum_entry_id > sqlc.arg(after_id)::uuid ORDER BY forum_entry_id LIMIT sqlc.arg(max_rows);

-- name: ReevaluateComments :one
-- 讓這批留言重新求值「是否計入」（db/objects.sql 的 reevaluate_comments）：鎖與求值都在函式裡，回傳旗標改變的筆數。
SELECT social.reevaluate_comments(sqlc.arg(ids)::uuid[]);

-- name: ReevaluateForumEntries :one
SELECT social.reevaluate_forum_entries(sqlc.arg(ids)::uuid[]);
