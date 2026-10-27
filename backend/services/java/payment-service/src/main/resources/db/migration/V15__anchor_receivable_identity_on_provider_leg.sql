-- =============================================================================
-- V15 — logistics_receivable 的 identity 錨回 provider fulfillment leg ＋ 責任成因快照
--
-- 權威：70 §3.1（欄位）／§3.1.1（觸發條件）／§3.1.2（identity 不變式）／
--       §3.5.1（logistics_adjustments 不是第二套收款引擎）／§3.6.2（值域分工）
-- ⚠️ 70 把這批標成「V11，實作階段必辦」，而 V11 的號碼已被 outbox exchange 用掉
--    ⇒ 本檔是那批的落地，號碼不同不代表內容不同。
--
-- 本檔關四個缺口，四道各自獨立：任一道失效時另外三道仍然成立。
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. one provider fulfillment leg ＝ one logistics receivable
-- -----------------------------------------------------------------------------
-- 🔴 V9 只建了**非唯一**的 partial index（idx_logrecv_delivery_ref）——它讓查詢走得到索引，
--    但**擋不住同一條 leg 建出第二筆應收**。而那正是 70 §3.1.2 的 identity 不變式本身。
--
-- 失效形狀：同一趟配送被收兩次錢，而帳面上兩筆都「正常」——各自有自己的
-- idempotency_key、各自走完收款流程、各自對得上一張 provider 帳單列。
-- 對帳時才會發現總額對不上，而那是一個月以後的事。
--
-- ⚠️ 述詞 `WHERE provider_delivery_ref IS NOT NULL` 不可省：V9 允許該欄為 NULL
--    （尚未有 provider leg 的列），而 PostgreSQL 的 UNIQUE 對多個 NULL 不去重，
--    但 partial 述詞讓意圖看得見——「沒有 leg 的列不受本約束拘束」是刻意的，
--    不是 NULL 語意的副作用。
--    🔴 反過來，70 §3.1.2 逐字「provider_delivery_ref 為 NULL 時不得建立 receivable」
--    ——那一條是**應用層**的觸發條件（沒有 leg ＝ 沒有 provider cost incurred），
--    不在本 migration 的射程內；兩者不互相取代。
CREATE UNIQUE INDEX "uq_logrecv_delivery_ref"
    ON "logistics_receivables" ("provider_delivery_ref")
    WHERE "provider_delivery_ref" IS NOT NULL;

-- -----------------------------------------------------------------------------
-- 2. 責任成因的 audit snapshot
-- -----------------------------------------------------------------------------
-- 🔴 它是「**建立 receivable 當下**的 resolved responsibility audit snapshot」，
--    **不是**所有 fulfillment responsibility 的 SSOT（70 §3.6.2）。
--    理由是機械的：PROVIDER_CAUSED／PLATFORM_CAUSED **可以沒有** Seller receivable，
--    RESPONSIBILITY_UNRESOLVED **明確沒有** ⇒ receivable 不能成為 responsibility
--    classification 存在的前提。未建立 receivable 時的權威落點是
--    fulfillment_exception_cases ＋ fulfillment_case_resolutions。
--
-- 🔴 值域分工（看起來像矛盾，但它是對的）：classification 本身是 5 值
--    （四個 resolved ＋ 案件 category RESPONSIBILITY_UNRESOLVED），而**本欄只允許
--    四個 resolved 值**——因為 receivable 只在 resolved 之後才建立。
--    下一個人看到兩個數字對不上時，請先讀這一段再動手。
--
-- ⚠️ nullable：V9 既有列（以及日後任何「還沒判定就先建」的路徑）都不該被擋下來。
--    🔴 但「該填而沒填」不是本 CHECK 擋得住的——那由 70 §3.1.1 的觸發條件負責
--    （counterparty 未 resolved ⇒ 根本不建立這一列）。本 CHECK 只保證**填了就必須合法**。
--
-- ⚠️ 本 CHECK 內嵌四個 ResponsibilityCause 值 ⇒ 這是該碼表在 schema 裡的**第三份複製**
--    （前兩份是 V12 的 chk_exc_responsibility_resolved_cause 與 V13 的
--    chk_fcr_proposed_cause_domain）。必須納入 LogisticsSchemaContractTest 的逐字對拍，
--    否則 CodeMeta 改值時它會靜默漂移。
--    🔴 SYSTEM_CAUSED 不在值域內，因為它**不存在**：平台／系統原因一律歸 PLATFORM_CAUSED
--    （36 §3.3.1 逐字）。不得為了「看起來完整」而新增它。
ALTER TABLE "logistics_receivables"
    ADD COLUMN "responsibility_cause" varchar(20);

ALTER TABLE "logistics_receivables"
    ADD CONSTRAINT "chk_logrecv_responsibility_cause" CHECK (
        "responsibility_cause" IS NULL
        OR "responsibility_cause" IN
           ('BUYER_CAUSED', 'SELLER_CAUSED', 'PROVIDER_CAUSED', 'PLATFORM_CAUSED'));

COMMENT ON COLUMN "logistics_receivables"."responsibility_cause" IS
    'Resolved responsibility audit snapshot at receivable creation (70 §3.6.2). NOT the SSOT of fulfillment responsibility: PROVIDER_CAUSED/PLATFORM_CAUSED may have no seller receivable at all. Domain is the 4 resolved ResponsibilityCause values only (the 5th classification state RESPONSIBILITY_UNRESOLVED is an exception-case category, never a cause).';

-- -----------------------------------------------------------------------------
-- 3. idempotency_key 重新錨定：{provider}:{provider_delivery_ref}
-- -----------------------------------------------------------------------------
-- 🔴 舊值 `logistics_receivable:{shipment_id}:{leg}` 與對帳用的 provider_delivery_ref
--    是**兩套 identity truth**（70 §3.1.2 逐字）。provider 建單結果不明後重建、或人工
--    cancel + recreate，會讓同一個 shipment_id 先後對應兩個 delivery
--    ⇒ 兩套 identity 直接分岔，而 uq_logrecv_idem **抓不到**（兩筆的 shipment_id 相同、
--    leg 也相同 ⇒ 它反而會把兩條**不同的 leg** 誤判成重複而擋下第二筆真實成本）。
--
-- ⚠️ 兩個方向都錯，而且症狀相反：一個是重複計費，一個是漏收。
--
-- 🔴 **本檔不做資料回填，因為沒有資料**：createReceivable 在 production code 中
--    **零呼叫端**（實查 src/main 僅 LogisticsCollectionService 自己的宣告，其餘 10 處
--    命中全在 src/test）⇒ 這條路徑從未寫入過任何一列。
--    ⚠️ 若日後發現有列存在，正確處置是**先停下來對帳**，不是寫一支猜測式的回填
--    ——idempotency_key 決定的是「這筆成本收不收得到」。
--    下面這句 UPDATE 因此寫成「對空表與非空表都正確」的形式：它只重寫
--    **推導得出來**的那些列（provider_delivery_ref 非空），推導不出來的留原值，
--    再由第 4 步的 CHECK 讓它現形。
UPDATE "logistics_receivables"
   SET "idempotency_key" = 'logistics_receivable:' || "provider" || ':' || "provider_delivery_ref"
 WHERE "provider_delivery_ref" IS NOT NULL
   AND "idempotency_key" <> 'logistics_receivable:' || "provider" || ':' || "provider_delivery_ref";

-- -----------------------------------------------------------------------------
-- 4. 讓「錨錯了」在寫入當下就現形，而不是對帳時
-- -----------------------------------------------------------------------------
-- 🔴 第 3 步只修了存量，擋不住**下一個**寫入者用舊格式。而 idempotency_key 是
--    「MQ 至少一次投遞不會重複計費」的唯一保證——它錨錯了，uq_logrecv_idem 仍然唯一，
--    只是唯一在錯的東西上 ⇒ **完全靜默**。
--
-- ⚠️ 只在 provider_delivery_ref 非空時要求：為空的列依 70 §3.1.1 本來就不該存在，
--    但那是應用層的觸發條件，本 CHECK 不越權去擋它（擋了會讓不相干的既有列寫不進來）。
ALTER TABLE "logistics_receivables"
    ADD CONSTRAINT "chk_logrecv_idem_anchor" CHECK (
        "provider_delivery_ref" IS NULL
        OR "idempotency_key" = 'logistics_receivable:' || "provider" || ':' || "provider_delivery_ref");

-- -----------------------------------------------------------------------------
-- 5. 更正 V9 對 logistics_adjustments.receivable_id 的欄註解
-- -----------------------------------------------------------------------------
-- V9 的註解逐字寫「未送達的情形不建 receivable … 掛在 store_id ＋ provider_delivery_ref 上，
-- 無母單」——那正是 70 §3.5.1 明令禁止的用法（把 adjustments 當成第二套收款引擎）。
-- 🔴 V9 已 commit **不得修改**（Flyway append-only）⇒ 只能由本檔覆蓋註解（V10 已有同形先例）。
--
-- 為什麼那句話是錯的：logistics_collection_attempts.receivable_id 是 NOT NULL ＋ FK
-- ⇒ 一筆沒有母單的 adjustment 在 DB 層就**掛不上任何一次收款嘗試**
-- ⇒ 那條路徑永遠收不到錢，只會產生一張沒有人去收的調整單。
-- 正解：凡是「provider 已經產生成本、需要向賣家結算」一律走 logistics_receivable。
COMMENT ON COLUMN "logistics_adjustments"."receivable_id" IS
    'Nullable: a responsibility reclassification may land on a leg that never had a receivable. NOT a collection mother record (70 §3.5.1): logistics_collection_attempts.receivable_id is NOT NULL + FK, so an adjustment without a receivable can never be collected. Any provider cost owed by a seller goes through logistics_receivable — one collection engine, not two. (Supersedes the V9 comment, which described the forbidden usage.)';
