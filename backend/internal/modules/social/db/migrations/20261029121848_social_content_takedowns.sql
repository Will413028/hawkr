-- Set comment to column: "counted" on table: "comments"
COMMENT ON COLUMN "social"."comments"."counted" IS '此刻是否計入貼文的留言數與父留言的回覆數（沒有刪除、沒有未還原的隱藏、沒有內容下架、作者不在 inactive_authors，在 social.reevaluate_comments 求值）；只由 social.reevaluate_comments 維護，讀取直接用它，貼文的 comment_count 與留言的 reply_count 是這個旗標的加總';
-- Set comment to column: "counted" on table: "forum_entries"
COMMENT ON COLUMN "social"."forum_entries"."counted" IS '此刻是否計入父討論區留言的回覆數（沒有刪除、沒有未還原的隱藏、沒有內容下架、作者不在 inactive_authors，在 social.reevaluate_forum_entries 求值）；只由 social.reevaluate_forum_entries 維護，讀取直接用它，父項的 reply_count 是這個旗標的加總';
-- Create "content_takedowns" table
CREATE TABLE "social"."content_takedowns" (
  "measure_id" uuid NOT NULL,
  "post_id" uuid NULL,
  "comment_id" uuid NULL,
  "forum_entry_id" uuid NULL,
  "created_at" timestamptz NOT NULL,
  CONSTRAINT "content_takedowns_comment_id_fkey" FOREIGN KEY ("comment_id") REFERENCES "social"."comments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "content_takedowns_forum_entry_id_fkey" FOREIGN KEY ("forum_entry_id") REFERENCES "social"."forum_entries" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "content_takedowns_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "content_takedowns_target_check" CHECK (num_nonnulls(post_id, comment_id, forum_entry_id) = 1)
);
-- Create index "content_takedowns_comment" to table: "content_takedowns"
CREATE INDEX "content_takedowns_comment" ON "social"."content_takedowns" ("comment_id") WHERE (comment_id IS NOT NULL);
-- Create index "content_takedowns_forum_entry" to table: "content_takedowns"
CREATE INDEX "content_takedowns_forum_entry" ON "social"."content_takedowns" ("forum_entry_id") WHERE (forum_entry_id IS NOT NULL);
-- Create index "content_takedowns_measure_comment" to table: "content_takedowns"
CREATE UNIQUE INDEX "content_takedowns_measure_comment" ON "social"."content_takedowns" ("measure_id", "comment_id") WHERE (comment_id IS NOT NULL);
-- Create index "content_takedowns_measure_forum_entry" to table: "content_takedowns"
CREATE UNIQUE INDEX "content_takedowns_measure_forum_entry" ON "social"."content_takedowns" ("measure_id", "forum_entry_id") WHERE (forum_entry_id IS NOT NULL);
-- Create index "content_takedowns_measure_post" to table: "content_takedowns"
CREATE UNIQUE INDEX "content_takedowns_measure_post" ON "social"."content_takedowns" ("measure_id", "post_id") WHERE (post_id IS NOT NULL);
-- Create index "content_takedowns_post" to table: "content_takedowns"
CREATE INDEX "content_takedowns_post" ON "social"."content_takedowns" ("post_id") WHERE (post_id IS NOT NULL);
-- 重新求值的定義加上內容下架（Atlas 社群版略過函式，手寫；與 objects.sql 相同）
CREATE OR REPLACE FUNCTION social.reevaluate_comments(ids UUID[]) RETURNS INTEGER
LANGUAGE sql
BEGIN ATOMIC
    SELECT 1 FROM social.comments WHERE id = ANY(ids) ORDER BY id FOR NO KEY UPDATE;
    SELECT 1 FROM social.post_stats WHERE post_id IN (SELECT post_id FROM social.comments WHERE id = ANY(ids)) ORDER BY post_id FOR UPDATE;
    SELECT 1 FROM social.comment_stats
        WHERE comment_id IN (SELECT parent_id FROM social.comments WHERE id = ANY(ids) AND parent_id IS NOT NULL) ORDER BY comment_id FOR UPDATE;
    WITH ev AS (
        SELECT c.id, c.counted AS was,
               (c.deleted_at IS NULL
                AND NOT EXISTS (SELECT 1 FROM social.hides AS h WHERE h.comment_id = c.id AND h.restored_at IS NULL)
                AND NOT EXISTS (SELECT 1 FROM social.content_takedowns AS t WHERE t.comment_id = c.id)
                AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM social.inactive_authors AS ia WHERE ia.buyer_id = c.actor_id))
               ) AS now_counted
          FROM social.comments AS c
         WHERE c.id = ANY(ids)
    ),
    ch AS (
        UPDATE social.comments AS c SET counted = ev.now_counted
          FROM ev
         WHERE c.id = ev.id AND ev.was <> ev.now_counted
        RETURNING c.post_id, c.parent_id, ev.now_counted::INTEGER - ev.was::INTEGER AS delta
    ),
    ps AS (
        UPDATE social.post_stats AS s SET comment_count = s.comment_count + d.delta
          FROM (SELECT post_id, sum(delta) AS delta FROM ch GROUP BY post_id) AS d
         WHERE s.post_id = d.post_id
        RETURNING 1
    ),
    cs AS (
        UPDATE social.comment_stats AS s SET reply_count = s.reply_count + d.delta
          FROM (SELECT parent_id, sum(delta) AS delta FROM ch WHERE parent_id IS NOT NULL GROUP BY parent_id) AS d
         WHERE s.comment_id = d.parent_id
        RETURNING 1
    )
    SELECT count(*)::INTEGER FROM ch;
END;

CREATE OR REPLACE FUNCTION social.reevaluate_forum_entries(ids UUID[]) RETURNS INTEGER
LANGUAGE sql
BEGIN ATOMIC
    SELECT 1 FROM social.forum_entries WHERE id = ANY(ids) ORDER BY id FOR NO KEY UPDATE;
    SELECT 1 FROM social.forum_entry_stats
        WHERE forum_entry_id IN (SELECT parent_id FROM social.forum_entries WHERE id = ANY(ids) AND parent_id IS NOT NULL) ORDER BY forum_entry_id FOR UPDATE;
    WITH ev AS (
        SELECT c.id, c.counted AS was,
               (c.deleted_at IS NULL
                AND NOT EXISTS (SELECT 1 FROM social.hides AS h WHERE h.forum_entry_id = c.id AND h.restored_at IS NULL)
                AND NOT EXISTS (SELECT 1 FROM social.content_takedowns AS t WHERE t.forum_entry_id = c.id)
                AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM social.inactive_authors AS ia WHERE ia.buyer_id = c.actor_id))
               ) AS now_counted
          FROM social.forum_entries AS c
         WHERE c.id = ANY(ids)
    ),
    ch AS (
        UPDATE social.forum_entries AS c SET counted = ev.now_counted
          FROM ev
         WHERE c.id = ev.id AND ev.was <> ev.now_counted
        RETURNING c.parent_id, ev.now_counted::INTEGER - ev.was::INTEGER AS delta
    ),
    cs AS (
        UPDATE social.forum_entry_stats AS s SET reply_count = s.reply_count + d.delta
          FROM (SELECT parent_id, sum(delta) AS delta FROM ch WHERE parent_id IS NOT NULL GROUP BY parent_id) AS d
         WHERE s.forum_entry_id = d.parent_id
        RETURNING 1
    )
    SELECT count(*)::INTEGER FROM ch;
END;
