

CREATE FUNCTION social.posts_create_stats() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO social.post_stats (post_id) VALUES (NEW.id);
    RETURN NULL;
END;
$$;

CREATE TRIGGER posts_create_stats AFTER INSERT ON social.posts
    FOR EACH ROW EXECUTE FUNCTION social.posts_create_stats();

CREATE FUNCTION social.post_hashtags_copy_published_at() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    SELECT published_at INTO NEW.published_at FROM social.posts WHERE id = NEW.post_id;
    RETURN NEW;
END;
$$;

CREATE TRIGGER post_hashtags_copy_published_at BEFORE INSERT ON social.post_hashtags
    FOR EACH ROW EXECUTE FUNCTION social.post_hashtags_copy_published_at();

CREATE FUNCTION social.posts_published_at_once() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.published_at IS NOT NULL THEN
        RAISE EXCEPTION 'social.posts.published_at 發布後不能改（貼文 %）', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    UPDATE social.post_hashtags SET published_at = NEW.published_at WHERE post_id = NEW.id;
    RETURN NULL;
END;
$$;

CREATE TRIGGER posts_published_at_once AFTER UPDATE OF published_at ON social.posts
    FOR EACH ROW WHEN (OLD.published_at IS DISTINCT FROM NEW.published_at)
    EXECUTE FUNCTION social.posts_published_at_once();


CREATE FUNCTION social.reevaluate_comments(ids UUID[]) RETURNS INTEGER
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

CREATE FUNCTION social.reevaluate_forum_entries(ids UUID[]) RETURNS INTEGER
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

CREATE FUNCTION social.comments_after_insert() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO social.comment_stats (comment_id) VALUES (NEW.id);
    PERFORM social.reevaluate_comments(ARRAY[NEW.id]);
    RETURN NULL;
END;
$$;

CREATE TRIGGER comments_after_insert AFTER INSERT ON social.comments
    FOR EACH ROW EXECUTE FUNCTION social.comments_after_insert();

CREATE FUNCTION social.comments_after_delete_change() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM social.reevaluate_comments(ARRAY[NEW.id]);
    RETURN NULL;
END;
$$;

CREATE TRIGGER comments_after_delete_change AFTER UPDATE ON social.comments
    FOR EACH ROW WHEN (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
    EXECUTE FUNCTION social.comments_after_delete_change();

CREATE FUNCTION social.forum_entries_after_insert() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO social.forum_entry_stats (forum_entry_id) VALUES (NEW.id);
    PERFORM social.reevaluate_forum_entries(ARRAY[NEW.id]);
    RETURN NULL;
END;
$$;

CREATE TRIGGER forum_entries_after_insert AFTER INSERT ON social.forum_entries
    FOR EACH ROW EXECUTE FUNCTION social.forum_entries_after_insert();

CREATE FUNCTION social.forum_entries_after_delete_change() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM social.reevaluate_forum_entries(ARRAY[NEW.id]);
    RETURN NULL;
END;
$$;

CREATE TRIGGER forum_entries_after_delete_change AFTER UPDATE ON social.forum_entries
    FOR EACH ROW WHEN (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
    EXECUTE FUNCTION social.forum_entries_after_delete_change();

CREATE FUNCTION social.hides_after_change() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_comment UUID;
    target_entry UUID;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF OLD.comment_id IS DISTINCT FROM NEW.comment_id OR OLD.forum_entry_id IS DISTINCT FROM NEW.forum_entry_id THEN
            RAISE EXCEPTION 'social.hides：隱藏的目標不能改' USING ERRCODE = 'integrity_constraint_violation';
        END IF;
        IF OLD.restored_at IS NOT DISTINCT FROM NEW.restored_at THEN
            RETURN NULL;
        END IF;
    END IF;
    IF TG_OP = 'DELETE' THEN
        target_comment := OLD.comment_id;
        target_entry := OLD.forum_entry_id;
    ELSE
        target_comment := NEW.comment_id;
        target_entry := NEW.forum_entry_id;
    END IF;
    IF target_comment IS NOT NULL THEN
        PERFORM social.reevaluate_comments(ARRAY[target_comment]);
    ELSE
        PERFORM social.reevaluate_forum_entries(ARRAY[target_entry]);
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER hides_after_change AFTER INSERT OR UPDATE OR DELETE ON social.hides
    FOR EACH ROW EXECUTE FUNCTION social.hides_after_change();

CREATE FUNCTION social.comment_likes_maintain_stats() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE social.comment_stats SET like_count = like_count + 1 WHERE comment_id = NEW.comment_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE social.comment_stats SET like_count = like_count - 1 WHERE comment_id = OLD.comment_id;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER comment_likes_maintain_stats AFTER INSERT OR DELETE ON social.comment_likes
    FOR EACH ROW EXECUTE FUNCTION social.comment_likes_maintain_stats();

CREATE FUNCTION social.post_likes_maintain_stats() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE social.post_stats SET like_count = like_count + 1 WHERE post_id = NEW.post_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE social.post_stats SET like_count = like_count - 1 WHERE post_id = OLD.post_id;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER post_likes_maintain_stats AFTER INSERT OR DELETE ON social.post_likes
    FOR EACH ROW EXECUTE FUNCTION social.post_likes_maintain_stats();

CREATE FUNCTION social.post_shares_maintain_stats() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE social.post_stats SET share_count = share_count + 1 WHERE post_id = NEW.post_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE social.post_stats SET share_count = share_count - 1 WHERE post_id = OLD.post_id;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER post_shares_maintain_stats AFTER INSERT OR DELETE ON social.post_shares
    FOR EACH ROW EXECUTE FUNCTION social.post_shares_maintain_stats();

CREATE FUNCTION social.post_views_maintain_stats() RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE social.post_stats SET view_count = view_count + 1 WHERE post_id = NEW.post_id;
    RETURN NULL;
END;
$$;

CREATE TRIGGER post_views_maintain_stats AFTER INSERT ON social.post_views
    FOR EACH ROW EXECUTE FUNCTION social.post_views_maintain_stats();
