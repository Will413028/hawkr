-- =============================================================================
-- V12 — Responsibility Core Finality（WP2A）
--
-- 權威：65 §5.5.2 E0（三個 finality 條件）／E0.1（序列化）／E0.2（lifecycle 閉環）
--       36 §3.3 N16 的 Atomic Boundary
--
-- 本檔把「責任判定案件」的四道不變式從文件落成 DB 機械保證。四道各自獨立，
-- 任一道失效時另外三道仍然成立——這是刻意的：責任判定決定買家拿不拿得到退款，
-- 而它的失效形狀全部是**靜默**的（每一步都「照規則成功」、沒有 CAS 失敗、
-- 沒有例外、沒有錯誤碼，唯一的線索是 Order 停在 35 而 aging 指標一路長上去）。
--
-- 🔴 本檔**不動**任何既有 invariant：
--    chk_exc_status 的四值原樣（DISMISSED 對其他 category 仍合法）
--    uq_exc_open_dedup 原樣（其他 category 仍可合法重開，動它會鎖死不相干的異常流）
--    chk_fcr_review_required 原樣（新型別因此自動 requires_review = true）
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. Lifetime uniqueness：同一 fulfillment leg 終其一生只能有一張責任判定案件
-- -----------------------------------------------------------------------------
-- 🔴 述詞裡**沒有 status**——跨 OPEN／IN_PROGRESS／RESOLVED／DISMISSED 都只能一張。
--    這正是它與 uq_exc_open_dedup 的差別（後者只在未結案的案件之間去重）。
--    它是 65 §5.5.2 E0 條件 ③「同一 subject 無其他責任案件」的**唯一**機械形式。
--
-- ⚠️ DISMISSED 對責任判定 category 是不可達狀態（下面第 2 項），列在述詞射程內
--    是為了讓這條 UNIQUE **不依賴** lifecycle 那一側的正確性——兩道各自獨立守住。
--
-- ⚠️ 述詞裡的 category 字面值是 CodeMeta FulfillmentExceptionCategory 的複製
--    （schema 事實），漂移由 LogisticsSchemaContractTest 逐字對拍守住。
CREATE UNIQUE INDEX "uq_exc_responsibility_subject"
  ON "fulfillment_exception_cases" ("subject_type", "subject_ref")
  WHERE "category" = 'RESPONSIBILITY_UNRESOLVED';

-- -----------------------------------------------------------------------------
-- 2. 責任判定案件不得 DISMISSED
-- -----------------------------------------------------------------------------
-- 責任判定沒有「不處理」這個結局——它決定的是買家拿不拿得到退款，而 Order 正卡在
-- SHIPPING_ABNORMAL(35) 等它。一旦進入 DISMISSED：
--   不能再提 proposal（allowed-from 不含它）→ 不能另開第二張（第 1 項擋住）
--   → 永遠到不了 APPROVED + RESOLVED → 永遠不發 N16 → Order 永遠停在 35。
--
-- 🔴 為什麼是 DB CHECK 不是應用層 if：應用層 if 會在下一次重構被抽掉而沒有人發現
--    （65 §6.4.2 硬約束 1 逐字）。判定欄位全在同一列 ⇒ 單列 CHECK 表達得了 ⇒ 就該落 DB。
--
-- ⚠️ 今天 DISMISSED **連寫入者都不存在**（實查：零 repository／usecase 寫入路徑），
--    所以這道 CHECK 現在攔不到任何真實流量。它的價值在**日後新增 dismiss 入口的那一天**
--    ——那時沒有人會記得這條禁令，而 DB 會記得。
ALTER TABLE "fulfillment_exception_cases"
  ADD CONSTRAINT "chk_exc_responsibility_no_dismiss" CHECK (
      NOT ("category" = 'RESPONSIBILITY_UNRESOLVED' AND "status" = 'DISMISSED'));

-- -----------------------------------------------------------------------------
-- 3. 責任判定案件結案時，resolution_code 只能是四個 resolved 的 ResponsibilityCause
-- -----------------------------------------------------------------------------
-- 🔴 RESPONSIBILITY_UNRESOLVED 是**未結案期間的 category**，不是可結案的 resolution_code。
--    若讓它結案成 UNRESOLVED，finality 三條件會**形式上全部成立**而 N16 仍發不出去
--    （payload 的 responsibility_cause 值域只有 4 個 resolved 值）⇒ Order 一樣卡在 35，
--    而且 ADM 佇列那一側看不到它——case 已經 RESOLVED，離開了 idx_exc_aging 的觀察窗。
--    ⇒ ADM 判不出來時的正確處置是讓案件留在 IN_PROGRESS 繼續補證據，不是結案成 UNRESOLVED。
--
-- 🔴 SYSTEM_CAUSED 不在值域內，因為它**不存在**：平台／系統原因一律歸 PLATFORM_CAUSED
--    （36 §3.3 逐字）。不得為了「看起來完整」而新增它。
--
-- ⚠️ 本 CHECK 內嵌了四個 ResponsibilityCause 值 ⇒ 那是一份**新的碼表複製**，
--    風險高於第 1／2 項（那兩項只嵌 category 一個值）。必須納入逐字對拍，
--    否則 CodeMeta 改值時它會靜默漂移。對拍在 LogisticsSchemaContractTest。
ALTER TABLE "fulfillment_exception_cases"
  ADD CONSTRAINT "chk_exc_responsibility_resolved_cause" CHECK (
      NOT ("category" = 'RESPONSIBILITY_UNRESOLVED'
       AND "status" = 'RESOLVED'
       AND "resolution_code" NOT IN
           ('BUYER_CAUSED', 'SELLER_CAUSED', 'PROVIDER_CAUSED', 'PLATFORM_CAUSED')));

-- -----------------------------------------------------------------------------
-- 4. resolution_type 值域擴充：加入責任判定型別
-- -----------------------------------------------------------------------------
-- 🔴 chk_fcr_review_required 逐字 requires_review = (resolution_type <> 'ANOMALY_RESUME')
--    ⇒ 新型別加進值域之後**自動就是 requires_review = true**，該 CHECK 一個字都不用改。
--    ⚠️ 反過來，若為了「evidence 充分時直接判定」而放行 requires_review = false，
--    則 chk_fcr_reviewer_present 不再要求第二人、chk_fcr_two_person 恆真
--    ⇒ APPROVED 不再保證有兩個人。「不向買家退款」是動買家的錢的決定，
--    maker-checker 就是這個程序要件的機械形式，不得為了省一個人而放行。
ALTER TABLE "fulfillment_case_resolutions" DROP CONSTRAINT "chk_fcr_type";
ALTER TABLE "fulfillment_case_resolutions"
  ADD CONSTRAINT "chk_fcr_type" CHECK ("resolution_type" IN (
      'CLAIM_SETTLE', 'CLAIM_DISMISS', 'SHIPMENT_CANCEL', 'SHIPMENT_CANCEL_CONFIRM',
      'SHIPMENT_CANCEL_AND_REFUND', 'ANOMALY_RESUME', 'RESPONSIBILITY_ATTRIBUTE'));

-- -----------------------------------------------------------------------------
-- 5. resolution_type ↔ case.category 綁定（**跨表**，用複合 FK 落到 DB）
-- -----------------------------------------------------------------------------
-- 這是與第 2 項同形的第二扇門：只關一扇等於沒關。
--   拿 CLAIM_DISMISS 去結一張責任案件 ⇒ case 進 RESOLVED（條件 ① 成立）
--   但它不是責任 resolution ⇒ 條件 ② 永遠不成立 ⇒ 不能再提案（allowed-from 不含 RESOLVED）
--   ⇒ 不能另開（第 1 項）⇒ N16 永不發出 ⇒ Order 永遠停在 35。失效形狀與第 2 項完全相同。
--   ⚠️ ANOMALY_RESUME 還多一層：它 requires_review = false ⇒ 一個人就能永久結掉一張
--   責任案件，同時繞過雙人程序要件。
--
-- 🔴 65 §5.5.2 E0.2 第 4.5 項判定這一道「單列 CHECK 表達不了 ⇒ 只能留在應用層」。
--    那個判斷對**單列 CHECK** 成立，但不是 DB 能力的邊界：把 case 的 category 以
--    **複合外鍵**投影進本表，跨表判定就變成同列判定。
--    ⇒ 本項比該節的最小要求更強，且**不放寬任何既有 invariant**（純加強）。
--    應用層 guard 仍然要有（它能回一個有意義的業務碼，而 DB 這道丟 23503／23514
--    ⇒ 呼叫端拿到 500）——兩道方向一致，不互相取代，與 chk_fcr_two_person 的先例同形。
--
-- ⚠️ 附帶保證：複合 FK 的 ON UPDATE 預設 NO ACTION ⇒ 案件一旦有 resolution，
--    它的 category 就**改不動**了。責任判定的 category 本來就該是 immutable 的。

-- 5a. 複合 FK 的被參照側必須有恰好匹配的 UNIQUE。id 已是 PK（本身唯一），
--     這條在邏輯上冗餘，但 PostgreSQL 的 FK 需要它才指得到 (id, category)。
ALTER TABLE "fulfillment_exception_cases"
  ADD CONSTRAINT "uq_exc_id_category" UNIQUE ("id", "category");

-- 5b. 投影欄。分三步（ADD nullable → 回填 → SET NOT NULL）而不是一句 ADD ... NOT NULL：
--     本表今天是空的，但「能由既有欄位推導的新欄位必須在同一個 migration 內回填」
--     是硬規則（DB Migration 相容性規範 §三）——寫成對空表與非空表都正確的形式，
--     這支 migration 才不會因為套用時機不同而有兩種結果。
ALTER TABLE "fulfillment_case_resolutions" ADD COLUMN "case_category" varchar(48);

UPDATE "fulfillment_case_resolutions" r
   SET "case_category" = c."category"
  FROM "fulfillment_exception_cases" c
 WHERE r."case_id" = c."id"
   AND r."case_category" IS NULL;

ALTER TABLE "fulfillment_case_resolutions" ALTER COLUMN "case_category" SET NOT NULL;

-- 5c. 複合 FK：保證 case_category 忠實反映該 case 真正的 category。
--     沒有它，投影欄就只是一個「寫入者說了算」的欄位 —— 那等於把跨表完整性
--     假裝成單表 CHECK，比留在應用層更糟（它看起來像 DB 守住了）。
ALTER TABLE "fulfillment_case_resolutions"
  ADD CONSTRAINT "fk_fcr_case_category"
  FOREIGN KEY ("case_id", "case_category")
  REFERENCES "fulfillment_exception_cases" ("id", "category")
  ON DELETE RESTRICT;

-- 5d. 雙向等式，不是單向蘊含：
--     責任 category ⇒ 只收責任型別（擋 CLAIM_DISMISS／ANOMALY_RESUME 結掉責任案件）
--     責任型別     ⇒ 只能用在責任 category（擋責任型別被拿去結一般異常案件）
--     單向會讓其中一個方向靜默通過，而兩個方向的後果都是「finality 永遠不成立」。
ALTER TABLE "fulfillment_case_resolutions"
  ADD CONSTRAINT "chk_fcr_responsibility_type_binding" CHECK (
      ("case_category" = 'RESPONSIBILITY_UNRESOLVED')
      = ("resolution_type" = 'RESPONSIBILITY_ATTRIBUTE'));

-- 5e. 讓複合 FK 的被參照側查詢與 ADM 依 category 篩選都走得到索引。
--     （uq_exc_id_category 已是 (id, category)；本索引補 category 單獨篩選的路徑）
CREATE INDEX "idx_fcr_case_category" ON "fulfillment_case_resolutions" ("case_category");
