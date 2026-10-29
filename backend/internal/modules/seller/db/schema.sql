-- seller 模組的表、約束與索引的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。view、函式與複合型別在 objects.sql（手寫 migration，單體規則 6），
-- 不放這裡：Atlas 社群版在 desired state 裡遇到 view 就報錯。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（seller.<表>），不建跨 schema 的 FK。store_id、period_id 仍被 Java 存成 BIGINT（product 的下架
-- 快照以 period_id 為 scope，Java 稱它 suspensionId），依 ADR 2026-09-25-row-identity-by-data-kind 的過渡規則暫用
-- IDENTITY；措施、通報與違規沒有 Java 的消費者，是實體（UUIDv7，應用層產生）。慣例見 PostgreSQL
-- 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA seller;

-- 店家：一列是一個 stint（ADR 2026-07-11-identity-store-stint-symmetry）。重新入駐新增一列，帶新的 store_id 與
-- store_code；目前的 stint 是 status 不是 CLOSED 的那一列，每個帳號最多一列（stores_current_account）。
-- store_code 是 PayUni 看到的外部代碼（MerID 由它組成），所以每個 stint 都要新的。
-- first_exposed_at 是這家店的公開時間：第一次被觀察到可公開曝光的時間（只在公告可曝光時由 profile 的公告函式寫入，寫一次不覆寫，永久保留）。
-- 新店加分以它起算（discovery）；店家卡片也只給「曾經公開過」的店（store_id 連號可列舉，沒公開過的店名不能被逐號讀出），store_id 換成
-- uuid 之後只移除卡片的這個過濾，欄位保留。
-- 停權狀態不存在這裡：它由措施（measures）在讀取時推導（ADR 2026-10-18-seller-governance-consolidated「決策」的「認定、措施與效果」
-- 與「考慮過的方案」的「停權狀態」）；文字欄位不存空字串，沒有值就是 NULL。
-- town_code 是內政部的鄉鎮市區代碼（8 碼，前 5 碼是縣市；表在 platform/twaddr，寫入端檢查是現行的代碼）。
-- logo、封面存 object key 與 media 的 asset ID：asset ID 是同步 job 登記引用時業務列目前指著的資產。
-- 個資清理（ADR 2026-10-19-seller-kyc-payuni-consolidated「關店與個資保留」）：stint 關閉之後過了保留期（一般關店 184 天，model.StoreRetention；
-- 因終止合作而關閉的，到帳號上未撤銷終止中最晚的申覆期限，且不早於關閉後 184 天；帳號上有終止的待處理申覆時暫停）才由 purge 套件清掉
-- store_name、description、contact_phone、contact_name、contact_mobile、support_email、地址、prize、website_url，並釋放 logo 與封面，記下 pii_purged_at；
-- store_name 因此可以是 NULL（只有清理過的列），呈現層對已清理的店家顯示墓碑。保留期的判斷只有一處（RetireDueStores 那支查詢，清理與恢復共用）；
-- stores_pii_purged_check 讓資料庫保證清理過的列一定是 CLOSED、欄位一定是 NULL、沒清理的一定有店名。
-- 關店可逆：不可逆的下游（logo 與封面的釋放、seller.deleted）都在清理時；CLOSED 的 stint 在保留期內可以恢復。關店的紀錄（何時、為什麼、哪個
-- 終止合作措施）只存在 store_closures 一處，status 是 CLOSED 的 stint 有一筆目前的關店紀錄（reopened_at IS NULL），反過來也成立，由資料庫保證（current_closed_at、stores_current_closure_fkey、store_closures_current_fkey），closing 套件同一個 transaction 維護。
CREATE TABLE seller.stores (
    store_id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_code           BIGINT NOT NULL,
    account_id           BIGINT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'ACTIVE',
    version              BIGINT NOT NULL DEFAULT 1,
    store_name           TEXT,
    description          TEXT,
    logo_key             TEXT,
    logo_asset_id        TEXT,
    cover_key            TEXT,
    cover_asset_id       TEXT,
    contact_phone        TEXT,
    contact_name         TEXT,
    contact_mobile       TEXT,
    support_email        TEXT,
    town_code            TEXT,
    address_detail       TEXT,
    prize                TEXT,
    website_url          TEXT,
    onboarding_completed BOOLEAN NOT NULL DEFAULT false,
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL,
    pii_purged_at        TIMESTAMPTZ,
    first_exposed_at     TIMESTAMPTZ,
    current_closed_at    TIMESTAMPTZ,
    retired_at           TIMESTAMPTZ,
    CONSTRAINT stores_status_check CHECK (status IN ('ACTIVE', 'PENDING_DELETION', 'CLOSED')),
    -- 關店後的兩段清理（ADR 2026-10-19-seller-kyc-payuni-consolidated 的 2026-10-20 修訂）：retired_at 是第一段（關店後 30 天：公開內容與圖片清掉、
    -- 發 StoreRetired 讓下游刪除，之後不能恢復營業），pii_purged_at 是第二段（關店後 184 天：註冊、身分與連線資料）。第二段一定在第一段之後。
    CONSTRAINT stores_closed_images_check CHECK (retired_at IS NULL OR (logo_key IS NULL AND cover_key IS NULL)),
    CONSTRAINT stores_retired_check CHECK (
        retired_at IS NULL OR (status = 'CLOSED' AND description IS NULL AND prize IS NULL AND website_url IS NULL)
    ),
    CONSTRAINT stores_pii_purged_check CHECK (
        (pii_purged_at IS NULL AND store_name IS NOT NULL) OR
        (pii_purged_at IS NOT NULL AND retired_at IS NOT NULL AND status = 'CLOSED' AND
         store_name IS NULL AND contact_phone IS NULL AND contact_name IS NULL AND contact_mobile IS NULL AND
         support_email IS NULL AND town_code IS NULL AND address_detail IS NULL)
    ),
    CONSTRAINT stores_current_closure_check CHECK ((status = 'CLOSED') = (current_closed_at IS NOT NULL)),
    CONSTRAINT stores_version_check CHECK (version >= 1),
    CONSTRAINT stores_store_code_check CHECK (store_code BETWEEN 1 AND 9999999999),
    -- 店名頭尾不是空白（platform/trimtext.Check，與寫入端的 trimtext.Trim 同一個集合），所以有值就不是空白，曝光條件 has_store_name 只看 NOT NULL。
    CONSTRAINT stores_store_name_check CHECK (
        char_length(store_name) >= 1 AND char_length(store_name) <= 100 AND
        store_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'
    ),
    CONSTRAINT stores_description_check CHECK (char_length(description) BETWEEN 1 AND 2000),
    CONSTRAINT stores_images_check CHECK (logo_key <> '' AND cover_key <> '' AND logo_asset_id <> '' AND cover_asset_id <> ''),
    CONSTRAINT stores_image_assets_check CHECK ((logo_key IS NULL) = (logo_asset_id IS NULL) AND (cover_key IS NULL) = (cover_asset_id IS NULL)),
    CONSTRAINT stores_contact_phone_check CHECK (contact_phone ~ '^0[0-9]{8,9}$'),
    CONSTRAINT stores_contact_name_check CHECK (char_length(contact_name) >= 1 AND char_length(contact_name) <= 20 AND contact_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT stores_contact_mobile_check CHECK (contact_mobile ~ '^09[0-9]{8}$'),
    CONSTRAINT stores_support_email_check CHECK (
        char_length(support_email) <= 50 AND support_email = lower(support_email) AND support_email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'
    ),
    CONSTRAINT stores_town_code_check CHECK (town_code ~ '^[0-9]{8}$'),
    CONSTRAINT stores_address_detail_check CHECK (
        address_detail IS NULL OR (town_code IS NOT NULL AND char_length(address_detail) BETWEEN 1 AND 255)
    ),
    CONSTRAINT stores_prize_check CHECK (char_length(prize) BETWEEN 1 AND 255),
    CONSTRAINT stores_website_url_check CHECK (website_url ~ '^https?://[^/?#]+' AND char_length(website_url) <= 500)
);
-- 子表的 account_id 是 stores.account_id 的複本（措施與認定的效果以帳號為單位），以 (store_id, account_id) 的複合外鍵
-- 由資料庫保證兩者相等；店家的 account_id 建立後不變。
ALTER TABLE seller.stores ADD CONSTRAINT stores_store_account_key UNIQUE (store_id, account_id);
-- current_closed_at 是目前那筆關店紀錄的 closed_at（status 是 CLOSED 才有，stores_current_closure_check）；store_closures 以複合外鍵指回來。
ALTER TABLE seller.stores ADD CONSTRAINT stores_current_closure_key UNIQUE (store_id, current_closed_at);
CREATE UNIQUE INDEX stores_store_code ON seller.stores (store_code);
CREATE UNIQUE INDEX stores_current_account ON seller.stores (account_id) WHERE status <> 'CLOSED';
-- 停權的效果以帳號為單位（任何一個 stint 有生效中的停權），以帳號找全部的 stint。
CREATE INDEX stores_account ON seller.stores (account_id);
-- 後台的店家清單：新到舊的 keyset。
CREATE INDEX stores_admin_list ON seller.stores (created_at DESC, store_id DESC);
-- 已關閉而還沒退場的 stint（第一段清理的候選）與已退場而個資還沒清理的 stint（第二段的候選）。
CREATE INDEX stores_retire_pending ON seller.stores (store_id) WHERE status = 'CLOSED' AND retired_at IS NULL;
CREATE INDEX stores_purge_pending ON seller.stores (store_id) WHERE retired_at IS NOT NULL AND pii_purged_at IS NULL;

-- 關店的紀錄，關店的唯一一份：每次關店一列（誰關的：賣家自己、終止合作連同造成關店的措施，或隨帳號刪除），stint 目前是關閉的就是 reopened_at 為 NULL 的那一列
-- （每個 stint 最多一列，store_closures_current），恢復營業時補上 reopened_at、紀錄永久保留。一個 stint 可以關閉、恢復、再關閉，所以主鍵帶關閉時間。
-- closed_by_measure_id 是造成關店的措施，關店當時的事實，不隨措施之後的撤銷改變。stores.status 是 CLOSED 的 stint 一定有目前的那一列、不是的一定沒有，
-- 由兩個可延遲的複合外鍵在 commit 時保證（先例 violations_current_revision_fkey）：stores 的 (store_id, current_closed_at) 指向目前那一列的
-- (store_id, open_closed_at)，目前那一列（open_closed_at 非 NULL）的 (store_id, open_closed_at) 又指回 stores；加上 stores_current_closure_check
-- 綁住 status 與指標，CLOSED 沒有目前的紀錄、有兩筆目前的紀錄、恢復營業卻沒補 reopened_at 或沒清指標，都在 commit 失敗。
CREATE TABLE seller.store_closures (
    store_id             BIGINT NOT NULL REFERENCES seller.stores (store_id),
    closed_at            TIMESTAMPTZ NOT NULL,
    closed_by            TEXT NOT NULL,
    closed_by_measure_id UUID,
    reopened_at          TIMESTAMPTZ,
    open_closed_at       TIMESTAMPTZ GENERATED ALWAYS AS (CASE WHEN reopened_at IS NULL THEN closed_at END) STORED,
    PRIMARY KEY (store_id, closed_at),
    CONSTRAINT store_closures_open_key UNIQUE (store_id, open_closed_at),
    CONSTRAINT store_closures_closed_by_check CHECK (closed_by IN ('ACCOUNT_OWNER', 'TERMINATION', 'ACCOUNT_DELETION')),
    CONSTRAINT store_closures_measure_check CHECK ((closed_by = 'TERMINATION') = (closed_by_measure_id IS NOT NULL)),
    CONSTRAINT store_closures_reopened_check CHECK (reopened_at IS NULL OR reopened_at >= closed_at)
);
CREATE UNIQUE INDEX store_closures_current ON seller.store_closures (store_id) WHERE reopened_at IS NULL;
-- 終止合作的措施找它關閉的 stint。
CREATE INDEX store_closures_measure ON seller.store_closures (closed_by_measure_id) WHERE closed_by_measure_id IS NOT NULL;


-- 最後公告的可曝光（ADR 2026-10-05-store-exposure-to-legacy 決策 1）：每家店一列，與店家列在同一個 transaction 建立
-- （onboarding 的開店；既有的店家由 migration 補），記下最近一次以 StoreExposureChanged 公告的值，讓「現在算出來的可曝光」與
-- 「已經告訴下游的」可以比對。version 是公告序號：從 1 開始、每次公告加一，下游依它排序並丟棄舊版本；version 0 是還沒公告過
-- （exposable 一定是 false、exposable_changed_at 是 NULL）。exposable_changed_at 是公告值最後一次翻轉的時間（第一次公告也記），重發同一個值不動它。只有 exposure 套件的公告函式更新；寫入者先鎖店家列再鎖這一列（鎖順序：
-- 店家、申請、註冊、公告），鎖住之後才算現在的可曝光。
CREATE TABLE seller.store_exposures (
    store_id             BIGINT PRIMARY KEY REFERENCES seller.stores (store_id),
    exposable            BOOLEAN NOT NULL DEFAULT false,
    version              BIGINT NOT NULL DEFAULT 0,
    exposable_changed_at TIMESTAMPTZ,
    CONSTRAINT store_exposures_version_check CHECK (version >= 0),
    CONSTRAINT store_exposures_exposable_changed_check CHECK ((version = 0) = (exposable_changed_at IS NULL)),
    CONSTRAINT store_exposures_placeholder_check CHECK (version > 0 OR NOT exposable)
);
-- 縣市篩選與對帳只看最後公告為可曝光的店。
CREATE INDEX store_exposures_exposable ON seller.store_exposures (store_id) WHERE exposable;

-- 案件（step10e-reports-brief R2、R3，Will 2026-10-18）：同一家店、同一個平台判定類別，同時最多一個進行中的案件（cases_one_open）。嚴重度在開案時設定，之後只經
-- ChangeCaseSeverity 修改（改前改後記在內部註記上：governance_notes.severity_from、severity_to）。決定就是結案（終態）：close_reason 是 DECIDED 時帶 decision
-- （ACTION_TAKEN 或 NO_ACTION）；通報都移到其他案件時以 NO_REPORTS 關閉，不算決定。version 是後台的 etag，updated_at 是上一次「處理」的時間（成員進出、成員有了結果、
-- 改嚴重度、結案；補充、註記與措施不改它）。處理狀態（進行中、已結案）由 closed_at 在讀取時推導，不存。
-- 兩個複合唯一鍵給別的表的複合外鍵：reports 以 (case_id, store_id, assessed_category) 指向 (id, store_id, category)，所以通報的店家與平台判定類別一定等於案件的；
-- measures.source_case_id 與 report_case_moves 以 (id, store_id) 指向它。
-- DB 保證不了、由模組保證並以資料集測試守的跨列不變量：I1 進行中的案件至少有一筆通報；I2 已結案的案件每筆通報都有結果；I3 決定是 NO_ACTION 的案件沒有結果是 ACTION_TAKEN 的通報；
-- I4 決定是 ACTION_TAKEN 的案件，決定當下有導出的措施（不論狀態）或結果是 ACTION_TAKEN 的成員；I5 每個進行中的案件至少有一個可以選的決定；I6 有導出措施的案件不會因移動而變空。
CREATE TABLE seller.cases (
    id                 UUID PRIMARY KEY,
    store_id           BIGINT NOT NULL REFERENCES seller.stores (store_id),
    category           TEXT NOT NULL,
    severity           TEXT NOT NULL,
    version            BIGINT NOT NULL DEFAULT 1,
    opened_at          TIMESTAMPTZ NOT NULL,
    opened_by_admin_id BIGINT NOT NULL,
    closed_at          TIMESTAMPTZ,
    closed_by_admin_id BIGINT,
    close_reason       TEXT,
    decision           TEXT,
    decision_note      TEXT,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT cases_report_key UNIQUE (id, store_id, category),
    CONSTRAINT cases_store_key UNIQUE (id, store_id),
    -- 政策分類（platform/policy）中平台判定與認定可用的值：全部除了 OTHER（含社群類別，內容下架措施的認定用它們）。
    CONSTRAINT cases_category_check CHECK (category IN (
        'FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT',
        'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE'
    )),
    CONSTRAINT cases_severity_check CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    -- 結案欄位只有三種組合。整個式子要是 TRUE（IS TRUE）：close_reason 或 decision 是 NULL 時，各個比較都是 NULL，CHECK 對 NULL 是放行的，
    -- 沒有 IS TRUE 的話，有結案時間卻沒有原因、或原因是 DECIDED 卻沒有決定的列會溜過去。
    CONSTRAINT cases_close_check CHECK ((
        (closed_at IS NULL AND closed_by_admin_id IS NULL AND close_reason IS NULL AND decision IS NULL AND decision_note IS NULL) OR
        (closed_at >= opened_at AND closed_by_admin_id > 0 AND close_reason = 'DECIDED' AND decision IN ('ACTION_TAKEN', 'NO_ACTION') AND
         (decision_note IS NULL OR char_length(decision_note) BETWEEN 1 AND 2000)) OR
        (closed_at >= opened_at AND closed_by_admin_id > 0 AND close_reason = 'NO_REPORTS' AND decision IS NULL AND decision_note IS NULL)
    ) IS TRUE),
    CONSTRAINT cases_admin_check CHECK (opened_by_admin_id > 0),
    CONSTRAINT cases_version_check CHECK (version >= 1),
    CONSTRAINT cases_updated_check CHECK (updated_at >= opened_at)
);
CREATE UNIQUE INDEX cases_one_open ON seller.cases (store_id, category) WHERE closed_at IS NULL;
-- 後台案件清單（開案時間新到舊）與它的狀態篩選；店家篩選。
CREATE INDEX cases_opened ON seller.cases (opened_at DESC, id DESC);
CREATE INDEX cases_open ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NULL;
CREATE INDEX cases_closed ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NOT NULL;
CREATE INDEX cases_store ON seller.cases (store_id, opened_at DESC, id DESC);

-- 通報（ADR 2026-10-18-seller-governance-consolidated、step10e-reports-brief R2）：檢舉與食安通報同一張表，一列是一位檢舉人的一次送出（之後的補充在 report_supplements）。
-- 檢舉人剛好一種（reports_reporter_check）：登入的買家（reporter_buyer_id，呼叫方給的業務鍵），或留下電話、email 的匿名者；匿名的聯絡欄位在保留期限後清成 NULL，
-- 記下 contact_erased_at（保留規則：結果設定後 30 天，且不早於建立後 90 天；沒有結果就不清）。
-- 每筆通報有自己的結果（outcome）：ACTION_TAKEN、NO_VIOLATION、INSUFFICIENT、DUPLICATE、ABUSIVE。通報二選一：分案（case_id、assessed_category、triaged_*：判定類別並掛到
-- 同一家店、同一個類別的進行中案件，複合外鍵保證店家與類別一致），或不開案件直接設定結果（除了 ACTION_TAKEN：它一定在案件裡）。ACTION_TAKEN 只會出現在案件裡
-- （reports_action_taken_check）。處理狀態（待分案、在案件裡、已有結果）在讀取時推導，不存。
-- version 是後台的 etag：補充、分案、設定結果（個別或結案帶入）與移動都加一。discarded_submissions 是補充已達上限、內容沒有存下的匿名送出次數。
-- 去重（partial UNIQUE）只比對還沒有結果的通報。product_id、order_no 是不透明的參照（擁有者是別的模組），不驗證存在。
CREATE TABLE seller.reports (
    id                    UUID PRIMARY KEY,
    store_id              BIGINT NOT NULL REFERENCES seller.stores (store_id),
    product_id            TEXT,
    order_no              TEXT,
    category              TEXT NOT NULL,
    description           TEXT NOT NULL,
    reporter_buyer_id     BIGINT,
    reporter_phone        TEXT,
    reporter_email        TEXT,
    reporter_name         TEXT,
    contact_erased_at     TIMESTAMPTZ,
    case_id               UUID,
    assessed_category     TEXT,
    triaged_at            TIMESTAMPTZ,
    triaged_by_admin_id   BIGINT,
    outcome               TEXT,
    resolved_at           TIMESTAMPTZ,
    resolved_by_admin_id  BIGINT,
    resolution_note       TEXT,
    discarded_submissions INTEGER NOT NULL DEFAULT 0,
    -- 管理員看過的最新補充的編號（report_supplements.seq；0 = 還沒看過任何補充）。分案、移動與明確已讀（MarkReportSupplementsSeen）把它設成當時最新一則的編號
    -- （三個動作都帶讀到的 etag，etag 含當時全部補充，所以做了就等於看過）。「有新的補充」= 還沒有結果，而且存在編號大於它的補充；不比時間，所以不依賴任何時鐘。
    -- 讀取、設定結果（有了結果就不再提醒）、結案帶入的預設結果、改案件嚴重度都不記。
    supplements_seen_seq  SMALLINT NOT NULL DEFAULT 0,
    version               BIGINT NOT NULL DEFAULT 1,
    created_at            TIMESTAMPTZ NOT NULL,
    updated_at            TIMESTAMPTZ NOT NULL,
    -- 政策分類（platform/policy）中檢舉人可以選的值：全部除了 PLATFORM_ABUSE。
    CONSTRAINT reports_category_check CHECK (category IN (
        'FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT',
        'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'OTHER'
    )),
    CONSTRAINT reports_outcome_check CHECK (outcome IN ('ACTION_TAKEN', 'NO_VIOLATION', 'INSUFFICIENT', 'DUPLICATE', 'ABUSIVE')),
    CONSTRAINT reports_description_check CHECK (char_length(description) BETWEEN 1 AND 2000),
    CONSTRAINT reports_product_id_check CHECK (char_length(product_id) BETWEEN 1 AND 64),
    CONSTRAINT reports_order_no_check CHECK (char_length(order_no) BETWEEN 1 AND 64),
    CONSTRAINT reports_reporter_check CHECK (
        (reporter_buyer_id IS NOT NULL AND reporter_buyer_id > 0 AND reporter_phone IS NULL AND reporter_email IS NULL AND reporter_name IS NULL AND contact_erased_at IS NULL) OR
        (reporter_buyer_id IS NULL AND contact_erased_at IS NULL AND (reporter_phone IS NOT NULL OR reporter_email IS NOT NULL)) OR
        (reporter_buyer_id IS NULL AND contact_erased_at IS NOT NULL AND reporter_phone IS NULL AND reporter_email IS NULL AND reporter_name IS NULL)
    ),
    CONSTRAINT reports_reporter_phone_check CHECK (reporter_phone ~ '^0[0-9]{8,9}$'),
    CONSTRAINT reports_reporter_email_check CHECK (
        reporter_email = lower(reporter_email) AND char_length(reporter_email) <= 254 AND reporter_email ~ '^[^@[:space:]]+@[^@[:space:]]+$'
    ),
    CONSTRAINT reports_reporter_name_check CHECK (char_length(reporter_name) BETWEEN 1 AND 100),
    CONSTRAINT reports_triage_check CHECK (
        (case_id IS NULL) = (assessed_category IS NULL) AND (case_id IS NULL) = (triaged_at IS NULL) AND
        (triaged_at IS NULL) = (triaged_by_admin_id IS NULL) AND (triaged_at IS NULL OR triaged_at >= created_at)
    ),
    CONSTRAINT reports_resolved_check CHECK (
        (outcome IS NULL) = (resolved_at IS NULL) AND (resolved_at IS NULL) = (resolved_by_admin_id IS NULL) AND
        (resolution_note IS NULL OR (resolved_at IS NOT NULL AND char_length(resolution_note) BETWEEN 1 AND 2000)) AND
        (resolved_at IS NULL OR resolved_at >= created_at)
    ),
    CONSTRAINT reports_action_taken_check CHECK (outcome IS DISTINCT FROM 'ACTION_TAKEN' OR case_id IS NOT NULL),
    CONSTRAINT reports_admin_check CHECK (triaged_by_admin_id > 0 AND resolved_by_admin_id > 0),
    CONSTRAINT reports_counts_check CHECK (discarded_submissions >= 0 AND version >= 1),
    CONSTRAINT reports_supplements_seen_check CHECK (supplements_seen_seq BETWEEN 0 AND 10),
    CONSTRAINT reports_store_key UNIQUE (id, store_id),
    CONSTRAINT reports_case_fkey FOREIGN KEY (case_id, store_id, assessed_category) REFERENCES seller.cases (id, store_id, category)
);
-- admin 的通報清單與各篩選條件的 keyset（新到舊）。
CREATE INDEX reports_created ON seller.reports (created_at DESC, id DESC);
CREATE INDEX reports_store ON seller.reports (store_id, created_at DESC, id DESC);
CREATE INDEX reports_reporter ON seller.reports (reporter_buyer_id, created_at DESC, id DESC) WHERE reporter_buyer_id IS NOT NULL;
-- 「待分案」的存取路徑：沒有案件也沒有結果的通報，依建立時間舊到新；案件的成員，依建立時間舊到新。
CREATE INDEX reports_awaiting_triage ON seller.reports (created_at, id) WHERE case_id IS NULL AND outcome IS NULL;
CREATE INDEX reports_case ON seller.reports (case_id, created_at, id) WHERE case_id IS NOT NULL;
-- 去重（舊 ADR 決策五的 partial UNIQUE，日與週以台北時間為界；只比對還沒有結果的通報，有了結果的通報之後同一天可以再送新的）：登入的檢舉人對同一個店家、同一個類別一天一筆；
-- 匿名的同一支電話或同一個 email 對同一個店家一週（ISO 週，週一起算）一筆。
CREATE UNIQUE INDEX reports_buyer_daily ON seller.reports (
    reporter_buyer_id, store_id, category, ((created_at AT TIME ZONE 'Asia/Taipei')::date)
) WHERE reporter_buyer_id IS NOT NULL AND outcome IS NULL;
CREATE UNIQUE INDEX reports_phone_weekly ON seller.reports (
    reporter_phone, store_id, date_trunc('week', created_at AT TIME ZONE 'Asia/Taipei')
) WHERE reporter_phone IS NOT NULL AND outcome IS NULL;
CREATE UNIQUE INDEX reports_email_weekly ON seller.reports (
    reporter_email, store_id, date_trunc('week', created_at AT TIME ZONE 'Asia/Taipei')
) WHERE reporter_email IS NOT NULL AND outcome IS NULL;
-- 匿名聯絡欄位的保留期限清理有兩條路：有結果的依結果設定的時間（greatest(建立 + 90 天, 結果 + 30 天)），以及不論有沒有結果、建立滿 365 天的絕對上限（依建立時間）；
-- 清理查詢依建立時間掃描，兩條路共用這一個索引（以 resolved_at 為鍵的索引量過：EXPLAIN 不選它，所以沒有建）。
CREATE INDEX reports_contact_cap ON seller.reports (created_at) WHERE reporter_buyer_id IS NULL AND contact_erased_at IS NULL;

-- 補充（R7）：去重期間內同一位檢舉人的重複送出，內容併入原通報；只新增，每筆通報最多 10 則（seq 1..10 且每筆通報內唯一，由 DB 保證）。matched_by 只在匿名的補充有值：以電話、聯絡信箱或兩者都相同比對到原通報。
-- 補充不改案件的版本（案件顯示「有新的補充」），只讓通報的版本前進。
CREATE TABLE seller.report_supplements (
    id          UUID PRIMARY KEY,
    report_id   UUID NOT NULL REFERENCES seller.reports (id),
    description TEXT NOT NULL,
    matched_by  TEXT,
    -- 這是通報的第幾則補充（1..10）；佐證的 submission_seq 指向它（原通報是 0）。
    seq         SMALLINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT report_supplements_report_key UNIQUE (id, report_id),
    CONSTRAINT report_supplements_seq_key UNIQUE (id, report_id, seq),
    CONSTRAINT report_supplements_report_seq_key UNIQUE (report_id, seq),
    CONSTRAINT report_supplements_seq_check CHECK (seq BETWEEN 1 AND 10),
    CONSTRAINT report_supplements_description_check CHECK (char_length(description) BETWEEN 1 AND 2000),
    CONSTRAINT report_supplements_matched_by_check CHECK (matched_by IN ('PHONE', 'EMAIL', 'BOTH'))
);
CREATE INDEX report_supplements_report ON seller.report_supplements (report_id, created_at, id);

-- 通報的佐證圖片：值的集合，隨「一次送出」（submission_seq 0 是原通報、1..10 是第幾則補充）一次寫入。鍵是 (通報, 第幾次送出, 次內位置)，次內位置 0..4，所以「每次送出最多 5 張」
-- 由 DB 保證（不只是模組檢查），一筆通報最多 5 × 11 張。supplement_id 與 submission_seq 一致：原通報沒有補充列，補充的佐證指向同一則補充（複合外鍵帶 seq）。
-- API 的「第 n 張」是 (submission_seq, position) 的順序，由讀取端編號，不存。media 的引用以 ref_id = 通報 ID 登記（GOVERNANCE_EVIDENCE），存 asset ID
-- 是同步 job 登記引用時業務列目前指著的資產。匿名檢舉不收佐證。
CREATE TABLE seller.report_evidence (
    report_id      UUID NOT NULL REFERENCES seller.reports (id),
    submission_seq SMALLINT NOT NULL,
    position       INTEGER NOT NULL,
    supplement_id  UUID,
    asset_id       TEXT NOT NULL,
    object_key     TEXT NOT NULL,
    PRIMARY KEY (report_id, submission_seq, position),
    CONSTRAINT report_evidence_submission_check CHECK (submission_seq BETWEEN 0 AND 10),
    CONSTRAINT report_evidence_position_check CHECK (position BETWEEN 0 AND 4),
    CONSTRAINT report_evidence_supplement_check CHECK ((submission_seq = 0) = (supplement_id IS NULL)),
    CONSTRAINT report_evidence_asset_check CHECK (asset_id <> '' AND object_key <> ''),
    CONSTRAINT report_evidence_supplement_fkey FOREIGN KEY (supplement_id, report_id, submission_seq) REFERENCES seller.report_supplements (id, report_id, seq)
);

-- 通報移到其他案件的紀錄（append-only）：複合外鍵保證通報與兩個案件都是同一家店。
CREATE TABLE seller.report_case_moves (
    id           UUID PRIMARY KEY,
    report_id    UUID NOT NULL,
    store_id     BIGINT NOT NULL,
    from_case_id UUID NOT NULL,
    to_case_id   UUID NOT NULL,
    admin_id     BIGINT NOT NULL,
    moved_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT report_case_moves_report_fkey FOREIGN KEY (report_id, store_id) REFERENCES seller.reports (id, store_id),
    CONSTRAINT report_case_moves_from_fkey FOREIGN KEY (from_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT report_case_moves_to_fkey FOREIGN KEY (to_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT report_case_moves_distinct_check CHECK (from_case_id <> to_case_id),
    CONSTRAINT report_case_moves_admin_check CHECK (admin_id > 0)
);
CREATE INDEX report_case_moves_from ON seller.report_case_moves (from_case_id, moved_at, id);
CREATE INDEX report_case_moves_to ON seller.report_case_moves (to_case_id, moved_at, id);

-- 詢問（rb R4）：平台對案件的賣家的一次要求，有期限、一則回覆、可撤回；每個案件同時最多一筆還沒結束的。
-- 回覆在 3c-b 只由管理員記下（系統外收到）；10f 加賣家在賣家中心的回覆（channel = SELLER_CENTER、replied_by_account_id）。
-- 回覆放在詢問列上而不是另一張表：一筆詢問一則回覆，一列一個事實，CHECK 直接保證「回覆與撤回互斥」；10f 改成多則回覆時才拆表（觸發：賣家要能補交）。
-- store_id 是案件店家的複本，由複合外鍵保證相等：附件的 media 引用與 10f 賣家端「這是不是我的店」的授權都要它，不必 join 案件（先例 measures.account_id）。
CREATE TABLE seller.case_inquiries (
    id                         UUID PRIMARY KEY,
    case_id                    UUID NOT NULL,
    store_id                   BIGINT NOT NULL,
    kind                       TEXT NOT NULL,
    message                    TEXT NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL,
    created_by_admin_id        BIGINT NOT NULL,
    due_at                     TIMESTAMPTZ NOT NULL,
    request_id                 UUID,
    request_hash               BYTEA,
    reply_body                 TEXT,
    reply_channel              TEXT,
    reply_received_at          TIMESTAMPTZ,
    reply_recorded_at          TIMESTAMPTZ,
    reply_recorded_by_admin_id BIGINT,
    withdrawn_at               TIMESTAMPTZ,
    withdrawn_by_admin_id      BIGINT,
    withdraw_cause             TEXT,
    CONSTRAINT case_inquiries_case_fkey FOREIGN KEY (case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT case_inquiries_kind_check CHECK (kind IN ('RECORDS', 'EXPLANATION')),
    CONSTRAINT case_inquiries_message_check CHECK (char_length(message) BETWEEN 1 AND 2000),
    CONSTRAINT case_inquiries_due_check CHECK (due_at > created_at),
    CONSTRAINT case_inquiries_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    -- 冪等鍵的範圍是（建立的管理員, request_id），同 measures_request。
    CONSTRAINT case_inquiries_request_key UNIQUE (created_by_admin_id, request_id),
    -- 回覆欄位全有或全無；整個式子要是 TRUE（IS TRUE），理由同 cases_close_check。
    CONSTRAINT case_inquiries_reply_check CHECK ((
        (reply_body IS NULL AND reply_channel IS NULL AND reply_received_at IS NULL AND reply_recorded_at IS NULL AND reply_recorded_by_admin_id IS NULL) OR
        (char_length(reply_body) >= 1 AND char_length(reply_body) <= 4000 AND reply_channel IN ('PHONE', 'EMAIL', 'MESSAGING', 'OTHER')
         AND reply_received_at >= created_at AND reply_recorded_at >= reply_received_at AND reply_recorded_by_admin_id > 0)
    ) IS TRUE),
    CONSTRAINT case_inquiries_withdraw_check CHECK ((
        (withdrawn_at IS NULL AND withdrawn_by_admin_id IS NULL AND withdraw_cause IS NULL) OR
        (withdrawn_at >= created_at AND withdrawn_by_admin_id > 0 AND withdraw_cause IN ('ADMIN', 'CASE_CLOSED'))
    ) IS TRUE),
    CONSTRAINT case_inquiries_terminal_check CHECK (reply_received_at IS NULL OR withdrawn_at IS NULL),
    CONSTRAINT case_inquiries_admin_check CHECK (created_by_admin_id > 0)
);
CREATE UNIQUE INDEX case_inquiries_one_open ON seller.case_inquiries (case_id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL;
CREATE INDEX case_inquiries_case ON seller.case_inquiries (case_id, created_at, id);
-- 3d「詢問逾期」的來源：還沒結束的詢問依期限。
CREATE INDEX case_inquiries_open_due ON seller.case_inquiries (due_at, id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL;

-- 詢問回覆的附件（機敏，同通報佐證）：只在記下回覆的 transaction 寫入，之後只讀；media 的引用以 ref_id = 詢問 ID 登記（GOVERNANCE_EVIDENCE）。
CREATE TABLE seller.case_inquiry_reply_evidence (
    inquiry_id UUID NOT NULL REFERENCES seller.case_inquiries (id),
    position   INTEGER NOT NULL,
    asset_id   TEXT NOT NULL,
    object_key TEXT NOT NULL,
    PRIMARY KEY (inquiry_id, position),
    CONSTRAINT case_inquiry_reply_evidence_position_check CHECK (position BETWEEN 0 AND 4),
    CONSTRAINT case_inquiry_reply_evidence_asset_check CHECK (asset_id <> '' AND object_key <> '')
);

-- 認定（違規）：admin 對一次違規事實的認定，只讀的事實，不帶處分。處分（警告與停權）是措施（measures），一個措施依據一筆或
-- 多筆認定（measure_findings），認定只由建立措施的 transaction 寫入（origin_measure_id 是建立它的措施）。severity 是
-- 套用的嚴重度：帳號 90 天內第 3 次 MEDIUM 升為 HIGH，escalated_from 記下原本的 MEDIUM；升級以 account_id 計（新 stint
-- 繼承舊 stint 的累計）。狀態 ENFORCED → REVOKED（撤銷措施並要求撤銷依據，或接受申覆時一併撤銷）。
CREATE TABLE seller.violations (
    id                  UUID PRIMARY KEY,
    store_id            BIGINT NOT NULL,
    account_id          BIGINT NOT NULL,
    category            TEXT NOT NULL,
    severity            TEXT NOT NULL,
    escalated_from      TEXT,
    status              TEXT NOT NULL DEFAULT 'ENFORCED',
    origin_measure_id   UUID NOT NULL,
    created_by_admin_id BIGINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    revoked_reason      TEXT,
    revoked_by_admin_id BIGINT,
    revoked_at          TIMESTAMPTZ,
    CONSTRAINT violations_store_account_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    -- 政策分類（platform/policy）中平台判定與認定可用的值：全部除了 OTHER（OTHER 只給檢舉人選；含社群類別，內容下架措施的認定用它們）。
    CONSTRAINT violations_category_check CHECK (category IN (
        'FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT',
        'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE'
    )),
    CONSTRAINT violations_severity_check CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT violations_status_check CHECK (status IN ('ENFORCED', 'REVOKED')),
    CONSTRAINT violations_revoked_reason_check CHECK (revoked_reason IN ('APPEAL_ACCEPTED', 'ADMIN_CORRECTION', 'EXTERNAL_DECISION', 'OTHER')),
    CONSTRAINT violations_escalation_check CHECK (escalated_from IS NULL OR (escalated_from = 'MEDIUM' AND severity = 'HIGH')),
    CONSTRAINT violations_revoked_check CHECK (
        (status = 'REVOKED') = (revoked_at IS NOT NULL) AND (revoked_at IS NULL) = (revoked_reason IS NULL) AND
        (revoked_at IS NULL) = (revoked_by_admin_id IS NULL) AND (revoked_at IS NULL OR revoked_at >= created_at)
    ),
    CONSTRAINT violations_admin_check CHECK (created_by_admin_id > 0 AND revoked_by_admin_id > 0)
);
CREATE INDEX violations_created ON seller.violations (created_at DESC, id DESC);
CREATE INDEX violations_store ON seller.violations (store_id, created_at DESC, id DESC);
CREATE INDEX violations_account ON seller.violations (account_id, created_at DESC, id DESC);
-- 升級規則的計數：帳號 90 天內未撤銷、原本是 MEDIUM 的認定。
CREATE INDEX violations_escalation ON seller.violations (account_id, created_at)
    WHERE status = 'ENFORCED' AND COALESCE(escalated_from, severity) = 'MEDIUM';

-- 認定與成立它的通報（多對多，每個認定最多 20 筆，上限由 measure.Create 檢查）：每一筆通報都必須是同一個店家、結果不是未違規、內容不足或濫報
-- （model.Report.Supporting；建立時在鎖住通報列的 transaction 內檢查）。認定只由建立措施的 transaction 寫入，所以這裡也只在那時寫入，之後只讀；通報那邊由 report_id 查導出的認定與措施。
CREATE TABLE seller.finding_reports (
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    report_id    UUID NOT NULL REFERENCES seller.reports (id),
    PRIMARY KEY (violation_id, report_id)
);
CREATE INDEX finding_reports_report ON seller.finding_reports (report_id);

-- 主管機關來函（step10e-reports-brief R5、step10e-3cb-design §2.3）：主管機關給平台的公文，獨立的收件，不是通報。期限與保存都從收文時間（received_at，管理員填）起算；
-- 完成不解除保存。hold_until 是保存期限：指名的帳號或店家在 seller 的個資到這時之前不被 RetireDueStores 的清理（apply_preservation_hold）選中，預設收文後 184 天，
-- 只能延長（authority_order_hold_extensions 每次留一列前後值），等於最後一列的 to_until（沒有延長時是 received_at 加預設值，由模組與資料集測試保證）。
-- version 是完成、延長保存與記下告知的樂觀鎖。request_id 的範圍是（登記的管理員, request_id），同 measures_request。
CREATE TABLE seller.authority_orders (
    id                     UUID PRIMARY KEY,
    agency                 TEXT NOT NULL,
    document_no            TEXT NOT NULL,
    received_at            TIMESTAMPTZ NOT NULL,
    kind                   TEXT NOT NULL,
    summary                TEXT NOT NULL,
    due_at                 TIMESTAMPTZ NOT NULL,
    hold_until             TIMESTAMPTZ NOT NULL,
    registered_at          TIMESTAMPTZ NOT NULL,
    registered_by_admin_id BIGINT NOT NULL,
    completed_at           TIMESTAMPTZ,
    completed_by_admin_id  BIGINT,
    resolution_summary     TEXT,
    version                BIGINT NOT NULL DEFAULT 1,
    request_id             UUID,
    request_hash           BYTEA,
    revoked_at             TIMESTAMPTZ,
    revoked_by_admin_id    BIGINT,
    revoke_reason          TEXT,
    CONSTRAINT authority_orders_kind_check CHECK (kind IN ('RESTRICT_PRODUCT', 'DATA_REQUEST', 'SUSPEND_ACCOUNT', 'OTHER')),
    CONSTRAINT authority_orders_text_check CHECK (
        char_length(agency) BETWEEN 1 AND 100 AND char_length(document_no) BETWEEN 1 AND 100 AND char_length(summary) BETWEEN 1 AND 4000),
    CONSTRAINT authority_orders_times_check CHECK (registered_at >= received_at AND due_at > received_at AND hold_until > received_at),
    CONSTRAINT authority_orders_complete_check CHECK ((
        (completed_at IS NULL AND completed_by_admin_id IS NULL AND resolution_summary IS NULL) OR
        (completed_at >= registered_at AND completed_by_admin_id > 0 AND char_length(resolution_summary) BETWEEN 1 AND 4000)) IS TRUE),
    CONSTRAINT authority_orders_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT authority_orders_request_key UNIQUE (registered_by_admin_id, request_id),
    CONSTRAINT authority_orders_admin_check CHECK (registered_by_admin_id > 0),
    CONSTRAINT authority_orders_version_check CHECK (version >= 1),
    -- 撤銷登記（登記錯誤）：只在完成之前，必附原因；撤銷之後保存解除、不再處理中，紀錄保留。
    CONSTRAINT authority_orders_revoke_check CHECK ((
        (revoked_at IS NULL AND revoked_by_admin_id IS NULL AND revoke_reason IS NULL) OR
        (revoked_at >= registered_at AND revoked_by_admin_id > 0 AND char_length(revoke_reason) BETWEEN 1 AND 2000)) IS TRUE),
    CONSTRAINT authority_orders_terminal_check CHECK (completed_at IS NULL OR revoked_at IS NULL)
);
-- 3d「主管機關來函」與清單 OPEN 的來源：還沒完成也沒撤銷的來函依期限。
CREATE INDEX authority_orders_open_due ON seller.authority_orders (due_at, id) WHERE completed_at IS NULL AND revoked_at IS NULL;
CREATE INDEX authority_orders_received ON seller.authority_orders (received_at DESC, id DESC);

-- 來函指名的對象：任何帳號（帳號是否存在由模組問 identity，不建跨 schema 外鍵），選填店家（NULL＝帳號全部的 stint；有值時複合外鍵保證它屬於帳號，
-- 而且只算那一家，同帳號的新 stint 不在指名內）。(來函, 帳號, 店家) 不可重複（NULLS NOT DISTINCT：不指定店家的帳號也只能列一次）；整個帳號加上它的店家是重疊，
-- 由模組拒絕（DB 保證不了）。保存的判斷（RetireDueStores）依帳號找有效的指名。
-- 告知（Q1 (a)，DSA 9(5)、10(5) 的基準）：完成來函時每個對象記一筆（notice：NOTIFIED 已告知、WITHHELD 機關要求暫不告知並必填確認日期 recheck_on、
-- MEASURE_IS_NOTICE 由措施告知），之後只有 WITHHELD 能改（RecordAuthorityTargetNotice，被取代的值留在 authority_target_notice_history）。
-- 「來函完成 ⇔ 每個對象都有 notice」跨兩張表，由模組在完成的 transaction 內保證並以資料集測試守。
CREATE TABLE seller.authority_order_targets (
    order_id                    UUID NOT NULL REFERENCES seller.authority_orders (id),
    position                    SMALLINT NOT NULL,
    account_id                  BIGINT NOT NULL,
    store_id                    BIGINT,
    product_ids                 TEXT[] NOT NULL DEFAULT '{}',
    notice                      TEXT,
    notified_at                 TIMESTAMPTZ,
    notice_note                 TEXT,
    recheck_on                  DATE,
    notice_recorded_at          TIMESTAMPTZ,
    notice_recorded_by_admin_id BIGINT,
    -- 位置連續（0 到 n-1）的結構保證：位置大於 0 的列必須有前一個位置的列（自我參照的 FK），所以不會有缺號；讀取與告知記錄都以位置為索引。
    prev_position               SMALLINT GENERATED ALWAYS AS (CASE WHEN position > 0 THEN position - 1 END) STORED,
    PRIMARY KEY (order_id, position),
    CONSTRAINT authority_order_targets_prev_fkey FOREIGN KEY (order_id, prev_position) REFERENCES seller.authority_order_targets (order_id, position),
    CONSTRAINT authority_order_targets_store_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    CONSTRAINT authority_order_targets_position_check CHECK (position BETWEEN 0 AND 19),
    CONSTRAINT authority_order_targets_products_check CHECK (cardinality(product_ids) <= 50),
    CONSTRAINT authority_order_targets_account_check CHECK (account_id > 0),
    CONSTRAINT authority_order_targets_notice_check CHECK ((
        (notice IS NULL AND notified_at IS NULL AND notice_note IS NULL AND recheck_on IS NULL AND notice_recorded_at IS NULL AND notice_recorded_by_admin_id IS NULL) OR
        (notice = 'NOTIFIED' AND notified_at IS NOT NULL AND recheck_on IS NULL AND notice_recorded_at IS NOT NULL AND notice_recorded_by_admin_id > 0) OR
        (notice = 'WITHHELD' AND notified_at IS NULL AND recheck_on IS NOT NULL AND notice_recorded_at IS NOT NULL AND notice_recorded_by_admin_id > 0) OR
        (notice = 'MEASURE_IS_NOTICE' AND notified_at IS NULL AND recheck_on IS NULL AND notice_recorded_at IS NOT NULL AND notice_recorded_by_admin_id > 0)) IS TRUE),
    CONSTRAINT authority_order_targets_note_check CHECK (notice_note IS NULL OR char_length(notice_note) BETWEEN 1 AND 2000)
);
CREATE UNIQUE INDEX authority_order_targets_unique ON seller.authority_order_targets (order_id, account_id, store_id) NULLS NOT DISTINCT;
CREATE INDEX authority_order_targets_account ON seller.authority_order_targets (account_id, order_id);
-- 3d「告知被指名者」的來源：暫不告知而且確認日期到了的對象，依確認日期。
CREATE INDEX authority_order_targets_withheld ON seller.authority_order_targets (recheck_on, order_id, position) WHERE notice = 'WITHHELD';

-- 登記時（鎖住被指名帳號的店家列之後）個資已經清理的店家（只新增）：涵蓋狀態 PURGED_BEFORE_REGISTRATION 的事實來源。是登記當下讀到 pii_purged_at 的快照，
-- 與任何應用程式時鐘無關（清理與登記可能在不同實例，時鐘有差）。
CREATE TABLE seller.authority_order_prepurged_stores (
    order_id UUID NOT NULL REFERENCES seller.authority_orders (id),
    store_id BIGINT NOT NULL REFERENCES seller.stores (store_id),
    PRIMARY KEY (order_id, store_id)
);

-- 暫不告知的對象的告知改變時，被取代的值（只新增）：誰、何時改的。只有 WITHHELD 能被取代。
CREATE TABLE seller.authority_target_notice_history (
    order_id             UUID NOT NULL,
    position             SMALLINT NOT NULL,
    seq                  SMALLINT NOT NULL,
    notice               TEXT NOT NULL,
    notice_note          TEXT,
    recheck_on           DATE NOT NULL,
    recorded_at          TIMESTAMPTZ NOT NULL,
    recorded_by_admin_id BIGINT NOT NULL,
    replaced_at          TIMESTAMPTZ NOT NULL,
    replaced_by_admin_id BIGINT NOT NULL,
    PRIMARY KEY (order_id, position, seq),
    CONSTRAINT authority_target_notice_history_target_fkey FOREIGN KEY (order_id, position) REFERENCES seller.authority_order_targets (order_id, position),
    CONSTRAINT authority_target_notice_history_notice_check CHECK (notice = 'WITHHELD' AND seq >= 1 AND replaced_at >= recorded_at),
    CONSTRAINT authority_target_notice_history_admin_check CHECK (recorded_by_admin_id > 0 AND replaced_by_admin_id > 0)
);

-- 保存期限的延長歷史（只新增）：保存期限是來函唯一可變的法律欄位，每次改變留下前後值、原因與作者；DATA_PROVIDED 是調取資料的來函完成時
-- 自動延到「提供之後 184 天」（沒有原因，作者是完成來函的管理員）。
CREATE TABLE seller.authority_order_hold_extensions (
    order_id   UUID NOT NULL REFERENCES seller.authority_orders (id),
    seq        SMALLINT NOT NULL,
    from_until TIMESTAMPTZ NOT NULL,
    to_until   TIMESTAMPTZ NOT NULL,
    cause      TEXT NOT NULL,
    reason     TEXT,
    admin_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (order_id, seq),
    CONSTRAINT authority_order_hold_extensions_order_check CHECK (to_until > from_until AND seq >= 1),
    CONSTRAINT authority_order_hold_extensions_cause_check CHECK ((
        (cause = 'ADMIN' AND char_length(reason) BETWEEN 1 AND 2000) OR (cause = 'DATA_PROVIDED' AND reason IS NULL)) IS TRUE),
    CONSTRAINT authority_order_hold_extensions_admin_check CHECK (admin_id > 0)
);

-- 認定引用來函（比照 finding_reports）：只在建立措施的 transaction 寫入，之後只讀；來函必須指名措施的帳號（模組保證，J4）。
CREATE TABLE seller.finding_authority_orders (
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    order_id     UUID NOT NULL REFERENCES seller.authority_orders (id),
    PRIMARY KEY (violation_id, order_id)
);
CREATE INDEX finding_authority_orders_order ON seller.finding_authority_orders (order_id);

-- 措施（ADR 2026-10-18-seller-governance-consolidated「決策」的「認定、措施與效果」「申訴、補正與凍結」「終止合作」）：admin 對帳號做的決定，
-- 警告（WARNING，沒有限制）、停權（SUSPENSION）或終止合作（TERMINATION，賣家合作條款第 21 條），記在它的店家（store_id）上；效果以帳號
-- （account_id）為單位，新 stint 繼承舊 stint 的停權。措施是停權狀態的唯一來源：「停權是否生效」在讀取時由 status（ENFORCED／REVOKED）、
-- starts_at（開始，不可變）與 ends_at 推導（查詢是 EffectiveMeasures 與 EffectiveMeasuresAll，與 ScheduledMeasures、model.StateOf 由資料集測試鎖住一致），
-- 不存第二份。starts_at：停權與立即生效的終止是 created_at；預告終止（termination_ground = NOTICE，條款 21.2）是 created_at 加 720 小時（30 日，
-- 用固定時數：timestamptz 加日數依 session 時區計算）；ends_at 在預告期間內因店家關閉而寫入時可以早於 starts_at。
-- basis 是依據：VIOLATION 依據至少一筆認定（measure_findings 存全部，是集合，沒有順序；lead_finding_id 是主要依據，建立時由規則決定：有新的
-- 認定時是新的認定，否則是引用的認定中最嚴重的，同嚴重度取最新建立的；可延遲的複合外鍵保證主要依據在其中），RISK 依據條款第 17.1、17.5 條的風險類型（risk_ground）、一定是停權、一定有複審期限 review_by（只給
-- 後台看，建立與重排時落在現在之後 30 天內，不需要 job：到期的進收件匣），CONTRACT 依據契約條款、只配終止合作（預告終止、關聯帳號與沒有
-- 認定的 21.3 事由）。終止合作一定有事由（termination_ground），永遠不是 RISK，沒有複審期限；LINKED_ACCOUNT 一定有來源措施（source_measure_id，
-- 另一個帳號的終止），來源撤銷時連帶撤銷。
-- 版本化採 SQL:2011 的系統版本表模式：這張表存每個會版本化欄位的「目前值」（給賣家的理由 statement_to_seller、結束時間 ends_at；
-- 目前值的唯一來源），measure_revisions 是每一版的不可變快照（含目前那一版）；每次寫入新版本由一個 SQL 語句同時更新這裡與追加快照
-- （AdvanceMeasureRevision、InsertMeasure），current_revision 是目前版本號，經可延遲的複合外鍵指向快照。所以「生效中」的讀取
-- 只看這張表，用部分索引（measures_active_timed、measures_active_open）排除已結束與已撤銷的列。
-- ends_at 空表示沒有固定期間：只有 RISK 的開放式停權與終止合作，警告一定沒有結束時間，依據認定的停權一定有（measures_period_check）。
-- version 是後台樂觀鎖（版本、狀態、review_by、請求狀態改變時加一，寫內部註記不加）；賣家的 etag 由 current_revision 產生。
-- request_id 與 request_hash 是建立的冪等（AIP-155）：範圍 (created_by_admin_id, request_id)，永久保存。
-- source_case_id 是從案件建立的措施所記下的來源案件（任何依據都可以：RISK 停權與不帶認定的終止沒有來源通報，只能這樣連到案件）；案件導出的措施 = source_case_id 是這個案件，
-- 或依據的認定（含引用的既有認定）以這個案件的通報為來源（measure_findings → finding_reports → reports.case_id）。兩處 SQL 的定義由資料集測試鎖住一致。
CREATE TABLE seller.measures (
    id                  UUID PRIMARY KEY,
    account_id          BIGINT NOT NULL,
    store_id            BIGINT NOT NULL,
    kind                TEXT NOT NULL,
    basis               TEXT NOT NULL,
    risk_ground         TEXT,
    lead_finding_id     UUID,
    review_by           TIMESTAMPTZ,
    statement_to_seller TEXT NOT NULL,
    starts_at           TIMESTAMPTZ NOT NULL,
    ends_at             TIMESTAMPTZ,
    termination_ground  TEXT,
    source_measure_id   UUID REFERENCES seller.measures (id),
    source_case_id      UUID,
    status              TEXT NOT NULL DEFAULT 'ENFORCED',
    version             BIGINT NOT NULL DEFAULT 1,
    current_revision    INTEGER NOT NULL,
    request_id          UUID,
    request_hash        BYTEA,
    created_by_admin_id BIGINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    revoked_reason      TEXT,
    revoked_by_admin_id BIGINT,
    revoked_at          TIMESTAMPTZ,
    source_authority_order_id UUID REFERENCES seller.authority_orders (id),
    CONSTRAINT measures_store_account_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    CONSTRAINT measures_source_case_fkey FOREIGN KEY (source_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT measures_kind_check CHECK (kind IN ('WARNING', 'SUSPENSION', 'TERMINATION')),
    CONSTRAINT measures_basis_value_check CHECK (basis IN ('VIOLATION', 'RISK', 'CONTRACT')),
    CONSTRAINT measures_termination_ground_check CHECK (termination_ground IN (
        'NOTICE', 'MATERIAL_BREACH', 'CEASED_BUSINESS', 'DISSOLUTION', 'BANKRUPTCY', 'LICENSE_REVOKED', 'FOOD_SAFETY_INCIDENT', 'FRAUD',
        'PLATFORM_SECURITY', 'OTHER_SERIOUS', 'LINKED_ACCOUNT'
    )),
    CONSTRAINT measures_termination_check CHECK ((kind = 'TERMINATION') = (termination_ground IS NOT NULL)),
    CONSTRAINT measures_contract_check CHECK (basis <> 'CONTRACT' OR kind = 'TERMINATION'),
    CONSTRAINT measures_ground_basis_check CHECK (termination_ground NOT IN ('NOTICE', 'LINKED_ACCOUNT') OR basis = 'CONTRACT'),
    CONSTRAINT measures_breach_check CHECK (termination_ground <> 'MATERIAL_BREACH' OR basis = 'VIOLATION'),
    CONSTRAINT measures_source_check CHECK ((source_measure_id IS NOT NULL) = (termination_ground IS NOT DISTINCT FROM 'LINKED_ACCOUNT')),
    CONSTRAINT measures_starts_check CHECK (
        starts_at >= created_at AND
        (kind = 'TERMINATION' OR starts_at = created_at) AND
        (termination_ground IS DISTINCT FROM 'NOTICE' OR starts_at = created_at + INTERVAL '720 hours') AND
        (termination_ground IS NULL OR termination_ground = 'NOTICE' OR starts_at = created_at)
    ),
    CONSTRAINT measures_ends_check CHECK (ends_at IS NULL OR ends_at > created_at),
    CONSTRAINT measures_risk_ground_check CHECK (risk_ground IN (
        'TRANSACTION', 'FOOD_SAFETY', 'FRAUD', 'PERSONAL_DATA', 'INFRINGEMENT', 'SECURITY', 'FULFILLMENT', 'EXTERNAL_ORDER'
    )),
    CONSTRAINT measures_status_check CHECK (status IN ('ENFORCED', 'REVOKED')),
    CONSTRAINT measures_version_check CHECK (version >= 1),
    CONSTRAINT measures_revision_check CHECK (current_revision >= 1),
    CONSTRAINT measures_basis_check CHECK (
        (basis = 'RISK') = (risk_ground IS NOT NULL) AND (basis = 'VIOLATION') = (lead_finding_id IS NOT NULL) AND
        (basis = 'RISK') = (review_by IS NOT NULL) AND (basis <> 'RISK' OR kind = 'SUSPENSION')
    ),
    CONSTRAINT measures_review_check CHECK (review_by IS NULL OR review_by > created_at),
    CONSTRAINT measures_statement_check CHECK (char_length(statement_to_seller) BETWEEN 1 AND 2000),
    CONSTRAINT measures_period_check CHECK (
        (kind <> 'WARNING' OR ends_at IS NULL) AND (basis <> 'VIOLATION' OR kind <> 'SUSPENSION' OR ends_at IS NOT NULL)
    ),
    CONSTRAINT measures_request_check CHECK (
        (request_id IS NULL) = (request_hash IS NULL) AND (request_hash IS NULL OR octet_length(request_hash) = 32)
    ),
    CONSTRAINT measures_revoked_reason_check CHECK (revoked_reason IN (
        'APPEAL_ACCEPTED', 'ADMIN_CORRECTION', 'EXTERNAL_DECISION', 'OTHER', 'FINDING_REVOKED', 'SOURCE_REVOKED'
    )),
    CONSTRAINT measures_revoked_check CHECK (
        (status = 'REVOKED') = (revoked_at IS NOT NULL) AND (revoked_at IS NULL) = (revoked_reason IS NULL) AND
        (revoked_at IS NULL) = (revoked_by_admin_id IS NULL) AND (revoked_at IS NULL OR revoked_at >= created_at)
    ),
    CONSTRAINT measures_admin_check CHECK (created_by_admin_id > 0 AND revoked_by_admin_id > 0),
    CONSTRAINT measures_id_basis_key UNIQUE (id, basis)
);
CREATE UNIQUE INDEX measures_request ON seller.measures (created_by_admin_id, request_id) WHERE request_id IS NOT NULL;
CREATE INDEX measures_account ON seller.measures (account_id, created_at DESC, id DESC);
CREATE INDEX measures_created ON seller.measures (created_at DESC, id DESC);
-- 指定帳號的生效中停權與終止（EffectiveMeasures）：帳號自己的未撤銷停權或終止（到期的停權也還是 ENFORCED），starts_at、ends_at 帶在索引裡，讓生效與否的條件也是 Index Cond（不然 planner 會改用 measures_account 與兩個全域部分索引合取，讀取量隨全部帳號的生效中措施成長）。
CREATE INDEX measures_enforced_suspension ON seller.measures (account_id, starts_at, ends_at) WHERE status = 'ENFORCED' AND kind IN ('SUSPENSION', 'TERMINATION');
-- 全部帳號的生效中停權與終止（EffectiveMeasuresAll、ScheduledMeasures）的兩段：有固定期間的用 ends_at 範圍掃（ends_at > now 與第二欄 starts_at 的比較都是
-- Index Cond，已結束的排在範圍外）、沒有固定期間的（RISK 的開放式停權與終止合作）整個索引只含未撤銷也未結束的列（結束會寫 ends_at，撤銷會改 status，
-- 兩者都讓列離開索引）。讀取量隨目前仍有效力或預定生效的措施成長，不隨歷史成長；但條款 21.3 的終止永不結束（只能撤銷），所以累計的 21.3 終止都留在
-- measures_active_open、每小時的終止對帳與 suspension_periods 裡，成長的觸發條件與去向見 CARRYOVER.md。
CREATE INDEX measures_active_timed ON seller.measures (ends_at, starts_at) WHERE status = 'ENFORCED' AND kind IN ('SUSPENSION', 'TERMINATION') AND ends_at IS NOT NULL;
CREATE INDEX measures_active_open ON seller.measures (starts_at) WHERE status = 'ENFORCED' AND kind IN ('SUSPENSION', 'TERMINATION') AND ends_at IS NULL;
-- 關聯帳號的終止找它的來源（連帶撤銷），同一個來源對同一個帳號最多一筆生效中的關聯終止。
CREATE INDEX measures_source ON seller.measures (source_measure_id) WHERE source_measure_id IS NOT NULL;
-- 案件導出的措施之一：從案件建立的措施（任何依據），複合外鍵保證是同一家店的案件。另一條是認定的來源通報在案件裡（finding_reports）。
CREATE INDEX measures_source_case ON seller.measures (source_case_id) WHERE source_case_id IS NOT NULL;
-- 來函導出的措施之一：從主管機關來函建立的措施（任何依據），來函必須指名措施的帳號（模組保證）。另一條是依據的認定引用了來函（finding_authority_orders）。
CREATE INDEX measures_source_authority_order ON seller.measures (source_authority_order_id) WHERE source_authority_order_id IS NOT NULL;
CREATE UNIQUE INDEX measures_linked_source ON seller.measures (source_measure_id, account_id) WHERE source_measure_id IS NOT NULL AND status = 'ENFORCED';
-- 複審到期的收件匣。
CREATE INDEX measures_review ON seller.measures (review_by) WHERE status = 'ENFORCED' AND basis = 'RISK';

-- 措施依據的認定：全部依據（含引用既有的認定），是集合，沒有順序（主要依據記在 measures.lead_finding_id）。一筆認定可以被多個措施引用；
-- 它只被建立它的措施（violations.origin_measure_id）撤銷，撤銷時引用它的其他措施連帶撤銷。
CREATE TABLE seller.measure_findings (
    measure_id   UUID NOT NULL REFERENCES seller.measures (id),
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    PRIMARY KEY (measure_id, violation_id)
);
CREATE INDEX measure_findings_violation ON seller.measure_findings (violation_id);

-- 措施的版本歷史（append-only，SQL:2011 系統版本表的歷史表）：每一版的完整快照（給賣家的理由、結束時間；警告沒有結束時間），
-- 含目前那一版，記誰、何時寫的（作者是管理員；店家關閉時結束預告終止（條款 21.2）的版本由系統寫入，沒有管理員，system_cause 記原因，比照
-- measure_requests.close_cause）。每次實際改了就由 AdvanceMeasureRevision 在更新 measures 的同一個語句追加一列（revision 從 1 起連號，
-- 建立措施時由 InsertMeasure 寫第 1 版）。目前值的讀取不經這張表；它只服務版本歷史、申覆引用的版本與 etag 的版本號。
CREATE TABLE seller.measure_revisions (
    measure_id          UUID NOT NULL REFERENCES seller.measures (id),
    revision            INTEGER NOT NULL,
    statement_to_seller TEXT NOT NULL,
    ends_at             TIMESTAMPTZ,
    admin_id            BIGINT,
    system_cause        TEXT,
    created_at          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (measure_id, revision),
    CONSTRAINT measure_revisions_revision_check CHECK (revision >= 1),
    CONSTRAINT measure_revisions_statement_check CHECK (char_length(statement_to_seller) BETWEEN 1 AND 2000),
    CONSTRAINT measure_revisions_admin_check CHECK (admin_id > 0),
    CONSTRAINT measure_revisions_system_cause_check CHECK (system_cause IN ('STORE_CLOSED')),
    CONSTRAINT measure_revisions_author_check CHECK ((system_cause IS NULL) = (admin_id IS NOT NULL))
);
ALTER TABLE seller.measures
    ADD CONSTRAINT measures_current_revision_fkey FOREIGN KEY (id, current_revision)
        REFERENCES seller.measure_revisions (measure_id, revision) DEFERRABLE INITIALLY DEFERRED,
    ADD CONSTRAINT measures_lead_finding_fkey FOREIGN KEY (id, lead_finding_id)
        REFERENCES seller.measure_findings (measure_id, violation_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE seller.violations
    ADD CONSTRAINT violations_origin_measure_fkey FOREIGN KEY (origin_measure_id, id)
        REFERENCES seller.measure_findings (measure_id, violation_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE seller.store_closures
    ADD CONSTRAINT store_closures_measure_fkey FOREIGN KEY (closed_by_measure_id) REFERENCES seller.measures (id),
    ADD CONSTRAINT store_closures_current_fkey FOREIGN KEY (store_id, open_closed_at) REFERENCES seller.stores (store_id, current_closed_at) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE seller.stores
    ADD CONSTRAINT stores_current_closure_fkey FOREIGN KEY (store_id, current_closed_at) REFERENCES seller.store_closures (store_id, open_closed_at) DEFERRABLE INITIALLY DEFERRED;

-- 治理的內部註記（append-only）：平台人員之間的說明時間軸，賣家看不到。掛在措施（measure_id）或案件（case_id），剛好一個；每則記作者、時間與內容，不能改也不能刪，
-- 更正就再寫一則；寫註記不動 measures 與 cases（不加 version），所以賣家依舊版本申覆、後台依舊 etag 的寫入不會被註記擋下。
-- 改案件嚴重度時寫的那則註記（原因）帶改前與改後（severity_from、severity_to），只有案件的註記可以有。
CREATE TABLE seller.governance_notes (
    note_id       UUID PRIMARY KEY,
    measure_id    UUID REFERENCES seller.measures (id),
    case_id       UUID REFERENCES seller.cases (id),
    admin_id      BIGINT NOT NULL,
    body          TEXT NOT NULL,
    severity_from TEXT,
    severity_to   TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT governance_notes_body_check CHECK (char_length(body) BETWEEN 1 AND 2000),
    CONSTRAINT governance_notes_admin_check CHECK (admin_id > 0),
    CONSTRAINT governance_notes_owner_check CHECK (num_nonnulls(case_id, measure_id) = 1),
    CONSTRAINT governance_notes_severity_check CHECK (
        (severity_from IS NULL) = (severity_to IS NULL) AND
        (severity_from IS NULL OR (
            case_id IS NOT NULL AND severity_from <> severity_to AND
            severity_from IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL') AND severity_to IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')
        ))
    )
);
CREATE INDEX governance_notes_measure ON seller.governance_notes (measure_id, created_at, note_id);
CREATE INDEX governance_notes_case ON seller.governance_notes (case_id, created_at, note_id) WHERE case_id IS NOT NULL;

-- 措施上賣家的請求：申覆（APPEAL，條款 18.4）與解除請求（REMEDIATION，條款 17.4，只限依據風險的措施）。revision 是提出時措施的那一版，
-- 之後 admin 再改也不變。basis 是措施依據的冗餘，複合外鍵 (measure_id, basis) 讓它跟著措施，CHECK 以它保證解除請求只掛在風險措施上。
-- 每個措施至多一次申覆（measure_requests_one_appeal）、同時至多一筆待處理的請求（measure_requests_one_pending，不分種類）。
-- 裁定（接受或駁回）一定附理由；請求以 CLOSED 結案時不帶理由，close_cause 記原因：REVOKED 是措施撤銷（一定有 admin），ENDED 是措施
-- 已結束而解除請求失去標的（可以是系統結案，沒有 admin）。
CREATE TABLE seller.measure_requests (
    id                  UUID PRIMARY KEY,
    measure_id          UUID NOT NULL,
    kind                TEXT NOT NULL,
    state               TEXT NOT NULL,
    reason              TEXT NOT NULL,
    revision            INTEGER NOT NULL,
    submitted_at        TIMESTAMPTZ NOT NULL,
    decision_reason     TEXT,
    decided_by_admin_id BIGINT,
    decided_at          TIMESTAMPTZ,
    basis               TEXT NOT NULL,
    close_cause         TEXT,
    CONSTRAINT measure_requests_measure_basis_fkey FOREIGN KEY (measure_id, basis) REFERENCES seller.measures (id, basis),
    CONSTRAINT measure_requests_revision_fkey FOREIGN KEY (measure_id, revision)
        REFERENCES seller.measure_revisions (measure_id, revision),
    CONSTRAINT measure_requests_kind_check CHECK (kind IN ('APPEAL', 'REMEDIATION')),
    CONSTRAINT measure_requests_state_check CHECK (state IN ('PENDING', 'ACCEPTED', 'REJECTED', 'CLOSED')),
    CONSTRAINT measure_requests_close_cause_check CHECK (close_cause IN ('REVOKED', 'ENDED')),
    CONSTRAINT measure_requests_remediation_basis_check CHECK (kind <> 'REMEDIATION' OR basis = 'RISK'),
    CONSTRAINT measure_requests_reason_check CHECK (char_length(reason) BETWEEN 1 AND 2000),
    CONSTRAINT measure_requests_decision_check CHECK (
        (state = 'PENDING' AND decision_reason IS NULL AND decided_by_admin_id IS NULL AND decided_at IS NULL AND close_cause IS NULL) OR
        (state IN ('ACCEPTED', 'REJECTED') AND decision_reason IS NOT NULL AND char_length(decision_reason) BETWEEN 1 AND 2000 AND
         decided_by_admin_id IS NOT NULL AND decided_by_admin_id > 0 AND decided_at IS NOT NULL AND decided_at >= submitted_at AND
         close_cause IS NULL) OR
        (state = 'CLOSED' AND decision_reason IS NULL AND close_cause IS NOT NULL AND (decided_by_admin_id IS NULL OR decided_by_admin_id > 0) AND
         (close_cause <> 'REVOKED' OR decided_by_admin_id IS NOT NULL) AND decided_at IS NOT NULL AND decided_at >= submitted_at)
    )
);
CREATE UNIQUE INDEX measure_requests_one_appeal ON seller.measure_requests (measure_id) WHERE kind = 'APPEAL';
CREATE UNIQUE INDEX measure_requests_one_pending ON seller.measure_requests (measure_id) WHERE state = 'PENDING';
CREATE INDEX measure_requests_pending ON seller.measure_requests (submitted_at, id) WHERE state = 'PENDING';

-- 請求的佐證圖片：值的集合，隨請求一次寫入；media 的引用以 ref_id = 請求 ID 登記（GOVERNANCE_EVIDENCE）。
CREATE TABLE seller.measure_request_evidence (
    request_id UUID NOT NULL REFERENCES seller.measure_requests (id),
    position   INTEGER NOT NULL,
    asset_id   TEXT NOT NULL,
    object_key TEXT NOT NULL,
    PRIMARY KEY (request_id, position),
    CONSTRAINT measure_request_evidence_position_check CHECK (position >= 0 AND position < 5),
    CONSTRAINT measure_request_evidence_asset_check CHECK (asset_id <> '' AND object_key <> '')
);

-- 佐證的讀取紀錄（通報、請求與詢問回覆共用，append-only）：佐證是機敏檔，後台每讀一張，在讀取之前寫一列（誰、哪一張、何時；不含內容）。
-- 通報的佐證以 (report_id, submission_seq, position)、請求的佐證以 (request_id, position)、詢問回覆的附件以 (inquiry_id, position) 指向佐證列本身（剛好一個）：
-- 佐證寫入後不可改、不可刪，那一列就是當時讀的檔。
CREATE TABLE seller.evidence_accesses (
    access_id      UUID PRIMARY KEY,
    report_id      UUID,
    -- 通報的佐證才有（第幾次送出）；請求與詢問的佐證沒有。
    submission_seq SMALLINT,
    request_id     UUID,
    inquiry_id     UUID,
    position       INTEGER NOT NULL,
    admin_id       BIGINT NOT NULL,
    accessed_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT evidence_accesses_report_fkey FOREIGN KEY (report_id, submission_seq, position) REFERENCES seller.report_evidence (report_id, submission_seq, position),
    CONSTRAINT evidence_accesses_seq_check CHECK ((report_id IS NULL) = (submission_seq IS NULL)),
    CONSTRAINT evidence_accesses_request_fkey FOREIGN KEY (request_id, position) REFERENCES seller.measure_request_evidence (request_id, position),
    CONSTRAINT evidence_accesses_inquiry_fkey FOREIGN KEY (inquiry_id, position) REFERENCES seller.case_inquiry_reply_evidence (inquiry_id, position),
    CONSTRAINT evidence_accesses_owner_check CHECK (num_nonnulls(report_id, request_id, inquiry_id) = 1),
    CONSTRAINT evidence_accesses_admin_check CHECK (admin_id > 0)
);
CREATE INDEX evidence_accesses_report ON seller.evidence_accesses (report_id, accessed_at DESC) WHERE report_id IS NOT NULL;
CREATE INDEX evidence_accesses_request ON seller.evidence_accesses (request_id, accessed_at DESC) WHERE request_id IS NOT NULL;
CREATE INDEX evidence_accesses_inquiry ON seller.evidence_accesses (inquiry_id, accessed_at DESC) WHERE inquiry_id IS NOT NULL;

-- 停權期間：帳號從未停權變成停權到恢復的一段，是停權事件的身分（StoreSuspended 與 StoreUnsuspended 帶同一個
-- period_id；Java 的商品下架快照以它為 scope）。只由 suspension.Recompute 開始與結束，與造成停權的措施無關：
-- 措施重疊或交接時仍是同一段。期間記在帳號的目前 stint 上（帳號停權時 stint 關閉的話就留在那個 stint，沒有目前 stint
-- 時不開始）；每個店家同時最多一段進行中。
CREATE TABLE seller.suspension_periods (
    period_id  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES seller.stores (store_id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at   TIMESTAMPTZ,
    CONSTRAINT suspension_periods_ended_check CHECK (ended_at IS NULL OR ended_at >= started_at)
);
CREATE UNIQUE INDEX suspension_periods_open ON seller.suspension_periods (store_id) WHERE ended_at IS NULL;

-- 措施的身分快照（ADR 2026-10-19-seller-kyc-payuni-consolidated「身分比對與比對 hash」）：停權措施建立時，把被處分帳號全部
-- KYC 申請（跨 stint）的身分 blind index 複製過來，措施結束或撤銷時刪除；KYC 列的個資照常清理，這裡是措施擁有的、
-- 唯一保留的比對依據。kind 是比對的種類（身分證號與企業負責人身分證號同一種：同一個人不論以個人或負責人入駐都算），
-- key_id 是算出 hash 的 HMAC 金鑰：快照沒有明文可以重算，所以這把金鑰要留在金鑰環直到措施結束。比對只讀這張表，
-- 「生效中」的判斷一律經 stint.Derive（不在這裡再寫一份）。
CREATE TABLE seller.measure_identities (
    measure_id UUID NOT NULL REFERENCES seller.measures (id),
    kind       TEXT NOT NULL,
    key_id     TEXT NOT NULL,
    hash       BYTEA NOT NULL,
    PRIMARY KEY (measure_id, kind, hash),
    CONSTRAINT measure_identities_kind_check CHECK (kind IN ('ID_CARD', 'TAX_ID', 'BANK_ACCOUNT')),
    CONSTRAINT measure_identities_key_check CHECK (key_id <> ''),
    CONSTRAINT measure_identities_hash_check CHECK (octet_length(hash) = 32)
);
CREATE INDEX measure_identities_hash ON seller.measure_identities (hash);

-- KYC 認證申請（ADR 2026-10-19-seller-kyc-payuni-consolidated）：以 store_id 為鍵，一個店家同時最多一筆還沒取消的申請
-- （kyc_applications_open；被退回的重送沿用原列，狀態回到 DRAFT）。草稿與送出共用同一組具型別的欄位（沒有自由格式的
-- JSON），草稿時除了身分以外都可以是 NULL，送出之後必填由 kyc_applications_complete_check 表達。status 只記人工決定
-- （草稿、送出、核准、退回、取消）；PayUni 的綁定狀態在 7c 的後續階段，不混進這條軸。ID 是 UUIDv7（應用層產生）：申請
-- 是 media 引用、事件與稽核指稱的實體，Java 不存它（row-identity ADR）。
-- 個資：id_card_no、rep_id_card_no（企業負責人）與 bank_account 是 AES-256-GCM 的密文（platform/seal，AAD 是申請 ID
-- 加欄位名），real_name 與 birth_date 明文（後台以姓名模糊搜尋，ADR 2026-07-20）。每個識別欄位另有一個 blind index
-- （HMAC，輸入帶欄位標籤；*_hash_key 是算出它的金鑰 ID），給處分的身分快照用。送給 PayUni 的聯絡資料（聯絡人、聯絡手機、
-- 客服信箱）是店家的欄位（stores），不在申請上。公司地址以內政部的鄉鎮市區代碼表示（同 stores）。
-- version 是樂觀鎖（etag 的來源）：所有寫入者必帶讀到的版本。
-- 個資清理：店家關閉滿保留期後 purge 套件清掉全部個人欄位（密文、blind index、姓名、生日、證件資訊、聯絡方式、公司與
-- 地址），記下 pii_purged_at；審核的事實（狀態、時間、審核紀錄）與不具名的欄位（seller_type、product_type）保留。kyc_applications_purged_check 列出被清掉的欄位：清理過的列這些一定是 NULL。處分的身分快照不在這裡
-- （measure_identities，隨措施結束才刪），所以清理不影響身分比對。
CREATE TABLE seller.kyc_applications (
    application_id          UUID PRIMARY KEY,
    store_id                BIGINT NOT NULL REFERENCES seller.stores (store_id),
    status                  TEXT NOT NULL DEFAULT 'DRAFT',
    version                 BIGINT NOT NULL DEFAULT 1,
    seller_type             TEXT NOT NULL,
    real_name               TEXT,
    birth_date              DATE,
    id_card_no_enc          BYTEA,
    id_card_hash            BYTEA,
    id_card_hash_key        TEXT,
    id_card_issue_date      DATE,
    id_card_issue_type      TEXT,
    id_card_issue_location  TEXT,
    english_name            TEXT,
    bank_name               TEXT,
    bank_code               TEXT,
    bank_account_enc        BYTEA,
    bank_account_hash       BYTEA,
    bank_account_hash_key   TEXT,
    product_type            TEXT,
    website_url             TEXT,
    company_name            TEXT,
    tax_id                  TEXT,
    tax_id_hash             BYTEA,
    tax_id_hash_key         TEXT,
    company_register_date   DATE,
    company_town_code       TEXT,
    company_address_detail  TEXT,
    company_phone           TEXT,
    rep_id_card_no_enc      BYTEA,
    rep_id_card_hash        BYTEA,
    rep_id_card_hash_key    TEXT,
    submitted_at            TIMESTAMPTZ,
    submission_seq          BIGINT NOT NULL DEFAULT 0,
    decided_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    pii_purged_at           TIMESTAMPTZ,
    CONSTRAINT kyc_applications_status_check CHECK (status IN ('DRAFT', 'SUBMITTED', 'APPROVED', 'REJECTED', 'CANCELLED')),
    CONSTRAINT kyc_applications_seller_type_check CHECK (seller_type IN ('INDIVIDUAL', 'COMPANY')),
    CONSTRAINT kyc_applications_version_check CHECK (version >= 1),
    CONSTRAINT kyc_applications_submission_seq_check CHECK (submission_seq >= 0 AND (status IN ('DRAFT', 'CANCELLED') OR submission_seq >= 1)),
    CONSTRAINT kyc_applications_real_name_check CHECK (char_length(real_name) >= 1 AND char_length(real_name) <= 100 AND real_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT kyc_applications_birth_date_check CHECK (birth_date >= DATE '1900-01-01'),
    CONSTRAINT kyc_applications_issue_date_check CHECK (id_card_issue_date >= DATE '1900-01-01'),
    CONSTRAINT kyc_applications_issue_type_check CHECK (char_length(id_card_issue_type) BETWEEN 1 AND 20),
    CONSTRAINT kyc_applications_issue_location_check CHECK (char_length(id_card_issue_location) BETWEEN 1 AND 100),
    CONSTRAINT kyc_applications_english_name_check CHECK (char_length(english_name) BETWEEN 1 AND 50),
    CONSTRAINT kyc_applications_bank_name_check CHECK (char_length(bank_name) BETWEEN 1 AND 100),
    CONSTRAINT kyc_applications_bank_code_check CHECK (bank_code ~ '^[0-9]{3}([0-9]{4})?$'),
    CONSTRAINT kyc_applications_product_type_check CHECK (char_length(product_type) BETWEEN 1 AND 100),
    CONSTRAINT kyc_applications_website_check CHECK (website_url ~ '^https?://[^/?#]+' AND char_length(website_url) <= 500),
    CONSTRAINT kyc_applications_company_name_check CHECK (char_length(company_name) BETWEEN 1 AND 200),
    CONSTRAINT kyc_applications_tax_id_check CHECK (tax_id ~ '^[0-9]{8}$'),
    CONSTRAINT kyc_applications_company_register_date_check CHECK (company_register_date >= DATE '1900-01-01'),
    CONSTRAINT kyc_applications_company_town_check CHECK (company_town_code ~ '^[0-9]{8}$'),
    CONSTRAINT kyc_applications_company_address_check CHECK (
        company_address_detail IS NULL OR (company_town_code IS NOT NULL AND char_length(company_address_detail) BETWEEN 1 AND 255)
    ),
    CONSTRAINT kyc_applications_company_phone_check CHECK (company_phone ~ '^0[0-9]{8,9}$'),
    -- 密文、blind index 與它的金鑰 ID 成組：有值就一起有、一起沒有。
    CONSTRAINT kyc_applications_id_card_check CHECK (
        (id_card_no_enc IS NULL) = (id_card_hash IS NULL) AND (id_card_hash IS NULL) = (id_card_hash_key IS NULL)
    ),
    CONSTRAINT kyc_applications_rep_id_card_check CHECK (
        (rep_id_card_no_enc IS NULL) = (rep_id_card_hash IS NULL) AND (rep_id_card_hash IS NULL) = (rep_id_card_hash_key IS NULL)
    ),
    CONSTRAINT kyc_applications_bank_account_check CHECK (
        (bank_account_enc IS NULL) = (bank_account_hash IS NULL) AND (bank_account_hash IS NULL) = (bank_account_hash_key IS NULL)
    ),
    CONSTRAINT kyc_applications_tax_id_hash_check CHECK ((tax_id IS NULL) = (tax_id_hash IS NULL) AND (tax_id_hash IS NULL) = (tax_id_hash_key IS NULL)),
    CONSTRAINT kyc_applications_hash_length_check CHECK (
        octet_length(id_card_hash) = 32 AND octet_length(rep_id_card_hash) = 32 AND octet_length(bank_account_hash) = 32 AND
        octet_length(tax_id_hash) = 32
    ),
    -- 個人只有自己的身分證號，公司只有負責人的；公司欄位只屬於公司。
    CONSTRAINT kyc_applications_type_fields_check CHECK (
        (seller_type = 'INDIVIDUAL' AND company_name IS NULL AND tax_id IS NULL AND
         company_register_date IS NULL AND company_town_code IS NULL AND company_address_detail IS NULL AND
         company_phone IS NULL AND rep_id_card_no_enc IS NULL) OR
        (seller_type = 'COMPANY' AND id_card_no_enc IS NULL)
    ),
    -- 送出之後（含核准、退回）的必填；草稿可以缺。
    CONSTRAINT kyc_applications_complete_check CHECK (
        status NOT IN ('SUBMITTED', 'APPROVED', 'REJECTED') OR (
            submitted_at IS NOT NULL AND (pii_purged_at IS NOT NULL OR (
                real_name IS NOT NULL AND english_name IS NOT NULL AND bank_name IS NOT NULL AND
                bank_code IS NOT NULL AND bank_account_enc IS NOT NULL AND
                ((seller_type = 'INDIVIDUAL' AND id_card_no_enc IS NOT NULL) OR
                 (seller_type = 'COMPANY' AND company_name IS NOT NULL AND tax_id IS NOT NULL AND rep_id_card_no_enc IS NOT NULL))
            ))
        )
    ),
    CONSTRAINT kyc_applications_purged_check CHECK (
        pii_purged_at IS NULL OR (
            real_name IS NULL AND birth_date IS NULL AND id_card_no_enc IS NULL AND id_card_hash IS NULL AND id_card_hash_key IS NULL AND
            id_card_issue_date IS NULL AND id_card_issue_type IS NULL AND id_card_issue_location IS NULL AND english_name IS NULL AND
            bank_name IS NULL AND bank_code IS NULL AND bank_account_enc IS NULL AND bank_account_hash IS NULL AND
            bank_account_hash_key IS NULL AND website_url IS NULL AND
            company_name IS NULL AND tax_id IS NULL AND tax_id_hash IS NULL AND tax_id_hash_key IS NULL AND
            company_register_date IS NULL AND company_town_code IS NULL AND company_address_detail IS NULL AND company_phone IS NULL AND
            rep_id_card_no_enc IS NULL AND rep_id_card_hash IS NULL AND rep_id_card_hash_key IS NULL
        )
    ),
    CONSTRAINT kyc_applications_decided_check CHECK ((status IN ('APPROVED', 'REJECTED')) = (decided_at IS NOT NULL))
);
-- 一個店家同時最多一筆還沒取消的申請（草稿、審核中、核准、退回）；取消之後可以重新建立。
CREATE UNIQUE INDEX kyc_applications_open ON seller.kyc_applications (store_id) WHERE status <> 'CANCELLED';
CREATE INDEX kyc_applications_store ON seller.kyc_applications (store_id, created_at DESC, application_id DESC);
-- admin 清單（審核佇列）只列送出過的申請（不含草稿，也不含沒送出就取消的），排序的鍵是送出時間：篩選審核中時舊到新
-- （等最久的在最前面），其他（含沒有篩選）新到舊；兩個方向走同一個索引。
CREATE INDEX kyc_applications_status_submitted ON seller.kyc_applications (status, submitted_at, application_id)
    WHERE status <> 'DRAFT' AND submitted_at IS NOT NULL;
CREATE INDEX kyc_applications_submitted ON seller.kyc_applications (submitted_at, application_id)
    WHERE status <> 'DRAFT' AND submitted_at IS NOT NULL;

-- 申請的身分比對索引：每個識別欄位（身分證號、企業負責人身分證號、統編、銀行帳號）用金鑰環每一把金鑰算的 blind index 各一列，所以措施的身分快照（同樣存每把金鑰的 hash）
-- 與申請以 (key_id, hash) 等值連接就能反查同身分的帳號，不必解密（ADR 2026-10-19-seller-kyc-payuni-consolidated「身分比對與比對 hash」）。新增金鑰（輪替）之後，
-- 既有申請缺新金鑰的列，由 seller 模組的 RehashKYCIdentities 在啟動時補算（只解密缺列的申請）；舊金鑰的列不會被刪，只隨個資清理整列刪除，啟動檢查 CheckKYCKeys 要求被參照的金鑰都在金鑰環裡。
-- kyc_applications 上的 *_hash 與 *_hash_key 是寫入當下第一把金鑰的值，只給草稿的「欄位有沒有改」判斷用。個資清理時整列刪除。
CREATE TABLE seller.kyc_identity_hashes (
    application_id UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    field          TEXT NOT NULL,
    key_id         TEXT NOT NULL,
    hash           BYTEA NOT NULL,
    PRIMARY KEY (application_id, field, key_id),
    CONSTRAINT kyc_identity_hashes_field_check CHECK (field IN ('ID_CARD_NO', 'REP_ID_CARD_NO', 'TAX_ID', 'BANK_ACCOUNT')),
    CONSTRAINT kyc_identity_hashes_key_check CHECK (key_id <> ''),
    CONSTRAINT kyc_identity_hashes_hash_check CHECK (octet_length(hash) = 32)
);
CREATE INDEX kyc_identity_hashes_hash ON seller.kyc_identity_hashes (hash, key_id);

-- 證件與文件圖片：身分證正反面、存摺封面各一張（position 0），公司文件最多 10 張。media 的引用以
-- ref_id = application_id 登記（CERTIFICATION），存 asset ID 是同步 job 登記引用時業務列目前指著的資產。
CREATE TABLE seller.kyc_documents (
    application_id UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    slot           TEXT NOT NULL,
    position       INTEGER NOT NULL DEFAULT 0,
    asset_id       TEXT NOT NULL,
    object_key     TEXT NOT NULL,
    PRIMARY KEY (application_id, slot, position),
    CONSTRAINT kyc_documents_slot_check CHECK (slot IN ('ID_CARD_FRONT', 'ID_CARD_BACK', 'BANK_BOOK', 'COMPANY_DOCUMENT')),
    CONSTRAINT kyc_documents_position_check CHECK (position >= 0 AND position < 10 AND (slot = 'COMPANY_DOCUMENT' OR position = 0)),
    CONSTRAINT kyc_documents_asset_check CHECK (asset_id <> '' AND object_key <> '')
);

-- 公司型申請送件時對經濟部商業司公開資料的查詢快照（ADR 2026-10-19-seller-kyc-payuni-consolidated「登記資料快照」）：審核時對照的
-- 是送件當時查到的登記資料，不是審核當下才查（審核紀錄引用這一列，事後才追得到依據）。一次查詢一列：送件與後台的重新查詢
-- 都新增一列 PENDING，job 查完寫入結果；結果寫入後不再改（寫入的查詢以仍是 PENDING 為條件；舊的查詢比新的晚完成也照樣寫入）。
-- submission_seq 是查詢所屬的那一輪送件（申請的 submission_seq：每次 SubmitKYC 加一；退回後重送沿用同一筆申請，所以要有輪次）：
-- 顯示的查詢是申請目前這一輪、seq 最大的已完成列，上一輪的快照不會顯示成這一輪的依據。seq 是同一申請內的查詢序號（遞增，新增的人
-- 持有申請列鎖；後台 etag 以它組成，所以要有序的整數），check_id 是 UUIDv7（應用層產生，審核紀錄引用它）：兩者並存，一個給順序、
-- 一個給身分。PENDING 的出口：job 寫入結果、認領時放棄（CANCELLED）、或後台重新查詢把這一輪的 PENDING 全部取代（CANCELLED）——
-- 所以任何原因（程序當掉、job 被丟棄、重試用盡前就被取消）留下的 PENDING，都能由重新查詢收掉。
-- request_kind 是誰觸發：SUBMIT 是賣家送件（沒有管理員），ADMIN_REQUERY 是後台重新查詢（記管理員 ID，顯示名稱由 client 經
-- identity 查，與 kyc_reviews 一致）。outcome：PENDING 還沒查完；FOUND 查到；NOT_FOUND 兩個資料集都查無；UNAVAILABLE 來源
-- 重試到最後一次仍不可用（或呼叫方取消、沒有分類的錯誤、來源拒絕統編格式）；CANCELLED 是放棄的查詢（認領時申請已經不是這一輪
-- 審核中、已不是公司型、同一輪已有更新的查詢，或被後台的重新查詢取代）：終態，不顯示、不比對。只有 FOUND 帶登記資料與比對結果：mismatched_fields 是申請與登記資料不一致的欄位、
-- unverifiable_fields 是無法比對的欄位（來源缺漏或地址拆不開），值是 KYCFieldNames 的路徑；比對只記錄，不改申請狀態。
-- status 是登記現況的分類（ACTIVE、ORDERED_TO_DISSOLVE、TERMINATED、UNKNOWN；UNKNOWN 時 status_text 的原文是唯一依據）；
-- 停業期間另存，是否涵蓋某個時間點由顯示端依 queried_at 判斷，不凍結進快照。地址：address_text 是來源原文，town_code 與
-- address_detail 是拆得開時的結果（拆不開時兩者都是 NULL）。
-- 查詢是有期限的資源（AIP-151：狀態在資源上、終態由伺服器保證）：deadline_at 在建立時寫入（requested_at 加 15 分鐘；GCIS 沒有 SLA，
-- 互動情境要幾分鐘內有結果，逾時收成 UNAVAILABLE 讓人重新查詢），後台的 pending_registration_check.deadline_time 讀它。job 執行時
-- 已過期限就直接記 UNAVAILABLE、不呼叫資料來源；掃描 job 以它收掉還在 PENDING 的查詢（見 kyc_registration_checks_pending）。
-- 個資清理：登記資料含公司名稱與地址，與申請的個資同時清除（purge 套件刪除該申請的全部列；kyc_reviews 的引用隨之置 NULL）。
CREATE TABLE seller.kyc_registration_checks (
    check_id               UUID PRIMARY KEY,
    application_id         UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    seq                    BIGINT NOT NULL,
    request_kind           TEXT NOT NULL,
    requested_by_admin_id  BIGINT,
    requested_at           TIMESTAMPTZ NOT NULL,
    deadline_at            TIMESTAMPTZ NOT NULL,
    submission_seq         BIGINT NOT NULL,
    outcome                TEXT NOT NULL DEFAULT 'PENDING',
    queried_at             TIMESTAMPTZ,
    tax_id                 TEXT,
    name                   TEXT,
    address_text           TEXT,
    town_code              TEXT,
    address_detail         TEXT,
    setup_date             DATE,
    organization_type      TEXT,
    status                 TEXT,
    status_text            TEXT,
    suspended_from         DATE,
    suspended_until        DATE,
    mismatched_fields      TEXT[] NOT NULL DEFAULT '{}',
    unverifiable_fields    TEXT[] NOT NULL DEFAULT '{}',
    CONSTRAINT kyc_registration_checks_seq_key UNIQUE (application_id, seq),
    CONSTRAINT kyc_registration_checks_seq_check CHECK (seq >= 1),
    CONSTRAINT kyc_registration_checks_deadline_check CHECK (deadline_at > requested_at),
    CONSTRAINT kyc_registration_checks_submission_seq_check CHECK (submission_seq >= 1),
    CONSTRAINT kyc_registration_checks_kind_check CHECK (
        (request_kind = 'SUBMIT' AND requested_by_admin_id IS NULL) OR
        (request_kind = 'ADMIN_REQUERY' AND requested_by_admin_id IS NOT NULL AND requested_by_admin_id > 0)
    ),
    CONSTRAINT kyc_registration_checks_outcome_check CHECK (outcome IN ('PENDING', 'FOUND', 'NOT_FOUND', 'UNAVAILABLE', 'CANCELLED')),
    CONSTRAINT kyc_registration_checks_queried_check CHECK ((outcome = 'PENDING') = (queried_at IS NULL)),
    CONSTRAINT kyc_registration_checks_found_check CHECK (
        (outcome = 'FOUND' AND tax_id IS NOT NULL AND tax_id ~ '^[0-9]{8}$' AND name IS NOT NULL AND char_length(name) >= 1
            AND address_text IS NOT NULL AND status IS NOT NULL AND status IN ('ACTIVE', 'ORDERED_TO_DISSOLVE', 'TERMINATED', 'UNKNOWN') AND status_text IS NOT NULL) OR
        (outcome <> 'FOUND' AND tax_id IS NULL AND name IS NULL AND address_text IS NULL AND town_code IS NULL
            AND address_detail IS NULL AND setup_date IS NULL AND organization_type IS NULL AND status IS NULL
            AND status_text IS NULL AND suspended_from IS NULL AND suspended_until IS NULL
            AND mismatched_fields = '{}' AND unverifiable_fields = '{}')
    ),
    CONSTRAINT kyc_registration_checks_town_check CHECK (
        (town_code IS NULL) = (address_detail IS NULL) AND (town_code IS NULL OR town_code ~ '^[0-9]{8}$')
    ),
    CONSTRAINT kyc_registration_checks_suspension_check CHECK (suspended_from IS NULL OR suspended_until IS NULL OR suspended_until >= suspended_from),
    CONSTRAINT kyc_registration_checks_fields_check CHECK (
        mismatched_fields <@ ARRAY['business.company_name', 'business.company_address', 'business.company_register_date']::text[] AND
        unverifiable_fields <@ ARRAY['business.company_name', 'business.company_address', 'business.company_register_date']::text[] AND
        NOT (mismatched_fields && unverifiable_fields)
    )
);
-- 掃描 job 的兩支查詢（過期的收成終態、閒置的補排）只看還在 PENDING 的列，並以 deadline_at 判斷；PENDING 是少數，部分索引很小
-- （payuni_registrations_unsynced 的先例）。
CREATE INDEX kyc_registration_checks_pending ON seller.kyc_registration_checks (deadline_at) WHERE outcome = 'PENDING';

-- 審核紀錄：append-only（只有 INSERT 的查詢），記人工決定的事實（誰、何時、什麼決定、理由），不含任何欄位值。
-- review_id 同時是對應模組事件的 event_id，relay 重送時不變。reviewer_admin_id 在賣家自己的動作（送出、取消）是 NULL；
-- 管理員只記 ID，顯示名稱由 client 經 identity 查（與關店、停權的管理員欄位一致）。
-- REBIND 是管理員對這份申請的 PayUni 註冊重新綁定（誰、何時；沒有事件）。
-- internal_note 是退回時審核員寫的內部備註（只有 REJECT 有，不給賣家看）。逐項問題在 kyc_review_issues。
-- registration_check_id 是核准或退回當下後台顯示的登記資料查詢（審核的依據；沒有查詢、查詢還沒完成或不可用時是 NULL）。個資清理
-- 刪掉查詢列時引用置 NULL（ON DELETE SET NULL）：審核紀錄本身保留，不需要為了它對 append-only 的表另寫 UPDATE；外鍵的動作要
-- 能由被刪的查詢找到引用它的審核紀錄，所以有 kyc_reviews_registration_check 的部分索引。
CREATE TABLE seller.kyc_reviews (
    review_id          UUID PRIMARY KEY,
    application_id     UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    action             TEXT NOT NULL,
    reviewer_admin_id  BIGINT,
    reason             TEXT,
    internal_note      TEXT,
    decided_at         TIMESTAMPTZ NOT NULL,
    registration_check_id UUID REFERENCES seller.kyc_registration_checks (check_id) ON DELETE SET NULL,
    CONSTRAINT kyc_reviews_action_check CHECK (action IN ('SUBMIT', 'APPROVE', 'REJECT', 'CANCEL', 'REBIND')),
    CONSTRAINT kyc_reviews_reviewer_check CHECK (
        (action IN ('APPROVE', 'REJECT', 'REBIND') AND reviewer_admin_id IS NOT NULL AND reviewer_admin_id > 0) OR
        (action IN ('SUBMIT', 'CANCEL') AND reviewer_admin_id IS NULL)
    ),
    CONSTRAINT kyc_reviews_reason_check CHECK (char_length(reason) BETWEEN 1 AND 1000),
    CONSTRAINT kyc_reviews_reject_reason_check CHECK (action <> 'REJECT' OR reason IS NOT NULL),
    CONSTRAINT kyc_reviews_internal_note_check CHECK (internal_note IS NULL OR (action = 'REJECT' AND char_length(internal_note) BETWEEN 1 AND 1000)),
    CONSTRAINT kyc_reviews_registration_check_check CHECK (registration_check_id IS NULL OR action IN ('APPROVE', 'REJECT'))
);
CREATE INDEX kyc_reviews_application ON seller.kyc_reviews (application_id, decided_at, review_id);
CREATE INDEX kyc_reviews_registration_check ON seller.kyc_reviews (registration_check_id) WHERE registration_check_id IS NOT NULL;

-- 退回時逐項指出的問題：哪個欄位（KYCFieldNames 的路徑）、文件類的欄位指到哪一張（asset_id，不是個資）、原因碼、給賣家看的說明；
-- 不含欄位值。除了 resolved_at 之外 append-only：resolved_at 是賣家處理這一項的時間，由寫入申請的 transaction 在比對新舊值之後
-- 填（欄位的值被改過，或指到的文件被移除或替換）；重新送出、核准、取消不寫它：賣家看到的「還沒處理」只算最近一次退回、
-- 而且之後沒有別的審核紀錄的那些。review_id 的外鍵不 CASCADE：審核紀錄不刪。
CREATE TABLE seller.kyc_review_issues (
    issue_id           UUID PRIMARY KEY,
    review_id          UUID NOT NULL REFERENCES seller.kyc_reviews (review_id),
    position           INTEGER NOT NULL,
    field              TEXT NOT NULL,
    document_asset_id  TEXT,
    code               TEXT NOT NULL,
    message            TEXT NOT NULL,
    resolved_at        TIMESTAMPTZ,
    CONSTRAINT kyc_review_issues_position_key UNIQUE (review_id, position),
    CONSTRAINT kyc_review_issues_position_check CHECK (position >= 0),
    CONSTRAINT kyc_review_issues_field_check CHECK (field IN (
        'real_name',
        'birth_date',
        'id_card_issue_date',
        'id_card_issue_type',
        'id_card_issue_location',
        'english_name',
        'bank_name',
        'bank_code',
        'bank_account',
        'product_type',
        'website_url',
        'id_card_front',
        'id_card_back',
        'bank_book',
        'individual.id_card_no',
        'business.company_name',
        'business.tax_id',
        'business.company_register_date',
        'business.company_address',
        'business.company_phone',
        'business.rep_id_card_no',
        'business.company_documents'
    )),
    CONSTRAINT kyc_review_issues_code_check CHECK (code IN ('UNREADABLE', 'MISMATCH', 'EXPIRED', 'MISSING', 'INVALID', 'OTHER')),
    CONSTRAINT kyc_review_issues_message_check CHECK (char_length(message) BETWEEN 1 AND 500),
    CONSTRAINT kyc_review_issues_asset_check CHECK (
        document_asset_id IS NULL OR
        (document_asset_id <> '' AND field IN ('id_card_front', 'id_card_back', 'bank_book', 'business.company_documents'))
    )
);

-- 後台「顯示全文」的稽核紀錄，同時是限時授權（JIT 存取）：誰、哪一筆申請、何時、理由，永遠不含值；append-only。expires_at
-- 是授權到期的時間（revealed_at 之後 15 分鐘，時間來自注入的時鐘）：同一位管理員在到期之前才能讀這筆申請的證件圖片，
-- 每次讀取寫一列 kyc_document_accesses。
CREATE TABLE seller.kyc_reveals (
    reveal_id      UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    admin_id       BIGINT NOT NULL,
    reason         TEXT NOT NULL,
    revealed_at    TIMESTAMPTZ NOT NULL,
    expires_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT kyc_reveals_admin_check CHECK (admin_id > 0),
    CONSTRAINT kyc_reveals_reason_check CHECK (char_length(reason) BETWEEN 1 AND 1000),
    CONSTRAINT kyc_reveals_expiry_check CHECK (expires_at > revealed_at),
    CONSTRAINT kyc_reveals_grant_key UNIQUE (reveal_id, application_id)
);
CREATE INDEX kyc_reveals_application ON seller.kyc_reveals (application_id, revealed_at DESC, reveal_id DESC);

-- 證件圖片的讀取紀錄：每次後台讀一張證件圖片寫一列，在送出內容之前寫入（寫不進去就不送）。append-only，記實際送出的那張圖片的
-- asset_id（資產 ID 不是個資；證件換過圖之後還查得出當時看的是哪一張），不含內容；reveal_id 是讀取所依據的授權（誰讀的從授權來）。
CREATE TABLE seller.kyc_document_accesses (
    access_id      UUID PRIMARY KEY,
    application_id UUID NOT NULL,
    reveal_id      UUID NOT NULL,
    slot           TEXT NOT NULL,
    position       INTEGER NOT NULL,
    asset_id       TEXT NOT NULL,
    accessed_at    TIMESTAMPTZ NOT NULL,
    -- 讀取與它依據的授權是同一筆申請（複合外鍵，application_id 只為了翻頁而存）。
    CONSTRAINT kyc_document_accesses_grant_fkey FOREIGN KEY (reveal_id, application_id) REFERENCES seller.kyc_reveals (reveal_id, application_id),
    CONSTRAINT kyc_document_accesses_asset_check CHECK (asset_id <> ''),
    CONSTRAINT kyc_document_accesses_slot_check CHECK (slot IN ('ID_CARD_FRONT', 'ID_CARD_BACK', 'BANK_BOOK', 'COMPANY_DOCUMENT')),
    CONSTRAINT kyc_document_accesses_position_check CHECK (position >= 0 AND position < 10 AND (slot = 'COMPANY_DOCUMENT' OR position = 0))
);
CREATE INDEX kyc_document_accesses_application ON seller.kyc_document_accesses (application_id, accessed_at DESC, access_id DESC);

-- PayUni 子商店註冊（ADR 2026-10-19-seller-kyc-payuni-consolidated「核准與 PayUni 綁定」）：一個店家一列（store_id 是主鍵），這一列
-- 就是綁定的意圖（intent）：核准時在同一個 transaction 寫入（bind_status = PENDING）並排入以 store_id 為唯一鍵的 job，
-- job 在 transaction 外呼叫 PayUni，再於自己的 transaction 寫結果。cert 軸（kyc_applications.status）只記人工決定，綁定與
-- PayUni 的審核結果在這張表。MerID 是 'HAWK' 加 store_code，一個 stint 一次性。
-- 不存請求本體（舊的 raw_request 含明文身分證號）：bind 與同步的內容在 job 執行時由 KYC（解密）、店家與這一列的聯絡欄位
-- 組出來，送出就丟。
-- 同步 intent：intent_seq 在任何需要送給 PayUni 的資料改變時加一（核准、店家改聯絡資料或地址、重新綁定），
-- synced_seq 是 job 上次送出時讀到的 intent_seq；兩者不等表示有東西還沒送。同一店家同時只有一個 job 呼叫 PayUni 由
-- River 的唯一鍵（排隊中、執行中、等重試）保證，這裡沒有另外的租約。sync_error_code 是上一次聯絡欄位或地址同步被 PayUni 永久拒絕的代碼
-- （賣家再改一次才會重送）；bind_error_code 只在 FAILED（綁定被永久拒絕、資料不完整或重試用完），由管理員重新綁定。
-- 送給 PayUni 的聯絡資料是店家的欄位（stores 的 contact_name、contact_mobile、support_email、contact_phone 與地址），這裡不存副本。
-- PayUni 自由文字的 reason 是個資：店家關閉滿保留期後由 purge 套件清成 NULL 並記下 pii_purged_at
-- （payuni_registrations_purged_check 要求清理過的列一定沒有它），PayUni 的狀態碼與 outcome 保留。
-- PayUni 目前的狀態是它回報的原碼（外部事實）加上推導的 outcome；outcome 與原碼由同一個條件式 UPDATE 一起寫入（唯一
-- 的寫入點是 payuni 套件的 apply），只有 executed_at 比 status_observed_at 新的觀測會更新（亂序的舊通知不倒退）。
-- 付款能力不存：outcome = APPROVED 且 bind_status = BOUND 時才有。
-- bind_failed_at：綁定失敗的時刻（只有 bind_status = FAILED 時有值，回到 PENDING 或 BOUND 時清空）；後台待辦 review_payuni_bind_retry 的開始時間，
-- 不用 updated_at：失敗期間店家改地址（BumpPayUniIntent）會推後 updated_at。
-- resubmitted_at：已綁定的註冊（PayUni 端已有這個商店）在 PayUni 拒絕之後重新送出、再核准的時間：重新核准不再綁定
-- （MerID 已存在），改以 update_merchant 送新的聯絡欄位與地址；PayUni 的拒絕是舊的狀態，輪詢在這個時間之後的一段期間內
-- 繼續問，直到 PayUni 回報新的結果。
CREATE TABLE seller.payuni_registrations (
    store_id           BIGINT PRIMARY KEY REFERENCES seller.stores (store_id),
    mer_id             TEXT NOT NULL,
    application_id     UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    bind_status        TEXT NOT NULL DEFAULT 'PENDING',
    bind_error_code    TEXT,
    bind_failed_at     TIMESTAMPTZ,
    bind_attempts      INTEGER NOT NULL DEFAULT 0,
    bound_at           TIMESTAMPTZ,
    intent_seq         BIGINT NOT NULL DEFAULT 1,
    synced_seq         BIGINT NOT NULL DEFAULT 0,
    sync_error_code    TEXT,
    resubmitted_at     TIMESTAMPTZ,
    mer_status         TEXT,
    member_status      TEXT,
    credit_status      TEXT,
    applepay_status    TEXT,
    googlepay_status   TEXT,
    atm_status         TEXT,
    reason             TEXT,
    outcome            TEXT NOT NULL DEFAULT 'NONE',
    status_observed_at TIMESTAMPTZ,
    status_checked_at  TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL,
    pii_purged_at      TIMESTAMPTZ,
    CONSTRAINT payuni_registrations_mer_id_check CHECK (mer_id ~ '^HAWK[0-9]{1,10}$'),
    CONSTRAINT payuni_registrations_bind_status_check CHECK (bind_status IN ('PENDING', 'BOUND', 'FAILED')),
    CONSTRAINT payuni_registrations_bind_error_check CHECK (
        (bind_status = 'FAILED') = (bind_error_code IS NOT NULL) AND bind_error_code ~ '^[A-Z][A-Z0-9_]{0,39}$'
    ),
    CONSTRAINT payuni_registrations_bind_failed_check CHECK ((bind_status = 'FAILED') = (bind_failed_at IS NOT NULL)),
    CONSTRAINT payuni_registrations_sync_error_check CHECK (sync_error_code ~ '^[A-Z][A-Z0-9_]{0,39}$'),
    CONSTRAINT payuni_registrations_bound_check CHECK ((bind_status = 'BOUND') = (bound_at IS NOT NULL)),
    CONSTRAINT payuni_registrations_attempts_check CHECK (bind_attempts >= 0),
    CONSTRAINT payuni_registrations_seq_check CHECK (intent_seq >= 1 AND synced_seq >= 0 AND synced_seq <= intent_seq),
    CONSTRAINT payuni_registrations_status_codes_check CHECK (
        char_length(mer_status) >= 1 AND char_length(mer_status) <= 20 AND
        char_length(member_status) >= 1 AND char_length(member_status) <= 20 AND
        char_length(credit_status) >= 1 AND char_length(credit_status) <= 20 AND
        char_length(applepay_status) >= 1 AND char_length(applepay_status) <= 20 AND
        char_length(googlepay_status) >= 1 AND char_length(googlepay_status) <= 20 AND
        char_length(atm_status) >= 1 AND char_length(atm_status) <= 20 AND
        char_length(reason) >= 1 AND char_length(reason) <= 500
    ),
    CONSTRAINT payuni_registrations_outcome_check CHECK (outcome IN ('NONE', 'UNDER_REVIEW', 'PENDING_INFO', 'APPROVED', 'REJECTED', 'UNRECOGNIZED')),
    CONSTRAINT payuni_registrations_observed_check CHECK ((outcome = 'NONE') = (status_observed_at IS NULL)),
    CONSTRAINT payuni_registrations_purged_check CHECK (pii_purged_at IS NULL OR reason IS NULL)
);
CREATE UNIQUE INDEX payuni_registrations_mer_id ON seller.payuni_registrations (mer_id);
-- 輪詢與補送的掃描只看還沒有定論的列。
CREATE INDEX payuni_registrations_poll ON seller.payuni_registrations (status_checked_at NULLS FIRST, store_id)
    WHERE bind_status = 'BOUND' AND (outcome IN ('NONE', 'UNDER_REVIEW', 'PENDING_INFO', 'UNRECOGNIZED') OR resubmitted_at IS NOT NULL);
CREATE INDEX payuni_registrations_unsynced ON seller.payuni_registrations (updated_at, store_id)
    WHERE bind_status <> 'FAILED' AND synced_seq < intent_seq;

-- PayUni 狀態的觀測紀錄：append-only（只有 INSERT 的查詢；唯一的 UPDATE 是個資清理清 reason），每一次從 webhook、輪詢、手動刷新或綁定得到的狀態各一列。
-- executed_at 是 PayUni 的事件時間（webhook 的 ExecuteTime；輪詢、刷新與綁定是我們收到回應的時間），排序與「是否比目前
-- 狀態新」只看它；received_at 是我們寫入的時間。(store_id, event_key) 是 webhook 的冪等鍵（Java 的 event_key 是
-- merId:MerStatus:ExecuteTime），重複的通知只記一次、只套用一次。不存 PayUni 回應的本文（沒有 raw）。狀態碼為空表示
-- 這個來源沒有回報它（webhook 只有 MerStatus；綁定成功的紀錄沒有任何狀態碼）。
-- 保留期限：併入店家個資清理的保留期（ADR 2026-10-19-seller-kyc-payuni-consolidated；舊的 payuni_status_history 的 180 天不另設）：店家關閉滿
-- 保留期後 purge 套件清掉 reason（PayUni 的自由文字，可能帶申請人的資料），狀態碼與時間保留。
CREATE TABLE seller.payuni_observations (
    observation_id   UUID PRIMARY KEY,
    store_id         BIGINT NOT NULL REFERENCES seller.stores (store_id),
    source           TEXT NOT NULL,
    event_key        TEXT,
    mer_status       TEXT,
    member_status    TEXT,
    credit_status    TEXT,
    applepay_status  TEXT,
    googlepay_status TEXT,
    atm_status       TEXT,
    reason           TEXT,
    executed_at      TIMESTAMPTZ NOT NULL,
    received_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT payuni_observations_source_check CHECK (source IN ('WEBHOOK', 'POLL', 'REFRESH', 'BIND')),
    CONSTRAINT payuni_observations_event_key_check CHECK (char_length(event_key) BETWEEN 1 AND 200),
    CONSTRAINT payuni_observations_webhook_key_check CHECK (source <> 'WEBHOOK' OR event_key IS NOT NULL),
    CONSTRAINT payuni_observations_status_codes_check CHECK (
        char_length(mer_status) >= 1 AND char_length(mer_status) <= 20 AND
        char_length(member_status) >= 1 AND char_length(member_status) <= 20 AND
        char_length(credit_status) >= 1 AND char_length(credit_status) <= 20 AND
        char_length(applepay_status) >= 1 AND char_length(applepay_status) <= 20 AND
        char_length(googlepay_status) >= 1 AND char_length(googlepay_status) <= 20 AND
        char_length(atm_status) >= 1 AND char_length(atm_status) <= 20 AND
        char_length(reason) >= 1 AND char_length(reason) <= 500
    ),
    CONSTRAINT payuni_observations_event_key UNIQUE (store_id, event_key)
);
CREATE INDEX payuni_observations_store ON seller.payuni_observations (store_id, executed_at DESC, observation_id DESC);

-- 商家名單（站外業者的 BD 名單，遷移計畫 A3b 步驟 7c 階段 4；ADR 2026-09-25-row-identity-by-data-kind）。ID 沿用 IDENTITY：
-- ai-service 以 BIGINT 存 procurement_candidates.prospect_id（階段 C 換 UUID）。
-- status：ACTIVE（可操作、可被採購搜尋）、ARCHIVED（封存或被合併，merged_into 有值表示被合併、不可還原）、CONVERTED（已轉成
-- 店家，converted_store_id 指向 seller.stores，只供檢視）；converted_store_id 有值 ⇔ CONVERTED。live 是 status 推導的
-- 「還佔著統編與比對鍵的列」（ACTIVE 與 CONVERTED），由資料庫維護；統編的唯一性只對 live 的列成立，比對鍵不唯一（見
-- prospect_contacts）。
-- 文字欄位不存空字串（沒有值是 NULL）；聯絡方式在 prospect_contacts（ADR 2026-10-04-prospect-contact-points-and-matching）。
-- version 是整列的樂觀鎖（文字與圖片共用，etag ADR）；logo、封面存 object key 與 media 的 asset ID，封存或合併時清成 NULL，
-- 清掉的圖記成退場，引用由 seller.media_sync 解除。created_by_admin_id、updated_by_admin_id 是管理員 ID（沒有 FK）。
CREATE TABLE seller.prospects (
    prospect_id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version                 BIGINT NOT NULL DEFAULT 1,
    display_name            TEXT NOT NULL,
    vendor_type             TEXT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'ACTIVE',
    outreach_status         TEXT NOT NULL DEFAULT 'NOT_CONTACTED',
    live                    BOOLEAN GENERATED ALWAYS AS (status <> 'ARCHIVED') STORED,
    tax_id                  TEXT,
    converted_store_id      BIGINT REFERENCES seller.stores (store_id),
    converted_at            TIMESTAMPTZ,
    merged_into             BIGINT REFERENCES seller.prospects (prospect_id),
    category                TEXT,
    region                  TEXT,
    product_summary         TEXT,
    positioning             TEXT,
    research_summary        TEXT,
    priority_letter         TEXT,
    priority_stars          INTEGER NOT NULL DEFAULT 0,
    logo_key                TEXT,
    logo_asset_id           TEXT,
    logo_source_url         TEXT,
    cover_key               TEXT,
    cover_asset_id          TEXT,
    cover_source_url        TEXT,
    created_by_admin_id     BIGINT,
    updated_by_admin_id     BIGINT,
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    CONSTRAINT prospects_version_check CHECK (version >= 1),
    CONSTRAINT prospects_display_name_check CHECK (char_length(display_name) >= 1 AND char_length(display_name) <= 255 AND display_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT prospects_vendor_type_check CHECK (vendor_type IN ('INDIVIDUAL', 'BUSINESS')),
    CONSTRAINT prospects_status_check CHECK (status IN ('ACTIVE', 'ARCHIVED', 'CONVERTED')),
    CONSTRAINT prospects_outreach_check CHECK (outreach_status IN ('NOT_CONTACTED', 'INVITED', 'REPLIED', 'DECLINED', 'DO_NOT_CONTACT', 'CONVERTED')),
    CONSTRAINT prospects_tax_id_check CHECK (tax_id ~ '^[0-9]{8}$'),
    CONSTRAINT prospects_converted_check CHECK (
        (status = 'CONVERTED') = (converted_store_id IS NOT NULL) AND (converted_store_id IS NULL) = (converted_at IS NULL) AND
        (status = 'CONVERTED') = (outreach_status = 'CONVERTED')
    ),
    CONSTRAINT prospects_merged_check CHECK (merged_into IS NULL OR (status = 'ARCHIVED' AND merged_into <> prospect_id)),
    CONSTRAINT prospects_priority_check CHECK (priority_stars >= 0 AND char_length(priority_letter) >= 1 AND char_length(priority_letter) <= 4),
    CONSTRAINT prospects_text_check CHECK (
        char_length(category) >= 1 AND char_length(category) <= 100 AND
        char_length(region) >= 1 AND char_length(region) <= 100 AND
        char_length(product_summary) >= 1 AND char_length(positioning) >= 1 AND char_length(research_summary) >= 1
    ),
    CONSTRAINT prospects_images_check CHECK (
        ((logo_key IS NULL) = (logo_asset_id IS NULL)) AND ((cover_key IS NULL) = (cover_asset_id IS NULL)) AND
        (status <> 'ARCHIVED' OR (logo_key IS NULL AND cover_key IS NULL))
    ),
    CONSTRAINT prospects_source_urls_check CHECK ((logo_key IS NOT NULL OR logo_source_url IS NULL) AND (cover_key IS NOT NULL OR cover_source_url IS NULL)),
    CONSTRAINT prospects_admin_check CHECK (created_by_admin_id > 0 AND updated_by_admin_id > 0)
);
CREATE UNIQUE INDEX prospects_tax_id ON seller.prospects (tax_id) WHERE tax_id IS NOT NULL AND live;
CREATE UNIQUE INDEX prospects_converted_store ON seller.prospects (converted_store_id) WHERE converted_store_id IS NOT NULL;
CREATE INDEX prospects_list ON seller.prospects (updated_at DESC, prospect_id DESC);
CREATE INDEX prospects_active_name ON seller.prospects (lower(display_name), prospect_id) WHERE status = 'ACTIVE';
CREATE INDEX prospects_merged_into ON seller.prospects (merged_into) WHERE merged_into IS NOT NULL;

-- 聯絡方式（ADR 2026-10-04-prospect-contact-points-and-matching D2）：一個業者可以有多個，value 是管理員輸入的原值（顯示用），
-- match_key 是解析器（model.ContactMatchKey）推導的比對鍵，帶種類前綴（EMAIL:、PHONE:、DOMAIN:、SOCIAL:）；解析器判定不具識別力
-- 時（佔位值、共用平台、短網址、貼文路徑）是 NULL，不參與比對。比對鍵不唯一：相同的鍵是「可能是同一家」的訊號（D3），不是約束。
-- 某個類型有聯絡方式時恰好一個是主要的：同類型至多一個主要的由 prospect_contacts_primary 保證（primary_marker 只有主要的列
-- 才是 true），至少一個由延後檢查的自我參照外鍵保證（每一列的 (prospect_id, kind, has_contact) 都要指到那個主要的列），所以
-- 整批替換、合併帶入聯絡方式時，只要在同一個 transaction 內補齊主要的即可。
CREATE TABLE seller.prospect_contacts (
    prospect_id    BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    kind           TEXT NOT NULL,
    value          TEXT NOT NULL,
    match_key      TEXT,
    is_primary     BOOLEAN NOT NULL,
    primary_marker BOOLEAN GENERATED ALWAYS AS (CASE WHEN is_primary THEN true END) STORED,
    has_contact    BOOLEAN GENERATED ALWAYS AS (true) STORED,
    PRIMARY KEY (prospect_id, kind, value),
    CONSTRAINT prospect_contacts_primary UNIQUE (prospect_id, kind, primary_marker),
    CONSTRAINT prospect_contacts_has_primary FOREIGN KEY (prospect_id, kind, has_contact)
        REFERENCES seller.prospect_contacts (prospect_id, kind, primary_marker) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT prospect_contacts_kind_check CHECK (kind IN ('EMAIL', 'PHONE', 'WEBSITE', 'SOCIAL')),
    CONSTRAINT prospect_contacts_value_check CHECK (
        char_length(value) >= 1 AND value !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$' AND
        char_length(value) <= CASE kind WHEN 'EMAIL' THEN 255 WHEN 'PHONE' THEN 50 ELSE 500 END
    ),
    CONSTRAINT prospect_contacts_match_key_check CHECK (
        match_key IS NULL OR (char_length(match_key) <= 600 AND CASE kind
            WHEN 'EMAIL' THEN match_key ~ '^EMAIL:.+$'
            WHEN 'PHONE' THEN match_key ~ '^PHONE:.+$'
            WHEN 'WEBSITE' THEN match_key ~ '^(DOMAIN|SOCIAL):.+$'
            ELSE match_key ~ '^SOCIAL:.+$'
        END)
    )
);
CREATE INDEX prospect_contacts_match_key ON seller.prospect_contacts (match_key, prospect_id) WHERE match_key IS NOT NULL;

-- 「不是同一家」的標記（ADR 2026-10-04-prospect-contact-points-and-matching D3）：管理員確認一對撞到比對鍵的業者不是同一家時寫入，
-- 之後這一對不再擋建立、修改與還原，也不再出現在疑似重複清單；較小的 ID 在前。合併時改掛贏家（D5）。
CREATE TABLE seller.prospect_distinct_pairs (
    low_prospect_id       BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    high_prospect_id      BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    confirmed_by_admin_id BIGINT NOT NULL,
    confirmed_at          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (low_prospect_id, high_prospect_id),
    CONSTRAINT prospect_distinct_pairs_order_check CHECK (low_prospect_id < high_prospect_id),
    CONSTRAINT prospect_distinct_pairs_admin_check CHECK (confirmed_by_admin_id > 0)
);
CREATE INDEX prospect_distinct_pairs_high ON seller.prospect_distinct_pairs (high_prospect_id);

-- 分類樹的標註（category_id 是平台分類樹的 ID），與 prospects.category 的自由文字並存（Q10）；合併時改掛存活的那一列。
CREATE TABLE seller.prospect_categories (
    prospect_id BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    category_id TEXT NOT NULL,
    PRIMARY KEY (prospect_id, category_id),
    CONSTRAINT prospect_categories_category_check CHECK (char_length(category_id) BETWEEN 1 AND 64)
);
CREATE INDEX prospect_categories_category ON seller.prospect_categories (category_id);

-- 商品瀏覽數的原始事件（`docs/modules/seller.md` §16；ADR 2026-09-18 的修訂）：寫入端只追加（登入買家的 ProductViewed 與匿名的打點各一列，
-- event_id 是 UUIDv7，由產生端給，重送的事件不會多一列）。store_id、processed_at、outcome 由背景的彙總 job 補上：商品→店家
-- 的解析在請求的關鍵路徑之外。outcome：COUNTED 是已解析並記進每日彙總（store_id 必有），UNRESOLVED 是商品不存在或沒有
-- 店家（不計入任何地方，留著除錯，不重試）。已處理的事件滿 7 天由清理 job 刪除；重算只涵蓋保留期內。product_id 不建 FK
-- （商品在 catalog）。
CREATE TABLE seller.product_view_events (
    event_id     UUID PRIMARY KEY,
    product_id   BIGINT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    store_id     BIGINT,
    processed_at TIMESTAMPTZ,
    outcome      TEXT,
    CONSTRAINT product_view_events_product_check CHECK (product_id > 0),
    CONSTRAINT product_view_events_outcome_check CHECK (outcome IN ('COUNTED', 'UNRESOLVED')),
    CONSTRAINT product_view_events_processed_check CHECK (
        (processed_at IS NULL AND outcome IS NULL AND store_id IS NULL)
        OR (processed_at IS NOT NULL AND outcome = 'COUNTED' AND store_id IS NOT NULL AND store_id > 0)
        OR (processed_at IS NOT NULL AND outcome = 'UNRESOLVED' AND store_id IS NULL)
    )
);
-- 彙總 job 依發生順序取還沒處理的事件。
CREATE INDEX product_view_events_pending ON seller.product_view_events (occurred_at, event_id) WHERE processed_at IS NULL;
-- 清理 job 與重算依發生時間掃已處理的事件。
CREATE INDEX product_view_events_processed ON seller.product_view_events (occurred_at) WHERE processed_at IS NOT NULL;

-- 商品瀏覽數的每日彙總（`docs/modules/seller.md` §16；Q1 選項 A）：一家店一天一列，台北日曆日；儀表板只讀這張表（唯一事實來源），由彙總 job 依
-- 事件的 occurred_at（不是處理時間）累加，也可以由 RebuildProductViews 在原始事件的保留期內重算。只留店家與日期：沒有商品、
-- 來源欄位（沒有讀取者）。store_id 參照 seller.stores：商品 port 解析出來的店家不在 seller.stores 時，彙總把事件標成
-- UNRESOLVED（不計入），不讓一個不認得的店家卡住整批；「總次數、不去重」是`docs/modules/seller.md` §16 的產品規格。
CREATE TABLE seller.product_views_daily (
    store_id BIGINT NOT NULL REFERENCES seller.stores (store_id),
    day      DATE NOT NULL,
    views    BIGINT NOT NULL,
    PRIMARY KEY (store_id, day),
    CONSTRAINT product_views_daily_views_check CHECK (views >= 1)
);

-- media 引用的同步版本：業務列改變 ref 的資產時在同一個 transaction 遞增 seq，同步 job 讀到 seq 之後做事，結算時只在 seq 沒變
-- 才記下 synced_seq，updated_at 是最後一次變動；保底掃描補排 synced_seq < seq 的 ref（internal/mediaref）。ref 的列不刪除：
-- 刪掉再建會讓 seq 從頭算，同步讀到的舊 seq 可能剛好相等而誤判為已同步。
CREATE TABLE seller.media_ref_sync (
    ref_type   TEXT   NOT NULL,
    ref_id     TEXT   NOT NULL,
    seq        BIGINT NOT NULL DEFAULT 1,
    synced_seq BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ref_type, ref_id)
);
-- 保底掃描只看還沒同步完的 ref。
CREATE INDEX media_ref_sync_unsynced ON seller.media_ref_sync (updated_at) WHERE synced_seq < seq;

-- 被業務列換掉或終態釋放的資產：同步 job 精確解除它們的引用，寫入端拒絕把它們寫回同一個 ref。unlinked_at 是第一次解除成功的時間。
CREATE TABLE seller.retired_assets (
    ref_type    TEXT        NOT NULL,
    ref_id      TEXT        NOT NULL,
    asset_id    TEXT        NOT NULL,
    retired_at  TIMESTAMPTZ NOT NULL,
    unlinked_at TIMESTAMPTZ,
    PRIMARY KEY (ref_type, ref_id, asset_id)
);
CREATE INDEX retired_assets_retired_at ON seller.retired_assets (retired_at);

-- 入駐同意紀錄：賣家在開店（SELLER_TERMS）、送出 KYC（KYC_PRIVACY_NOTICE、PAYUNI_TERMS）時同意或確認已閱讀的文件版本，記事實（哪家
-- 店、哪份文件的哪個版本、誰、何時、來源 IP 與 User-Agent）。每次同意事件一列：沒有 ON CONFLICT，紀錄是一連串事件；開店的 SELLER_TERMS
-- 每家店一列（部分唯一索引），KYC 的兩份每次送出各一列、以 submission_review_id 綁定那次送出的 SUBMIT 審核紀錄（kyc_reviews；
-- 被退回後換了申請主體再送出，也各自留下一組）。
-- 依種類恰好綁一個來源（source_check）。version 是文件的生效日（Go 常數表 model.CurrentAgreements），全文以 repo 裡的正本保存。
-- 只有 INSERT 的查詢，agreement_queries_test.go 檢查；唯一的例外是個資清理把來源欄位清成 NULL（PurgeAgreementSources），以及 dev
-- seed 的重設（internal/devseed 的 clearChildrenSQL 刪掉種子店家的紀錄再依 fixture 重寫）。
-- client_ip 是 inet（取不到是 NULL），user_agent 取不到是 NULL（空字串存成 NULL），上限 512 字是儲存成本。IP 與 User-Agent 是欄位值，
-- 依 ADR 2026-10-19-seller-kyc-payuni-consolidated「關店與個資保留」只保留審核的事實：店家關閉滿保留期由 purge 套件清成 NULL 並記下
-- pii_purged_at（agreement_acceptances_purged_check），種類、版本、時間、帳號與送出的紀錄保留。外鍵不設 ON DELETE（店家列與審核紀錄都不刪）。
CREATE TABLE seller.agreement_acceptances (
    acceptance_id        BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id             BIGINT      NOT NULL REFERENCES seller.stores (store_id),
    kind                 TEXT        NOT NULL,
    version              TEXT        NOT NULL,
    account_id           BIGINT      NOT NULL,
    accepted_at          TIMESTAMPTZ NOT NULL,
    submission_review_id UUID        REFERENCES seller.kyc_reviews (review_id),
    client_ip            INET,
    user_agent           TEXT,
    pii_purged_at        TIMESTAMPTZ,
    CONSTRAINT agreement_acceptances_kind_check CHECK (kind IN ('SELLER_TERMS', 'KYC_PRIVACY_NOTICE', 'PAYUNI_TERMS')),
    CONSTRAINT agreement_acceptances_version_check CHECK (char_length(version) >= 1),
    CONSTRAINT agreement_acceptances_account_check CHECK (account_id > 0),
    CONSTRAINT agreement_acceptances_user_agent_check CHECK (user_agent IS NULL OR char_length(user_agent) BETWEEN 1 AND 512),
    CONSTRAINT agreement_acceptances_source_check CHECK (
        (kind = 'SELLER_TERMS' AND submission_review_id IS NULL) OR
        (kind IN ('KYC_PRIVACY_NOTICE', 'PAYUNI_TERMS') AND submission_review_id IS NOT NULL)
    ),
    CONSTRAINT agreement_acceptances_purged_check CHECK (pii_purged_at IS NULL OR (client_ip IS NULL AND user_agent IS NULL))
);
CREATE UNIQUE INDEX agreement_acceptances_seller_terms ON seller.agreement_acceptances (store_id) WHERE kind = 'SELLER_TERMS';
CREATE INDEX agreement_acceptances_submission ON seller.agreement_acceptances (submission_review_id) WHERE submission_review_id IS NOT NULL;
CREATE INDEX agreement_acceptances_store ON seller.agreement_acceptances (store_id, accepted_at, acceptance_id);

-- 店家封鎖：店家封鎖一位買家（單向：店家看得到買家，買家仍看得到店家的公開內容；封鎖只擋買家在這家店的新互動，不擋下單）。
-- buyer_id 屬於 buyer 模組，不建外鍵；買家刪帳時由 accountdeletion 在同一個刪帳 transaction 刪掉以他為對象的列。反向索引供
-- 「這位買家被哪些店封鎖」的批次查詢。
CREATE TABLE seller.store_blocks (
    store_id   BIGINT NOT NULL REFERENCES seller.stores (store_id),
    buyer_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id),
    CONSTRAINT store_blocks_buyer_id_check CHECK (buyer_id > 0)
);
CREATE INDEX store_blocks_buyer ON seller.store_blocks (buyer_id, store_id);
