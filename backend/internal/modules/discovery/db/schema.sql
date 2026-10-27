-- discovery 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（discovery.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA discovery;

-- 推薦賣家的分數快照（ADR 2026-10-03-recommended-sellers-scoring）。排程每次重算全部合格店家，在同一個 transaction
-- 內整批取代；讀取依 (score, published_at, store_id) 由大到小，再套用即時的曝光資格。
--
-- store_id 是 Java 期間的店家 ID（BIGINT 業務鍵），不建 FK：店家在 seller 模組的 schema。各成分欄位存算出來的值，
-- 連同它的原始輸入與公式版本，所以任何一列都能說明「為何在榜上」，改公式時只換 formula_version。
-- published_at 是這家店的公開時間（seller 的 ExposableStore.PublishedAt：stores.first_exposed_at，第一次可公開曝光的時間，
-- 不是核准時間；還沒被記下的是讀取當下），新店加分與同分時的
-- 排序都以它為準。computed_at 是產生這一份快照的那次重算的開始時間（同一份快照的每一列相同，也等於 recompute_state 的
-- last_succeeded_started_at）。
-- score 不設範圍的 CHECK：它的範圍取決於公式的權重（在程式裡，隨 formula_version 改變），資料庫不重複寫死權重；只擋 NaN
-- 與無限大（NaN 在 PostgreSQL 的比較大於一切數值，所以 < 'Infinity' 把它也擋掉）。各成分的值域是 0 到 1，與公式版本無關。
CREATE TABLE discovery.seller_scores (
    store_id                BIGINT PRIMARY KEY,
    score                   DOUBLE PRECISION NOT NULL,
    demand                  DOUBLE PRECISION NOT NULL,
    quality                 DOUBLE PRECISION NOT NULL,
    reliability_penalty     DOUBLE PRECISION NOT NULL,
    newness                 DOUBLE PRECISION NOT NULL,
    distinct_buyers         BIGINT NOT NULL,
    qualifying_orders       BIGINT NOT NULL,
    seller_cancelled_orders BIGINT NOT NULL,
    rating_count            BIGINT NOT NULL,
    rating_average          DOUBLE PRECISION,
    published_at            TIMESTAMPTZ NOT NULL,
    formula_version         INTEGER NOT NULL,
    computed_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_scores_store_check CHECK (store_id > 0),
    CONSTRAINT seller_scores_score_check CHECK (score > '-Infinity'::double precision AND score < 'Infinity'::double precision),
    CONSTRAINT seller_scores_demand_check CHECK (demand BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_quality_check CHECK (quality BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_penalty_check CHECK (reliability_penalty BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_newness_check CHECK (newness BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_counts_check CHECK (
        distinct_buyers >= 0 AND qualifying_orders >= 0 AND seller_cancelled_orders >= 0 AND rating_count >= 0
    ),
    -- 沒有評價就沒有平均（NULL），有評價就必須有平均。
    CONSTRAINT seller_scores_rating_check CHECK (
        (rating_count = 0 AND rating_average IS NULL) OR (rating_count > 0 AND rating_average BETWEEN 0 AND 5)
    ),
    CONSTRAINT seller_scores_version_check CHECK (formula_version >= 1)
);

CREATE INDEX seller_scores_rank_idx ON discovery.seller_scores (score DESC, published_at DESC, store_id DESC);

-- 重算的新鮮度與序列化（ADR「重算」）：每個排程工作一列（目前只有 seller_scores）。
-- last_attempt_started_at：最近一次嘗試的開始時間；last_succeeded_started_at／last_succeeded_at：目前快照是哪一次重算的
-- 結果（開始與結束時間，「不存在」≠「沒有合格店家」：從沒成功過這兩欄是 NULL）；last_failed_at／last_error：最近一次失敗的
-- 時間與有界的錯誤文字（成功不清除它，用 last_failed_at 與 last_succeeded_at 比較新舊）。
-- 取代快照的 transaction 先鎖這一列，只有開始時間晚於 last_succeeded_started_at 的重算才能寫入，重疊或重試時
-- 舊的結果不會蓋掉新的。告警不在模組內：由單體對 River job 狀態的監控負責（CD 待辦）。
CREATE TABLE discovery.recompute_state (
    job                       TEXT PRIMARY KEY,
    last_attempt_started_at   TIMESTAMPTZ NOT NULL,
    last_succeeded_started_at TIMESTAMPTZ,
    last_succeeded_at         TIMESTAMPTZ,
    last_failed_at            TIMESTAMPTZ,
    last_error                TEXT,
    CONSTRAINT recompute_state_job_check CHECK (job ~ '^[a-z][a-z_]{0,39}$'),
    CONSTRAINT recompute_state_success_check CHECK (
        (last_succeeded_started_at IS NULL AND last_succeeded_at IS NULL) OR
        (last_succeeded_started_at IS NOT NULL AND last_succeeded_at >= last_succeeded_started_at AND
         last_succeeded_started_at <= last_attempt_started_at)
    ),
    CONSTRAINT recompute_state_failure_check CHECK (
        (last_failed_at IS NULL AND last_error IS NULL) OR
        (last_failed_at IS NOT NULL AND last_error IS NOT NULL AND char_length(last_error) BETWEEN 1 AND 500)
    )
);
