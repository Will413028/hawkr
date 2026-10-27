-- B-153: Post Hashtag schema (spec 34)
-- 新增 cms_hashtag (canonical name UNIQUE + denormalized post_count) 與
-- cms_post_hashtag (post-hashtag 關聯，含 display_name / position)。
-- pg_trgm 用於 CONTAIN suggest mode 的 trigram 模糊搜尋。
--
-- 注意：CREATE EXTENSION 在 Atlas community 版需登入才能 diff，
-- 因此本檔案沿用 content-service `20260430000001_add_pg_trgm_extension.sql`
-- 慣例手動撰寫。schema/schema.sql 為 source of truth。
--
-- 部署前檢查（spec 34 §9.2）：執行 Atlas 的 DB role 需有 CREATE EXTENSION 權限
-- （RDS: rds_superuser；自管 PG: superuser 或 db owner）。若無權限，先以 superuser
-- 預先 `CREATE EXTENSION pg_trgm;` 後再跑 atlas migrate apply。

-- ===========================================================================
-- 1. pg_trgm extension（CONTAIN mode 模糊搜尋用）
-- ===========================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ===========================================================================
-- 2. cms_hashtag
-- ===========================================================================

CREATE TABLE cms_hashtag (
    id          BIGINT PRIMARY KEY,
    name        VARCHAR(40) NOT NULL,
    post_count  INT NOT NULL DEFAULT 0,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uk_cms_hashtag_name          ON cms_hashtag (name);
CREATE INDEX        idx_cms_hashtag_name_pattern ON cms_hashtag (name varchar_pattern_ops);
CREATE INDEX        idx_cms_hashtag_name_trgm    ON cms_hashtag USING gin (name gin_trgm_ops);

-- ===========================================================================
-- 3. cms_post_hashtag
-- ===========================================================================

CREATE TABLE cms_post_hashtag (
    post_id      BIGINT NOT NULL,
    hashtag_id   BIGINT NOT NULL,
    display_name VARCHAR(40) NOT NULL,
    position     SMALLINT NOT NULL DEFAULT 0,
    create_time  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, hashtag_id)
);
CREATE INDEX idx_cms_post_hashtag_lookup  ON cms_post_hashtag (hashtag_id, post_id);
CREATE INDEX idx_cms_post_hashtag_created ON cms_post_hashtag (create_time, hashtag_id);
