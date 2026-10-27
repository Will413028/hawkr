-- =============================================================================
-- V13 — Responsibility Approval 綁定到 Maker 的提案（WP2A-R2）
--
-- 權威：65 §6.4.2 A-4／A-5（maker-checker 契約）＋ 四條硬約束
--
-- 🔴 本檔要關掉的缺陷：**覆核者可以批准一個提案者從未提出的成因。**
--
-- V12 之前的形狀：成因由 checker 在 approve 當下第一次提供（FinalizeCommand.responsibilityCause），
-- 而 maker 的提案列**沒有任何欄位**存得下他主張的成因。於是雙人制只保證了
--   chk_fcr_two_person：覆核者 <> 提案者
-- 卻**不保證**
--   覆核者批准的，正是提案者提出的那一個。
--
-- 兩者的差別不是形式問題：責任成因決定買家拿不拿得到退款
-- （BUYER_CAUSED ⇒ DELIVERY_FAILED_NO_REFUND(54)；其餘三值 ⇒ 全額退款鏈，65 §6.4.2）。
-- 一個人獨自決定「不退款」時，雙人制在帳面上仍然成立——這正是它要防的那件事本身。
--
-- ⚠️ 65 §6.4.2 的 A-4 request **也沒有**成因欄位（A-5 只有 decision ＋ comment）。
--    ⇒ 正式契約當時把「成因從哪裡來」整個留白了，WP2A 把它填在 checker 側，
--    而那是唯一會破壞雙人制的填法。本檔把它前移到 maker 側，並同批回灌 65 §6.4.2。
--
-- 🔴 本檔**不動**任何 V9／V12 既有 invariant（純加強）：
--    chk_fcr_two_person／chk_fcr_review_required／chk_fcr_reviewer_present 原樣
--    chk_fcr_responsibility_type_binding 原樣（本檔的新約束與它同形但管的是不同欄位）
--    uq_fcr_open_per_case 原樣（same-case re-proposal 仍然走得通）
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. Maker 提出的責任成因（提案列上的 typed 欄位）
-- -----------------------------------------------------------------------------
-- 🔴 為什麼是獨立欄位，不是塞進 reason／impact_preview：
--    成因是會驅動退款、54 狀態與財務分流的**正式 domain decision**，
--    不是給人看的說明。塞進 free-text 的後果是它無法被 CHECK 拘束、無法被索引、
--    也無法在 approve 時做機械比對——而「機械比對」正是本檔存在的理由。
--
-- ⚠️ 型別與寬度跟齊 fulfillment_exception_cases.resolution_code（varchar(48)）——
--    approve 時這個值會原封不動寫進那一欄，兩邊不同寬會在最長值那天才炸。
ALTER TABLE "fulfillment_case_resolutions"
  ADD COLUMN "proposed_responsibility_cause" varchar(48);

-- 回填：APPROVED 的責任提案，其成因就是案件的結案碼（chk_exc_resolved 保證它非 null）。
-- 🔴 「能由既有欄位推導的新欄位必須在同一個 migration 內回填」
--    （DB Migration 相容性規範 §三，V12 5b 同一條）。寫成對空表與非空表都正確的形式。
--
-- ⚠️ AWAITING_REVIEW／REJECTED 的責任提案**推導不出來**：案件此時沒有 resolution_code
--    （那正是還沒定案的意思）。這類列回填後仍是 NULL ⇒ 下面第 2 項的 CHECK 會擋下 migration。
--    **那是刻意的**：它代表有一批「提了但沒人說提的是什麼」的資料，
--    該由人決定怎麼處理，不得由本檔編一個成因填進去——那等於偽造一個財務決定。
--    （實測 2026-09-17：dev 上兩張表皆 0 列；prod 的 chk_fcr_type 尚未含
--      RESPONSIBILITY_ATTRIBUTE ⇒ 結構上不可能有這類列。）
UPDATE "fulfillment_case_resolutions" r
   SET "proposed_responsibility_cause" = c."resolution_code"
  FROM "fulfillment_exception_cases" c
 WHERE r."case_id" = c."id"
   AND r."resolution_type" = 'RESPONSIBILITY_ATTRIBUTE'
   AND r."status" = 'APPROVED'
   AND r."proposed_responsibility_cause" IS NULL
   AND c."resolution_code" IS NOT NULL;

-- -----------------------------------------------------------------------------
-- 2. 雙向等式：責任判定型別 ⇔ 有 maker 成因
-- -----------------------------------------------------------------------------
-- 與 chk_fcr_responsibility_type_binding 同形，且同樣**必須雙向**：
--   責任型別 ⇒ 必有成因（擋「提了一個沒有主張的責任提案」⇒ approve 時無值可用
--              ⇒ 只能退回去跟 checker 要 ⇒ 缺陷原樣復活）
--   有成因   ⇒ 只能是責任型別（擋 generic resolution 夾帶一個沒有人會讀的成因，
--              那會讓稽核看到一個「有責任主張」的理賠處置）
-- 單向會讓其中一個方向靜默通過。
ALTER TABLE "fulfillment_case_resolutions"
  ADD CONSTRAINT "chk_fcr_proposed_cause_binding" CHECK (
      ("resolution_type" = 'RESPONSIBILITY_ATTRIBUTE')
      = ("proposed_responsibility_cause" IS NOT NULL));

-- -----------------------------------------------------------------------------
-- 3. 值域：非 null 時必為四個 resolved 的 ResponsibilityCause 之一
-- -----------------------------------------------------------------------------
-- 🔴 這是 ResponsibilityCause 四值在 schema 裡的**第二份複製**
--    （第一份是 V12 的 chk_exc_responsibility_resolved_cause）。V12 該節已寫明：
--    碼表複製必須納入逐字對拍，否則 CodeMeta 改值時它會靜默漂移。
--    ⇒ 對拍在 LogisticsSchemaContractTest.v13ProposedCauseCheckMatchesCodeMeta。
--
-- 🔴 RESPONSIBILITY_UNRESOLVED 不在值域內：它是**未結案期間的 category**，不是可提出的成因。
--    讓它進來 ⇒ approve 之後 case.resolution_code 變成 UNRESOLVED
--    ⇒ 撞 chk_exc_responsibility_resolved_cause（23514）⇒ 交易 abort ⇒ 呼叫端 500。
--    判不出成因時的正確處置是**不要提案**，讓案件留在 IN_PROGRESS 繼續補證據。
-- 🔴 SYSTEM_CAUSED 不在值域內，因為它不存在：平台／系統原因一律歸 PLATFORM_CAUSED（36 §3.3）。
--
-- ⚠️ 拆成兩條 CHECK（binding／domain）而不是合併成一條：合併之後任一種違規都撞同一個
--    constraint 名，測試分不出「漏帶成因」與「帶了非法成因」——而這兩者的修法完全不同。
ALTER TABLE "fulfillment_case_resolutions"
  ADD CONSTRAINT "chk_fcr_proposed_cause_domain" CHECK (
      "proposed_responsibility_cause" IS NULL
      OR "proposed_responsibility_cause" IN
         ('BUYER_CAUSED', 'SELLER_CAUSED', 'PROVIDER_CAUSED', 'PLATFORM_CAUSED'));
