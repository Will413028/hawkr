-- social 的函式與 trigger（單體規則 6：Atlas 社群版略過函式與 trigger，這支 migration 是手寫的，`atlas migrate new`）；內容與 objects.sql 相同
-- （TestSchemaFileMatchesMigrations），註解見 objects.sql。
-- social 的函式與 trigger（單體規則 6：Atlas 只讀 schema.sql，這些以手寫的 migration 建立；schema.sql 加本檔與 migration
-- 套用後的結果相同，TestSchemaFileMatchesMigrations）。
--
-- 這裡沒有 view：公開可讀、可見的規則（貼文已發布、店家可曝光、留言計入、觀看者規則）在 Go 的 internal/visibility，讀取的 query 用 go-jet 在上面組成
-- （internal/reads）。「計入」在資料庫端求值，結果存成內容列的旗標 counted（下面的 reevaluate_*），讀取只看旗標。
--
-- 這裡的物件只能參照 social 自己的表，函式一律 BEGIN ATOMIC（觸發函式 RETURNS trigger 只能用 plpgsql，是唯一的例外）：migration 單獨套用在
-- 空的 database 上也要成功（TestEveryModuleStaysInsideItsOwnSchema）。別的模組擁有的事實（買家停用）由 social 的訂閱者複製進自己的表。

-- 計數的 trigger（Lemmy 型）：每個寫入在同一個 transaction 內讓受影響的內容重新求值「是否計入」，求值結果與儲存的旗標不同才加減 stats。
-- 這是計數唯一的維護路徑；寫入端不直接改 stats。stats 的 CHECK (>= 0) 讓維護出錯時寫入失敗，而不是靜默漂移。

-- 貼文建立時建 stats 列。
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

-- 標籤列的 published_at 是貼文 published_at 的複本（schema.sql 的 post_hashtags）：標籤列寫入時帶入貼文當下的值；貼文第一次發布時寫進它所有的標籤列；
-- 貼文的 published_at 一旦有值就不能改（Publish 固定在第一次發布），所以複本不需要再同步，這個 trigger 擋掉改動而不是同步它。
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

-- 「是否計入」的唯一定義，與維護它的方式：一則內容計入貼文的留言數與父項的回覆數，若且唯若它的旗標 counted 為真；旗標只由 reevaluate_* 設，
-- 讀取直接用旗標，重算把旗標加總。定義的輸入全是 social 自己的表：內容的 deleted_at、hides、inactive_authors（買家刪帳的事實由 social 的訂閱者
-- 在同一個 transaction 寫進這張表，再讓該作者的內容重新求值）。reevaluate_* 對一批內容（ids）求值，與旗標不同才更新旗標並同一個 transaction
-- 加減計數，回傳旗標改變的筆數；冪等：求值相同就什麼都不做。
-- 鎖的順序固定，不依呼叫方（與寫入端相同，counts-brief.md §4.6）：內容列（id 由小到大）→ 貼文的 stats（id 由小到大）→ 父項的 stats（id 由小到大）。
-- 先鎖再求值：函式是 VOLATILE，body 的每個語句各取新的 snapshot（READ COMMITTED 下看得到別的 transaction 已提交的改動），求值是鎖之後的新語句，
-- 所以兩個並行的求值互相等待、後到的看得到先到的結果，沒有人需要在寫入前自己鎖內容列。這些函式不能 STABLE：STABLE 的函式沿用呼叫者的 snapshot，
-- 看不到鎖等待之後別人提交的改動。逐列寫入再靠 trigger 求值的批次寫入者（清除 job）在第一個寫入之前用 stats.Service.Lock 依同一個順序鎖好 stats；一次集合呼叫求值的（inactive_authors 的訂閱者）由函式自己依這個順序鎖整批。

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

-- 留言：建立時建自己的 stats 列並求值；刪除狀態改變時重新求值（只有 deleted_at 影響計入，旗標自己的更新與內文編輯不觸發，也就不會遞迴）。
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

-- 討論區留言：同留言（頂層的討論區留言沒有貼文那樣的容器計數，只加減父項的回覆數）。
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

-- 隱藏與還原：內容自己沒有變，「有沒有未還原的隱藏」變了，目標內容重新求值。隱藏的目標不能改（要改就是另一筆）。求值先鎖內容列，
-- 與刪除（同樣先鎖內容列）序列化，所以隱藏與刪除並行時兩邊的 trigger 都求得到對方已提交的結果。
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

-- 讚與分享：集合是存在的列，依實際新增或刪除的列加減（並發雙擊只有一列，只加一）。
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

-- 瀏覽：去重表新增一列才把瀏覽數加一（只增的事件計數）。去重表由週期 job 清掉舊的列，刪除不扣瀏覽數，所以只有 INSERT 的 trigger。
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
