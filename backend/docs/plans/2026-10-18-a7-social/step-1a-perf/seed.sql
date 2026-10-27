-- A7-1a §9.3 的播種腳本：把單體的 seller、buyer、social 三個 schema 灌到預估正式量（用法見 README.md）。
-- 前提：資料庫已套用單體的 migration（hawkr migrate），而且是用完即丟的 database，不是共用的開發 database。
-- 參數（psql -v 名稱=值，預設值是正式量的預估）：stores、buyers、posts、comments_per_post、likes_per_post。
-- 做法：session_replication_role = replica 關掉 FK 與 trigger（只有 CHECK 照常檢查，所以每一列仍然是合法的資料），
-- 最後由聚合算出 stats 表；分佈用 random()^k 做成偏斜（少數熱門店家、貼文、標籤）。

\set ON_ERROR_STOP on
\if :{?stores} \else \set stores 10000 \endif
\if :{?buyers} \else \set buyers 50000 \endif
\if :{?posts} \else \set posts 100000 \endif
\if :{?comments_per_post} \else \set comments_per_post 5 \endif
\if :{?likes_per_post} \else \set likes_per_post 10 \endif

-- 固定亂數種子：重跑得到相同的列數與分佈，量測前後才能比較（uuid 不受 setseed 影響，每次不同，不影響分佈）。
SELECT setseed(0.42);
SET session_replication_role = replica;
SET maintenance_work_mem = '256MB';

\echo '== seller：店家、KYC、PayUni、停權'
-- 3% 的店家沒有核准的 KYC（不可曝光）、2% 的帳號有生效中的停權、1% 正在隨帳號刪除。
INSERT INTO seller.stores (store_id, store_code, account_id, status, store_name, logo_key, logo_asset_id, town_code, contact_phone, contact_name, contact_mobile, description, created_at, updated_at)
OVERRIDING SYSTEM VALUE
SELECT g, g, 1000000 + g, CASE WHEN g % 100 = 3 THEN 'PENDING_DELETION' ELSE 'ACTIVE' END,
       '店家' || g, 'logo/' || g, 'asset-' || g, '10016010', '0212345678', '王小明', '0912345678', '店家介紹',
       now() - interval '800 days', now()
FROM generate_series(1, :stores) g;

INSERT INTO seller.kyc_applications (application_id, store_id, seller_type, status, created_at, updated_at, submitted_at, decided_at, submission_seq, pii_purged_at)
SELECT gen_random_uuid(), store_id, 'INDIVIDUAL', 'APPROVED', now() - interval '790 days', now(), now() - interval '790 days', now() - interval '789 days', 1, now()
FROM seller.stores WHERE store_id % 33 <> 0;

INSERT INTO seller.payuni_registrations (application_id, store_id, mer_id, created_at, updated_at, bind_status, bound_at, outcome, status_observed_at)
SELECT application_id, store_id, 'HAWK' || store_id, now() - interval '789 days', now(), 'BOUND', now() - interval '788 days', 'APPROVED', now() - interval '788 days'
FROM seller.kyc_applications;

INSERT INTO seller.measures (id, account_id, store_id, basis, kind, status, risk_ground, review_by, created_at, starts_at, created_by_admin_id, current_revision, statement_to_seller, updated_at)
SELECT gen_random_uuid(), account_id, store_id, 'RISK', 'SUSPENSION', 'ENFORCED', 'FRAUD', now() + interval '30 days',
       now() - interval '10 days', now() - interval '10 days', 1, 1, '測試停權', now()
FROM seller.stores WHERE store_id % 50 = 7;

\echo '== buyer：買家（2% 刪除中）'
INSERT INTO buyer.buyers (buyer_id, account_id, status, display_name, created_at, updated_at)
OVERRIDING SYSTEM VALUE
SELECT g, 2000000 + g, CASE WHEN g % 50 = 11 THEN 'PENDING_DELETION' ELSE 'ACTIVE' END, '買家' || g,
       now() - interval '500 days', now()
FROM generate_series(1, :buyers) g;

\echo '== social：貼文'
-- 店家偏斜：random()^2，前 1% 的店家各有上千篇。狀態：85% 已發布、8% 下架、2% 草稿、5% 已刪除（軟刪）。
CREATE TEMP TABLE seed_posts AS
SELECT g AS rn, uuidv7() AS id, 1 + floor(:stores * power(random(), 2))::bigint AS store_id,
       (CASE WHEN r < 0.85 THEN 'PUBLISHED' WHEN r < 0.93 THEN 'UNPUBLISHED' ELSE 'DRAFT' END)::text AS status,
       now() - random() * interval '730 days' AS ts, (random() < 0.05) AS gone
FROM (SELECT g, random() AS r FROM generate_series(1, :posts) g) s;

INSERT INTO social.posts (id, store_id, body, image_key, image_asset_id, status, published_at, version, created_at, updated_at, deleted_at)
SELECT id, store_id, '貼文內容 ' || rn || ' ' || repeat('好吃的水果與蔬菜，產地直送。', 4), 'post/' || rn, 'asset-p' || rn, status,
       CASE WHEN status = 'DRAFT' THEN NULL ELSE ts END, 1, ts - interval '1 hour', ts,
       CASE WHEN gone THEN ts + interval '1 day' END
FROM seed_posts;

\echo '== social：標籤（5,000 個，60% 中文）'
-- 中文標籤由編號決定：兩到四個漢字，取自 1500 個漢字的字池，前兩個字的組合不重複（第一個字對 1500 取模時是雙射，第二個字的區段
-- 由編號除以 1500 決定），所以名稱保證不重複，又讓單一漢字在多個標籤裡出現；其餘是 6 到 10 個字母的偽隨機單字（md5 的前綴，數字換成 g–p）。
CREATE TEMP TABLE seed_tags AS
SELECT i, (CASE WHEN i % 5 < 3
           THEN chr(19968 + (i * 37) % 1500) || chr(19968 + (i / 1500) * 300 + (i * 13) % 300)
                || CASE WHEN i % 7 < 3 THEN chr(19968 + (i * 53) % 1500) ELSE '' END
                || CASE WHEN i % 7 = 0 THEN chr(19968 + (i * 29) % 1500) ELSE '' END
           ELSE translate(substr(md5(i::text), 1, 6 + i % 5), '0123456789', 'ghijklmnop') END)::text AS name
FROM generate_series(1, 5000) i;
CREATE UNIQUE INDEX ON seed_tags (name);

-- 含 volatile 函式的子查詢不會被展平，所以每一列只抽一次；把 random() 直接寫在 JOIN 條件裡會對每個配對各抽一次。
INSERT INTO social.post_hashtags (post_id, position, name, display_name)
SELECT s.id, s.pos, t.name, t.name
FROM (
    SELECT p.id, k.pos, 1 + floor(5000 * power(random(), 3))::int AS tag_i
    FROM seed_posts p
    CROSS JOIN LATERAL generate_series(0, (random() * 4)::int) AS k(pos)
) s
JOIN seed_tags t ON t.i = s.tag_i
ON CONFLICT DO NOTHING;
-- 標籤列的 published_at 是貼文 published_at 的複本（trigger 在 replica 模式下不跑，這裡直接補）。
UPDATE social.post_hashtags h SET published_at = p.published_at FROM social.posts p WHERE p.id = h.post_id AND p.published_at IS NOT NULL;

\echo '== social：留言（80% 頂層，其餘是回覆；熱門貼文偏斜）'
CREATE TEMP TABLE seed_post_pick AS SELECT rn, id, store_id, ts FROM seed_posts;
CREATE UNIQUE INDEX ON seed_post_pick (rn);

CREATE TEMP TABLE seed_top AS
SELECT uuidv7() AS id, p.id AS post_id, p.store_id,
       CASE WHEN s.r1 < 0.08 THEN 'STORE' ELSE 'BUYER' END AS kind,
       1 + floor(:buyers * power(s.r2, 1.5))::bigint AS buyer_id,
       p.ts + s.r3 * interval '60 days' AS ts, (s.r4 < 0.03) AS gone, s.g
FROM (
    SELECT g, 1 + floor(:posts * power(random(), 2.5))::int AS pr, random() AS r1, random() AS r2, random() AS r3, random() AS r4
    FROM generate_series(1, (:posts * :comments_per_post * 0.8)::int) g
) s
JOIN seed_post_pick p ON p.rn = s.pr;

INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at, deleted_at, counted)
SELECT id, post_id, NULL, NULL, kind, CASE WHEN kind = 'STORE' THEN store_id ELSE buyer_id END, '留言 ' || g || ' 很好吃', ts, ts,
       CASE WHEN gone THEN ts + interval '1 day' END, NOT gone
FROM seed_top;

INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at, deleted_at, counted)
SELECT uuidv7(), t.post_id, t.id, CASE WHEN random() < 0.3 THEN t.id END,
       CASE WHEN random() < 0.6 THEN 'STORE' ELSE 'BUYER' END,
       CASE WHEN random() < 0.6 THEN t.store_id ELSE 1 + floor(:buyers * random())::bigint END,
       '回覆 ' || t.g, t.ts + interval '1 hour', t.ts + interval '1 hour', CASE WHEN random() < 0.03 THEN t.ts + interval '2 hours' END, TRUE
FROM seed_top t
CROSS JOIN LATERAL generate_series(1, 1 + (random() * 3)::int) r
WHERE t.g % 4 = 0;
-- 旗標 counted 與 stats 同一個定義（沒有刪除、沒有未還原的隱藏、作者不在 inactive_authors）：回覆有 3% 已刪除，補成假。
UPDATE social.comments SET counted = FALSE WHERE deleted_at IS NOT NULL AND counted;

-- 1% 的留言被隱藏，其中三成已還原。
INSERT INTO social.hides (id, comment_id, reason_category, reason_text, admin_id, hidden_at, restored_at, restored_by, restore_reason)
SELECT uuidv7(), c.id, 'SPAM', '測試', 1, c.created_at + interval '1 day',
       CASE WHEN r < 0.3 THEN c.created_at + interval '2 days' END, CASE WHEN r < 0.3 THEN 1 END, CASE WHEN r < 0.3 THEN '還原' END
FROM (SELECT id, created_at, random() AS r FROM social.comments WHERE random() < 0.01) c;
UPDATE social.comments c SET counted = FALSE FROM social.hides h WHERE h.comment_id = c.id AND h.restored_at IS NULL AND c.counted;

-- 買家刪帳的事實的複本（schema.sql 的 inactive_authors）：與 buyer.buyers 不營運中的列一致（2% 刪除中）；這些買家寫的留言旗標補成假。
INSERT INTO social.inactive_authors (buyer_id, since) SELECT buyer_id, now() FROM buyer.buyers WHERE status <> 'ACTIVE';
UPDATE social.comments c SET counted = FALSE
WHERE c.counted AND c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM social.inactive_authors ia WHERE ia.buyer_id = c.actor_id);

\echo '== social：讚'
INSERT INTO social.post_likes (post_id, buyer_id, created_at)
SELECT p.id, s.buyer_id, p.ts + s.r * interval '30 days'
FROM (
    SELECT g, 1 + floor(:posts * power(random(), 2))::int AS pr, 1 + floor(:buyers * random())::bigint AS buyer_id, random() AS r
    FROM generate_series(1, (:posts * :likes_per_post)::int) g
) s
JOIN seed_post_pick p ON p.rn = s.pr
ON CONFLICT DO NOTHING;

INSERT INTO social.comment_likes (comment_id, actor_kind, actor_id, created_at)
SELECT c.id, 'BUYER', 1 + floor(:buyers * random())::bigint, c.created_at + interval '1 hour'
FROM social.comments c WHERE random() < 0.4
ON CONFLICT DO NOTHING;

\echo '== social：討論區（每店約 10 則，兩成是回覆）'
CREATE TEMP TABLE seed_forum_top AS
SELECT uuidv7() AS id, 1 + floor(:stores * power(random(), 2))::bigint AS store_id,
       CASE WHEN random() < 0.1 THEN 'STORE' ELSE 'BUYER' END AS kind,
       1 + floor(:buyers * random())::bigint AS buyer_id, now() - random() * interval '400 days' AS ts, g
FROM generate_series(1, (:stores * 8)::int) g;
INSERT INTO social.forum_entries (id, store_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at, counted)
SELECT id, store_id, NULL, NULL, kind, CASE WHEN kind = 'STORE' THEN store_id ELSE buyer_id END, '討論 ' || g, ts, ts, TRUE FROM seed_forum_top;
INSERT INTO social.forum_entries (id, store_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at, counted)
SELECT uuidv7(), t.store_id, t.id, t.id, 'STORE', t.store_id, '回覆 ' || t.g, t.ts + interval '1 hour', t.ts + interval '1 hour', TRUE
FROM seed_forum_top t WHERE t.g % 4 = 0;
UPDATE social.forum_entries e SET counted = FALSE
WHERE e.counted AND e.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM social.inactive_authors ia WHERE ia.buyer_id = e.actor_id);

\echo '== social：stats（由事實聚合，與 trigger 維護的結果一致）'
-- 先 ANALYZE：聚合與 join 的計畫要看得到剛灌進去的資料量。stats 是旗標的加總（stats = 旗標之和）。
ANALYZE;
INSERT INTO social.post_stats (post_id, like_count, comment_count, view_count, share_count)
SELECT p.id, COALESCE(l.n, 0), COALESCE(c.n, 0), (random() * 5000)::bigint, 0
FROM social.posts p
LEFT JOIN (SELECT post_id, count(*) AS n FROM social.post_likes GROUP BY post_id) l ON l.post_id = p.id
LEFT JOIN (
    SELECT c.post_id, count(*) AS n FROM social.comments c
    WHERE c.counted
    GROUP BY c.post_id
) c ON c.post_id = p.id;
INSERT INTO social.comment_stats (comment_id, like_count, reply_count)
SELECT c.id, COALESCE(l.n, 0), COALESCE(r.n, 0)
FROM social.comments c
LEFT JOIN (SELECT comment_id, count(*) AS n FROM social.comment_likes GROUP BY comment_id) l ON l.comment_id = c.id
LEFT JOIN (SELECT parent_id, count(*) AS n FROM social.comments WHERE counted AND parent_id IS NOT NULL GROUP BY parent_id) r ON r.parent_id = c.id;
INSERT INTO social.forum_entry_stats (forum_entry_id, reply_count)
SELECT e.id, COALESCE(r.n, 0)
FROM social.forum_entries e
LEFT JOIN (SELECT parent_id, count(*) AS n FROM social.forum_entries WHERE counted AND parent_id IS NOT NULL GROUP BY parent_id) r ON r.parent_id = e.id;

-- 處分紀錄（ListSanctions 的 sanctions_buyer 索引要有資料才看得出計畫）：前 2,000 位買家各 10 筆。
\echo '== social：處分'
INSERT INTO social.sanctions (id, buyer_id, kind, reason_category, reason_text, source_ref, admin_id, created_at, ends_at)
SELECT gen_random_uuid(), b.buyer_id, 'WARNING', 'SPAM', '', NULL, 1, now() - (g * interval '1 day') - (random() * interval '12 hours'), NULL
FROM (SELECT buyer_id FROM buyer.buyers ORDER BY buyer_id LIMIT 2000) b, generate_series(1, 10) g;

RESET session_replication_role;
-- VACUUM 設好 visibility map 與 hint bit（index-only scan 才不用回表），ANALYZE 更新統計；剛灌完不做的話，第一輪量測會被 autovacuum 與回表干擾。
\echo '== VACUUM (ANALYZE)'
VACUUM (ANALYZE);
SELECT 'stores' AS what, count(*) FROM seller.stores
UNION ALL SELECT 'exposable stores', count(*) FROM seller.pub_exposable_stores(now())
UNION ALL SELECT 'buyers', count(*) FROM buyer.buyers
UNION ALL SELECT 'posts', count(*) FROM social.posts
UNION ALL SELECT 'published posts', count(*) FROM social.posts WHERE deleted_at IS NULL AND status = 'PUBLISHED'
UNION ALL SELECT 'post_hashtags', count(*) FROM social.post_hashtags
UNION ALL SELECT 'comments', count(*) FROM social.comments
UNION ALL SELECT 'hides', count(*) FROM social.hides
UNION ALL SELECT 'sanctions', count(*) FROM social.sanctions
UNION ALL SELECT 'post_likes', count(*) FROM social.post_likes
UNION ALL SELECT 'comment_likes', count(*) FROM social.comment_likes
UNION ALL SELECT 'forum_entries', count(*) FROM social.forum_entries;
