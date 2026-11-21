-- Modify "measure_contents" table
ALTER TABLE "seller"."measure_contents" DROP CONSTRAINT "measure_contents_content_kind_check", ADD CONSTRAINT "measure_contents_content_kind_check" CHECK (content_kind = ANY (ARRAY['POST'::text, 'COMMENT'::text, 'FORUM_ENTRY'::text, 'CHAT_MESSAGE'::text, 'REVIEW_REPLY'::text])), DROP CONSTRAINT "measure_contents_link_check", ADD CONSTRAINT "measure_contents_link_check" CHECK ((snapshot_state <> 'CAPTURED'::text) OR
CASE content_kind
    WHEN 'FORUM_ENTRY'::text THEN ((link_post_id IS NULL) AND (link_store_id IS NOT NULL) AND (link_store_id > 0) AND (link_thread_id IS NOT NULL))
    WHEN 'CHAT_MESSAGE'::text THEN ((link_post_id IS NULL) AND (link_store_id IS NULL) AND (link_thread_id IS NULL))
    WHEN 'REVIEW_REPLY'::text THEN ((link_post_id IS NULL) AND (link_store_id IS NULL) AND (link_thread_id IS NULL))
    ELSE ((link_post_id IS NOT NULL) AND (link_store_id IS NULL) AND (link_thread_id IS NULL))
END);
