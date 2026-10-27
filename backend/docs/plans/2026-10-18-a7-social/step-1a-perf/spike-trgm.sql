-- A7-1a §9.4 的 spike：hashtag 的「包含」搜尋（name LIKE '%q%'）在單體 Postgres 18 的 locale 下，對漢字的行為。
-- 在 seed.sql 灌好的 database 跑（沒有 pg_trgm 索引時是不用索引的基準；加上索引的版本見 README.md）：
--   psql -v ON_ERROR_STOP=1 -d perf -f spike-trgm.sql
-- 1) pg_trgm 對漢字的切分；2) 1 到 4 個漢字的 LIKE '%q%' 與 strpos 的結果相同（正確性）；3) 各長度的查詢耗時與計畫。

\echo '== 1. 資料庫的 locale 與 pg_trgm 對漢字的切分（需要 pg_trgm；沒有裝時略過這一節）'
SELECT datcollate, datctype, datlocprovider FROM pg_database WHERE datname = current_database();
SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') AS has_trgm \gset
\if :has_trgm
SELECT show_trgm('烘焙') AS cjk_2, show_trgm('烘焙茶') AS cjk_3, show_trgm('烘') AS cjk_1, show_trgm('abc烘焙') AS mixed;
\endif

\echo '== 2. 正確性：LIKE ''%q%'' 與 strpos 在 1–4 個漢字、前綴／中段／結尾的子字串上結果相同'
DO $$
DECLARE
    q text;
    k int;
    pos text;
    n_like bigint;
    n_strpos bigint;
    checked int := 0;
BEGIN
    FOR k IN 1..4 LOOP
        FOREACH pos IN ARRAY ARRAY['start', 'middle', 'end'] LOOP
            FOR q IN
                SELECT DISTINCT CASE pos WHEN 'start' THEN left(name, k)
                                         WHEN 'end' THEN right(name, k)
                                         ELSE substr(name, 2, k) END
                FROM (SELECT DISTINCT name FROM social.post_hashtags WHERE name >= E'一' AND char_length(name) >= k + 1 LIMIT 40) t
            LOOP
                CONTINUE WHEN char_length(q) <> k;
                SELECT count(*) INTO n_like FROM social.post_hashtags WHERE name LIKE '%' || q || '%';
                SELECT count(*) INTO n_strpos FROM social.post_hashtags WHERE strpos(name, q) > 0;
                IF n_like <> n_strpos THEN
                    RAISE EXCEPTION 'LIKE 與 strpos 不一致：q=% LIKE=% strpos=%', q, n_like, n_strpos;
                END IF;
                checked := checked + 1;
            END LOOP;
        END LOOP;
    END LOOP;
    RAISE NOTICE '檢查了 % 個查詢字串，LIKE 與 strpos 全部一致', checked;
END $$;

\echo '== 3. 各長度的耗時與計畫（只看謂詞本身；完整的建議 query 另見 measure.go）'
SELECT left(name, 1) AS q1, left(name, 2) AS q2, left(name, 3) AS q3
FROM social.post_hashtags WHERE name >= E'\u4e00' AND char_length(name) >= 3
GROUP BY name ORDER BY count(*) DESC LIMIT 1 \gset
SELECT left(name, 4) AS q4
FROM social.post_hashtags WHERE name >= E'\u4e00' AND char_length(name) >= 4
GROUP BY name ORDER BY count(*) DESC LIMIT 1 \gset
\echo 查詢字串 :q1 / :q2 / :q3 / :q4
\echo '-- 1 個漢字'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY ON) SELECT name, count(*) FROM social.post_hashtags WHERE name LIKE '%' || :'q1' || '%' GROUP BY name;
\echo '-- 2 個漢字'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY ON) SELECT name, count(*) FROM social.post_hashtags WHERE name LIKE '%' || :'q2' || '%' GROUP BY name;
\echo '-- 3 個漢字'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY ON) SELECT name, count(*) FROM social.post_hashtags WHERE name LIKE '%' || :'q3' || '%' GROUP BY name;
\echo '-- 4 個漢字'
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF, SUMMARY ON) SELECT name, count(*) FROM social.post_hashtags WHERE name LIKE '%' || :'q4' || '%' GROUP BY name;
