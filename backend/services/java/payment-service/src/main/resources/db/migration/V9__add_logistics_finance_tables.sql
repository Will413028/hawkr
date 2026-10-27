-- =============================================================================
-- V9：Logistics Finance —— 線 B（賣家物流財務）併入 payment-service
-- =============================================================================
--
-- 架構裁決 DEC-20260912-184025：MERGE SETTLEMENT DOMAIN INTO PAYMENT SERVICE。
-- 本檔把原本規劃在 hawkr_settlement 的線 B 資料模型移入 hawkr_payment。
--
-- 🔴 DATA MIGRATION: NONE —— 本檔是**純 DDL**
--    實查（2026-09-12）：hawkr_settlement 這個 DB **在本機與 dev 都不存在**
--    （同一句 SELECT datname 回了 23 個其他 hawkr_* DB ⇒ 查詢有效，
--     不是「查不到」而是「真的沒有」）。⇒ 沒有列要搬、沒有流量要切、
--     沒有 atlas_schema_revisions 要處置。
--
-- 搬進來的 9 張表分三群：
--   線 B 本體 ── logistics_receivables / logistics_collection_attempts
--                / seller_logistics_accounts / logistics_adjustments
--   雙分錄帳本 ─ ledger_transactions / ledger_entries
--   供應商對帳 ─ provider_settlement_lines
--                / fulfillment_exception_cases / fulfillment_case_resolutions
--
-- 🔴 為什麼 fulfillment_exception_cases 一起進來（它一度被判為「代銷 dead code」）
--    實查推翻：該表**完全不綁 commerce_model**（整段 DDL 對 commerce_model／
--    consignment 零命中），它是通用的維運例外案件表（source_service 三值、
--    subject_type 多值）。而 66 §6.3.6 指名它是 Uber refund_request 的
--    compensation case 落點 ⇒ 它在**現行 target model** 有消費者。
--    另外 provider_settlement_lines.exception_case_id 對它有 ON DELETE RESTRICT 的 FK，
--    拆開會讓一個真實的完整性保證消失。
--
-- ⚠️ **沒有**搬進來的（屬不可達的 PLATFORM_CONSIGNMENT 舊鏈，本波退役）：
--    payout_instructions / settlement_candidates / settlement_holds
--    / settlement_status_history / seller_invoice_obligations
--    / settlement_admin_audit_logs / settlement_inbox_events
--    / settlement_outbox_events / settlement_provider_audit_logs
--    依據：OrderPolicyResolver 逐字「本波的推導結果：一律 SELLER_DIRECT」，
--    且 65 §11.1 自陳帳本／撥款／F 軸／G 軸「目前零消費者」。
--
-- 🔴 兩處值域在搬入時收斂（見各自的 CHECK 旁註解）：
--    chk_exc_source  : 'settlement' → 'payment'（那個服務已不存在）
--    chk_exc_subject : 移除 'PAYOUT_INSTRUCTION'（它指的表本波退役）
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. Sequences
-- -----------------------------------------------------------------------------
-- ⚠️ 只建本波真的用得到的兩支。原 settlement 還有 payout_instruction_no_seq，
--    它服務的是已退役的撥款鏈 ⇒ 不搬。
CREATE SEQUENCE IF NOT EXISTS "ledger_txn_no_seq"     AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;
CREATE SEQUENCE IF NOT EXISTS "exception_case_no_seq" AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;

-- -----------------------------------------------------------------------------
-- 2. Tables & Indexes
-- -----------------------------------------------------------------------------
-- 🔴 建立順序即依賴序：provider_settlement_lines.exception_case_id 對
--    fulfillment_exception_cases 有 ON DELETE RESTRICT 的 FK，必須排在它之後。
--    ⚠️ 2026-09-12 實跑於探測 DB 時就是先踩到這個順序才發現的——純審查看不出來。


CREATE TABLE "logistics_receivables" (
  "id"                      bigint NOT NULL,                     -- TSID
  "store_id"                bigint NOT NULL,                     -- 賣家業務鍵（🔴 不是 account_id）
  "order_no"                varchar(32),                         -- 溯源；🔴 不是唯一鍵
  "shipment_id"             bigint,                              -- 溯源；🔴 一張訂單可有正逆向兩筆
  "provider"                varchar(8) NOT NULL,                 -- ShippingMethod 碼值（'3003'…），與 ledger owner_ref_id 同口徑
  "provider_delivery_ref"   varchar(64),                         -- uber_deliveries.delivery_id；對帳的關聯鍵
  -- 維度 2：估計 provider 成本（送達當下的暫估值，ADR-008）
  "estimated_cost"          bigint NOT NULL,
  -- 維度 3：實際 provider 成本。🔴 **NULL ＝ 尚未對帳，不是 0**
  "actual_cost"             bigint,
  -- 維度 7：平台補貼（ADR-007）。🔴 與「平台原因造成的成本」是兩件事，不得混記
  "platform_subsidy"        bigint NOT NULL DEFAULT 0,
  -- 🔴 維度 4：Collection Target Net ＝ 這次要讓平台**淨收**多少
  --    ＝ 本期認列（estimated_cost − platform_subsidy）＋ carried_residual。
  --    **不是送給通道的金額**——那個由 payment adapter 依費率反推（ADR-011），
  --    而 domain 不得知道費率。
  "target_net"              bigint NOT NULL,
  -- 🔴 本筆 target_net 內含的前期收款殘差（可負）。
  --    ⚠️ **它不是「上期的手續費」**——是「上期預估與實際的差」，費率估準時為 0。
  --    把它讀成手續費，就會做出 ADR-011 初版那個 fee-on-fee 遞迴。
  "carried_residual"        bigint NOT NULL DEFAULT 0,
  "status"                  varchar(16) NOT NULL DEFAULT 'PENDING',
  -- 🔴 immutable charge basis：凍結當下的計費依據時間。之後政策改了不回溯本列。
  "charge_basis_frozen_at"  timestamptz NOT NULL DEFAULT NOW(),
  -- 🔴 'logistics_receivable:{shipment_id}:{leg}'。UNIQUE 是「送達事件重放不會重複
  --    計費」的**唯一**保證——MQ 至少一次投遞，沒有它就會對同一趟配送建兩筆應收。
  "idempotency_key"         varchar(128) NOT NULL,
  "invoice_document_id"     bigint,                              -- 58 號憑證關聯（方向二：平台 → 賣家）
  -- 🔴 提交 worker 的租約欄（10 檔 §29），**不是「已送出」旗標**。
  --
  --    沒有它的後果很具體：兩個 pod 同時撈到同一列 PENDING ⇒ 各自生一個 TSID trade_no
  --    ⇒ 各自 pre-commit ⇒ **各自呼叫 PayUni** ⇒ 同一筆應收被收兩次錢。
  --    ⚠️ `trade_no` 的 UNIQUE 擋不住這個：兩個 pod 的 trade_no 本來就不同。
  --    ⚠️ `PENDING → SUBMITTED` 的 CAS 也擋不住：它發生在**呼叫 PayUni 之後**。
  --
  --    租約基準必須在 claim 當下**逐筆**取（`NOW() + make_interval`），不在批次迴圈外
  --    算一次——後者會讓不變式從「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，
  --    隨 batch_size 一起破。
  "submit_lease_until"      timestamptz,
  "statement_line_id"       bigint,                              -- provider_settlement_lines.id，對帳後回填
  "created_at"              timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"              timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "logistics_receivables_pkey" PRIMARY KEY ("id"),
  -- 🔴 狀態值域與 allowed-from 見 70 §3.2。SETTLED **不是終態**——收到錢不等於帳結清，
  --    還要等 provider 帳單確定 actual_cost 才能 true-up。把它當終態的後果是
  --    「actual > estimated 的差額永遠不會被收，而帳面上看起來一切正常」。
  CONSTRAINT "chk_logrecv_status" CHECK ("status" IN (
      'PENDING','SUBMITTED','SETTLED','UNCONFIRMED','FAILED','CLOSED','WRITTEN_OFF')),
  CONSTRAINT "chk_logrecv_store"  CHECK ("store_id" > 0),
  -- 🔴 金額恆非負的只有這三個；carried_residual 與 adjustments.amount 允許負數（見各自註解）
  CONSTRAINT "chk_logrecv_estimated" CHECK ("estimated_cost" >= 0),
  CONSTRAINT "chk_logrecv_actual"    CHECK ("actual_cost" IS NULL OR "actual_cost" >= 0),
  CONSTRAINT "chk_logrecv_subsidy"   CHECK ("platform_subsidy" >= 0),
  CONSTRAINT "chk_logrecv_target"    CHECK ("target_net" >= 0),
  -- 對帳完成（statement_line_id 有值）⇔ actual_cost 有值。雙向等式，防「對到帳卻沒填金額」
  CONSTRAINT "chk_logrecv_recon_pair" CHECK (
      ("statement_line_id" IS NULL) = ("actual_cost" IS NULL))
);

CREATE UNIQUE INDEX "uq_logrecv_idem"    ON "logistics_receivables" ("idempotency_key");
CREATE INDEX "idx_logrecv_status"        ON "logistics_receivables" ("status", "created_at");
CREATE INDEX "idx_logrecv_store_created" ON "logistics_receivables" ("store_id", "created_at" DESC);
CREATE INDEX "idx_logrecv_delivery_ref"  ON "logistics_receivables" ("provider_delivery_ref")
  WHERE "provider_delivery_ref" IS NOT NULL;
CREATE INDEX "idx_logrecv_submit_lease" ON "logistics_receivables" ("submit_lease_until")
  WHERE "status" = 'PENDING';

CREATE TABLE "logistics_collection_attempts" (
  "id"                bigint NOT NULL,                           -- TSID
  "receivable_id"     bigint NOT NULL,
  -- 🔴 **我方生成並保證唯一**。PayUni 的 TradeNo 官方三處逐字標「可重複」，
  --    且 agent_fee_query 回**單筆結構非陣列** ⇒ 真的送重複了，用官方 API 也對不出有幾筆。
  "trade_no"          varchar(32) NOT NULL,
  -- 🔴 AgentFee Gross Amount ＝ 實際送出的 Amt，由 payment adapter 依 verified fee policy
  --    反推（domain 不算它，也不得出現費率）。PayUni Amt 值域 1~9999999 的正整數。
  "gross_amount"      bigint NOT NULL,
  -- 🔴 Agent Net Proceeds ＝ gross_amount − actual_fee ＝ 平台**真正收到**的錢。
  --    NULL ＝ 尚未查詢；**不是 0**（0 代表查到了而且真的一毛沒收到）。
  "net_proceeds"      bigint,
  -- 🔴 Collection Residual ＝ target_net − net_proceeds（可負）。**不為 0 不得標記已收訖。**
  "residual"          bigint,
  "submit_status"     varchar(32),                               -- agent_fee 回傳只有 Status／Message 兩欄
  "submit_message"    text,
  "submitted_at"      timestamptz,
  -- 🔴 送出日 ＋ 1 個工作日。UNCONFIRMED 判定的錨——PayUni 沒有 status lifecycle，
  --    「還沒到次日」與「次日執行失敗」都是 FundDate='0000-00-00'，長得一模一樣。
  "expected_fund_date" date,
  -- 來自 agent_fee_query；'0000-00-00' 一律映成 NULL，不得存字面值
  "fund_date"         date,
  -- 🔴 維度 5：實際手續費，來自查詢的 Fee。**不得用預估值（含 1%）冒充。**
  --    正常路徑**不進帳本**——手續費由賣家負擔（ADR-005），對平台不是費用而是
  --    「應收未收足的原因」，未收足的部分自然留在 SELLER_LOGISTICS_RECEIVABLE 餘額裡。
  "actual_fee"        bigint,
  -- 🔴 租約欄，**不是「已查過」旗標**（10 檔 §29）。逐筆在 claim 當下取 now()+N，
  --    不在批次迴圈外算一次——那會讓不變式從「單筆最壞耗時 < 租約」退化成
  --    「批次總耗時 < 租約」，隨 batch-size 一起破。
  "query_lease_until" timestamptz,
  "created_at"        timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"        timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "logistics_collection_attempts_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_logcoll_receivable" FOREIGN KEY ("receivable_id")
      REFERENCES "logistics_receivables" ("id") ON DELETE RESTRICT,
  -- PayUni Amt 值域：Int，1~9999999，不接受 0 或負數。正負號只能由 BalType 表達。
  CONSTRAINT "chk_logcoll_gross" CHECK ("gross_amount" >= 1 AND "gross_amount" <= 9999999),
  CONSTRAINT "chk_logcoll_fee"   CHECK ("actual_fee" IS NULL OR "actual_fee" >= 0),
  -- 🔴 net_proceeds ＝ gross_amount − actual_fee。這條 CHECK 是 M-U4 變異的機械形式：
  --    把它改成 net = gross 會在 DB 層就被擋下。
  CONSTRAINT "chk_logcoll_net" CHECK (
      "net_proceeds" IS NULL OR "actual_fee" IS NULL
      OR "net_proceeds" = "gross_amount" - "actual_fee"),
  -- 查到 FundDate ⇔ 有實際手續費。單邊有值代表查詢只讀了一半
  CONSTRAINT "chk_logcoll_settled_pair" CHECK (
      ("fund_date" IS NULL) = ("actual_fee" IS NULL))
);

CREATE UNIQUE INDEX "uq_logcoll_trade_no" ON "logistics_collection_attempts" ("trade_no");
CREATE INDEX "idx_logcoll_receivable"     ON "logistics_collection_attempts" ("receivable_id", "created_at" DESC);
CREATE INDEX "idx_logcoll_query_lease"    ON "logistics_collection_attempts" ("query_lease_until")
  WHERE "fund_date" IS NULL;

CREATE TABLE "seller_logistics_accounts" (
  "store_id"            bigint NOT NULL,                         -- PK：賣家層級餘額，不以 shipment 為軸
  -- 賣家欠平台（含前期結轉殘差）
  "receivable_balance"  bigint NOT NULL DEFAULT 0,
  -- 🔴 **平台欠賣家**。與上一欄是兩個科目、不是一個欄位的正負：
  --    兩者可以同時存在（這期欠、上期多收），合成淨額之後 ADR-008 第 1 條保證
  --    （永久可追蹤）就失效了。
  -- 🔴 **永不過期、不因賣家無後續交易而清零**（ADR-008 第 2、3 條）
  --    ⇒ **不得有任何排程會把它歸零**。
  "credit_balance"      bigint NOT NULL DEFAULT 0,
  -- 🔴 §5 帳齡階梯的錨。**不是「餘額為正」**——費率估不準時會留下小額殘差，
  --    那是機制的正常產物不是欠款訊號；用餘額當條件會把每個正常賣家判成逾期。
  "last_activity_at"    timestamptz NOT NULL DEFAULT NOW(),
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "seller_logistics_accounts_pkey" PRIMARY KEY ("store_id"),
  CONSTRAINT "chk_sla_store"  CHECK ("store_id" > 0),
  -- 🔴 credit_balance 恆非負：它是「平台欠賣家多少」，負的沒有語意（那是 receivable）
  CONSTRAINT "chk_sla_credit" CHECK ("credit_balance" >= 0)
);

CREATE TABLE "logistics_adjustments" (
  "id"                    bigint NOT NULL,                       -- TSID
  -- 🔴 可為 NULL：未送達的情形不建 receivable（ADR-003 應收只在送達完成成立），
  --    但 Uber 的取消費仍然發生 ⇒ 掛在 store_id ＋ provider_delivery_ref 上，無母單。
  --    ⚠️ 「未送達不收」講的是**價金**，不是「取消費沒人付」。兩種錢的載體因此不同。
  "receivable_id"         bigint,
  "store_id"              bigint NOT NULL,
  "provider_delivery_ref" varchar(64),
  "kind"                  varchar(16) NOT NULL,                  -- RESPONSIBILITY / RECONCILIATION
  -- ⚠️ **允許負數**（與 ledger_entries.amount 恆正相反）：這裡是業務調整單，
  --    正負是它的語意；帳本那邊由 direction 表達。**兩層的規則不同是刻意的。**
  "amount"                bigint NOT NULL,
  -- 🔴 判定原料不足時一律落 PROVIDER_CAUSED（先進爭議），**不是**落「先不記帳」
  "responsibility_cause"  varchar(20),
  "statement_line_id"     bigint,
  "created_by"            bigint,                                -- ADM account_id；系統產生時為 NULL
  "reason"                text NOT NULL,
  "created_at"            timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "logistics_adjustments_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_logadj_receivable" FOREIGN KEY ("receivable_id")
      REFERENCES "logistics_receivables" ("id") ON DELETE RESTRICT,
  CONSTRAINT "chk_logadj_kind"  CHECK ("kind" IN ('RESPONSIBILITY','RECONCILIATION')),
  CONSTRAINT "chk_logadj_store" CHECK ("store_id" > 0),
  -- 🔴 責任調整必須指名成因；對帳調整必須指得回帳單列。兩者都是「這筆錢憑什麼動」的憑據
  CONSTRAINT "chk_logadj_cause" CHECK (
      "responsibility_cause" IS NULL OR "responsibility_cause" IN (
      'SELLER_CAUSED','BUYER_CAUSED','PLATFORM_CAUSED','PROVIDER_CAUSED')),
  CONSTRAINT "chk_logadj_responsibility_needs_cause" CHECK (
      "kind" <> 'RESPONSIBILITY' OR "responsibility_cause" IS NOT NULL),
  CONSTRAINT "chk_logadj_reason" CHECK (length("reason") >= 1),
  -- 🔴 amount = 0 的調整沒有語意，只會讓對帳報表多一列雜訊
  CONSTRAINT "chk_logadj_amount_nonzero" CHECK ("amount" <> 0)
);

CREATE INDEX "idx_logadj_receivable" ON "logistics_adjustments" ("receivable_id")
  WHERE "receivable_id" IS NOT NULL;
CREATE INDEX "idx_logadj_store"      ON "logistics_adjustments" ("store_id", "created_at" DESC);
CREATE INDEX "idx_logadj_statement"  ON "logistics_adjustments" ("statement_line_id")
  WHERE "statement_line_id" IS NOT NULL;

CREATE TABLE "ledger_transactions" (
  "id"                bigint NOT NULL,                       -- TSID（pkg/tsid，Go 服務無 Node ID 概念）
  "txn_no"            varchar(32) NOT NULL,                  -- 'LT' + yyyyMMdd + LPAD(nextval('ledger_txn_no_seq'),8,'0')
  "txn_type"          varchar(32) NOT NULL,                  -- CodeMeta LedgerTxnType（11 值）
  "store_id"          bigint NOT NULL,                       -- store surrogate（非 account_id）
  "order_no"          varchar(64),                           -- MANUAL_ADJUSTMENT 可為 NULL
  "commerce_model"    varchar(24) NOT NULL,                  -- INV-P8 冗餘快照
  "settlement_period" char(6) NOT NULL,                      -- 'YYYYMM'（UTC 曆月）
  "source_type"       varchar(24) NOT NULL,                  -- ORDER / REFUND / PAYOUT / ADMIN
  -- INV-L3：每個減項都要指得回一張單據。NOT NULL 就是那個「指得回」的機械保證。
  "source_ref"        varchar(64) NOT NULL,
  "idempotency_key"   varchar(160) NOT NULL,                 -- '{txn_type}:{source_type}:{source_ref}'
  "posted_at"         timestamptz NOT NULL DEFAULT NOW(),    -- 入帳的業務時間
  "created_at"        timestamptz NOT NULL DEFAULT NOW(),
  -- 🔴 **沒有 updated_at**：本表 append-only，零 UPDATE、零 DELETE。
  --    更正一律開一筆反向的 MANUAL_ADJUSTMENT，不改原列。
  CONSTRAINT "ledger_transactions_pkey" PRIMARY KEY ("id"),
  -- 🔴 **ADR-010：放寬成值域，不是單值。**
  --    原本是 `= 'PLATFORM_CONSIGNMENT'`，而商流改成 SELLER_DIRECT 之後那條 CHECK 讓
  --    整張帳本在 DB 層寫不進任何一列——那不是「零消費者」，是**結構性排他**。
  --    雙分錄模型本身是商流中立的（account_code ＋ owner 五元組），把商流鎖進表級
  --    CHECK 是本波 fail-closed 的權宜，不是永久設計。
  CONSTRAINT "chk_ledgtxn_model"  CHECK ("commerce_model" IN ('PLATFORM_CONSIGNMENT','SELLER_DIRECT')),
  -- 🔴 **不得保留舊值 REFUND_REVERSAL**：它已拆成 _GOODS／_SHIPPING。用 _GOODS 記運費
  --    退款 ⇒ 賣家替一筆他從未收到的運費買單（直接違反需求書 §3.5）。
  CONSTRAINT "chk_ledgtxn_type"   CHECK ("txn_type" IN (
      'ORDER_ACCRUAL','REFUND_REVERSAL_GOODS','REFUND_REVERSAL_SHIPPING','PAYOUT_FEE',
      'PLATFORM_SERVICE_FEE','HOLD_APPLY','HOLD_RELEASE','PAYOUT_SETTLE',
      'MANUAL_ADJUSTMENT','LOGISTICS_COST_ACCRUAL','PROVIDER_SETTLEMENT',
      -- ADR-010 新增（SELLER_DIRECT 的物流財務軸，70 §3.7）
      'LOGISTICS_RECEIVABLE_ACCRUAL','LOGISTICS_COLLECTION_SETTLED','LOGISTICS_TRUE_UP',
      'LOGISTICS_CREDIT_ISSUE','LOGISTICS_CREDIT_APPLY','COLLECTION_FEE_ACCRUAL')),
  -- 🔴 **ADR-010 的第二半：放寬沒有這條就退化成「什麼都能寫」。**
  --    兩套商流的 txn_type 不得交叉——代銷專屬的 PAYOUT_*／HOLD_*／ORDER_ACCRUAL
  --    在 DB 層**不可能**在 SELLER_DIRECT 下寫入，而撥款的 G0 軸守門
  --    （PayoutEligibility 的 commerce_model = PLATFORM_CONSIGNMENT）完全不受影響。
  --    **兩道防線方向一致，不互相取代。**
  --
  -- ⚠️ LOGISTICS_COST_ACCRUAL 與 PROVIDER_SETTLEMENT **兩邊都要有**：物流成本這件事
  --    本身與商流無關（兩種商流都會付錢給 provider），差別只在**對手科目**——
  --    舊商流是 PLATFORM_COLLECTED_CASH（平台代收），新商流是 PLATFORM_OPERATING_CASH
  --    （平台自有現金；SELLER_DIRECT 下平台沒有代收，前者無語意）。
  --    🔴 txn_type 的歸屬看「這個動作在哪個商流下發生」，不看「它是誰引入的」。
  CONSTRAINT "chk_ledgtxn_model_type" CHECK (
    ("commerce_model" = 'PLATFORM_CONSIGNMENT' AND "txn_type" IN (
       'ORDER_ACCRUAL','REFUND_REVERSAL_GOODS','REFUND_REVERSAL_SHIPPING','PAYOUT_FEE',
       'PLATFORM_SERVICE_FEE','HOLD_APPLY','HOLD_RELEASE','PAYOUT_SETTLE',
       'MANUAL_ADJUSTMENT','LOGISTICS_COST_ACCRUAL','PROVIDER_SETTLEMENT'))
    OR
    ("commerce_model" = 'SELLER_DIRECT' AND "txn_type" IN (
       'LOGISTICS_COST_ACCRUAL','PROVIDER_SETTLEMENT','LOGISTICS_RECEIVABLE_ACCRUAL',
       'LOGISTICS_COLLECTION_SETTLED','LOGISTICS_TRUE_UP','LOGISTICS_CREDIT_ISSUE',
       'LOGISTICS_CREDIT_APPLY','COLLECTION_FEE_ACCRUAL','MANUAL_ADJUSTMENT'))
  ),
  CONSTRAINT "chk_ledgtxn_source" CHECK ("source_type" IN ('ORDER','REFUND','PAYOUT','ADMIN')),
  CONSTRAINT "chk_ledgtxn_period" CHECK ("settlement_period" ~ '^[0-9]{6}$'),
  CONSTRAINT "chk_ledgtxn_store"  CHECK ("store_id" > 0)
);

CREATE UNIQUE INDEX "uq_ledgtxn_no"   ON "ledger_transactions" ("txn_no");
CREATE UNIQUE INDEX "uq_ledgtxn_idem" ON "ledger_transactions" ("idempotency_key");
CREATE INDEX "idx_ledgtxn_store_period" ON "ledger_transactions" ("store_id", "settlement_period", "posted_at");
CREATE INDEX "idx_ledgtxn_order" ON "ledger_transactions" ("order_no") WHERE "order_no" IS NOT NULL;

CREATE TABLE "ledger_entries" (
  "id"            bigserial NOT NULL,
  "txn_id"        bigint NOT NULL,
  "line_seq"      smallint NOT NULL,                      -- 表頭內序號，≥1
  "account_code"  varchar(32) NOT NULL,                   -- CodeMeta LedgerAccountCode（13 值）
  "owner_type"    varchar(16) NOT NULL,                   -- CodeMeta LedgerOwnerType：PLATFORM / SELLER / PROVIDER
  -- PLATFORM ⇒ 0；SELLER ⇒ store_id；PROVIDER ⇒ ShippingMethod 碼值（3003／3004）
  "owner_ref_id"  bigint NOT NULL,
  "direction"     varchar(6) NOT NULL,                    -- DEBIT / CREDIT
  "amount"        bigint NOT NULL,                        -- 元（整數，INV-L2），恆為正
  "memo"          varchar(200),
  "created_at"    timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "ledger_entries_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_ledgentry_txn" FOREIGN KEY ("txn_id") REFERENCES "ledger_transactions" ("id") ON DELETE RESTRICT,
  CONSTRAINT "chk_ledgentry_dir"    CHECK ("direction" IN ('DEBIT','CREDIT')),
  -- 🔴 PROVIDER 與 chk_ledgtxn_type 的四個新值是同一批改動的兩半：只改一半 ⇒ 物流成本
  --    分錄在 INSERT 當下被 23514 擋下，而症狀是「撥款流程 500」。
  CONSTRAINT "chk_ledgentry_owner"  CHECK ("owner_type" IN ('PLATFORM','SELLER','PROVIDER')),
  CONSTRAINT "chk_ledgentry_ref"    CHECK (
      ("owner_type" = 'PLATFORM' AND "owner_ref_id" = 0)
   OR ("owner_type" = 'SELLER'   AND "owner_ref_id" > 0)
   OR ("owner_type" = 'PROVIDER' AND "owner_ref_id" > 0)),
  CONSTRAINT "chk_ledgentry_amount" CHECK ("amount" > 0),
  CONSTRAINT "chk_ledgentry_seq"    CHECK ("line_seq" >= 1)
);

CREATE UNIQUE INDEX "uq_ledgentry_txn_seq" ON "ledger_entries" ("txn_id", "line_seq");
CREATE INDEX "idx_ledgentry_seller_account" ON "ledger_entries" ("owner_ref_id", "account_code", "txn_id") WHERE "owner_type" = 'SELLER';

CREATE TABLE "fulfillment_exception_cases" (
  "id"                     bigint NOT NULL,                      -- TSID
  "case_no"                varchar(32) NOT NULL,                 -- 'EX' + yyyyMMdd + 8 位（exception_case_no_seq）
  "source_service"         varchar(32) NOT NULL,
  -- CodeMeta FulfillmentExceptionCategory。🔴 **刻意沒有 chk_exc_category**：
  -- 該 enum 本波尚未落進 CodeMeta（實查 docs/代碼管理（CodeMeta）/*.md 零命中），
  -- 在 DB 硬寫一份值域清單 ＝ 在 SSOT 之外造第二份碼表。值域由寫入路徑的
  -- CodeMetaValidator 守；enum 落地後再補這條 CHECK。
  "category"               varchar(48) NOT NULL,
  "severity"               varchar(8) NOT NULL,
  "subject_type"           varchar(24) NOT NULL,
  "subject_ref"            varchar(64) NOT NULL,
  "store_id"               bigint,
  "status"                 varchar(16) NOT NULL DEFAULT 'OPEN',
  "status_entered_at"      timestamptz NOT NULL DEFAULT NOW(),
  -- D-5 定案：**描述性欄位不是排他鎖**（兩個人同時開同一張案件是可接受的，
  -- 兩個人同時「處理中」而沒人負責才不可接受 ⇒ 由 chk_exc_owner 表達）。
  "owner_account_id"       bigint,
  "owner_assigned_at"      timestamptz,
  "dedup_key"              varchar(200) NOT NULL,                -- '{category}:{subject_type}:{subject_ref}'
  "detail"                 jsonb NOT NULL DEFAULT '{}',
  -- 只存 CodeMeta 碼，不存端點或 URL（URL 會腐化，碼不會）。
  "proposed_action_codes"  jsonb NOT NULL DEFAULT '[]',
  "resolution_code"        varchar(48),
  "resolution_note"        text,
  "resolved_by_account_id" bigint,
  "resolved_at"            timestamptz,
  "occurred_at"            timestamptz NOT NULL,                 -- 來源端發生時間（事件帶進來），不可與 created_at 混用
  "created_at"             timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"             timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "fulfillment_exception_cases_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_exc_status"   CHECK ("status" IN ('OPEN','IN_PROGRESS','RESOLVED','DISMISSED')),
  CONSTRAINT "chk_exc_severity" CHECK ("severity" IN ('P0','P1','P2')),
  -- 🔴 同上：第四個 subject 指向的那張表屬不可達的 PLATFORM_CONSIGNMENT 舊鏈，
  -- 本波一併退役 ⇒ 該 subject 不再有任何可指的對象。
  CONSTRAINT "chk_exc_subject"  CHECK ("subject_type" IN ('ORDER','SHIPMENT','SHIPMENT_BATCH','CONFIG')),
  -- 🔴 2026-09-12 合併時收斂：原值域是三值，第三個是已不存在的那個服務名
  -- （DEC-20260912-184025 選 C）。值域裡留著死值 ⇒「這一列是誰寫的」要比對字面值才知道。
  CONSTRAINT "chk_exc_source"   CHECK ("source_service" IN ('shipping','order','payment')),
  CONSTRAINT "chk_exc_resolved" CHECK (
      "status" NOT IN ('RESOLVED','DISMISSED')
   OR ("resolved_at" IS NOT NULL AND "resolved_by_account_id" IS NOT NULL AND "resolution_code" IS NOT NULL)),
  -- D-5：OPEN ⇒ 無承辦人；離開 OPEN ⇒ 必有承辦人。這是「IN_PROGRESS 有沒有寫入者」
  -- 的機械保證——沒有它，案件可以在無人負責的狀態下停留而不留痕。
  CONSTRAINT "chk_exc_owner" CHECK (
      ("status" = 'OPEN' AND "owner_account_id" IS NULL)
   OR ("status" <> 'OPEN' AND "owner_account_id" IS NOT NULL AND "owner_assigned_at" IS NOT NULL)),
  CONSTRAINT "chk_exc_actions_array" CHECK (jsonb_typeof("proposed_action_codes") = 'array'),
  CONSTRAINT "chk_exc_detail_object" CHECK (jsonb_typeof("detail") = 'object')
);

CREATE UNIQUE INDEX "uq_exc_no" ON "fulfillment_exception_cases" ("case_no");
CREATE UNIQUE INDEX "uq_exc_open_dedup" ON "fulfillment_exception_cases" ("dedup_key")
  WHERE ((status)::text = ANY ((ARRAY['OPEN'::character varying, 'IN_PROGRESS'::character varying])::text[]));
CREATE INDEX "idx_exc_queue" ON "fulfillment_exception_cases" ("severity", "created_at")
  WHERE ((status)::text = ANY ((ARRAY['OPEN'::character varying, 'IN_PROGRESS'::character varying])::text[]));
CREATE INDEX "idx_exc_subject" ON "fulfillment_exception_cases" ("subject_type", "subject_ref", "created_at" DESC);
CREATE INDEX "idx_exc_aging"   ON "fulfillment_exception_cases" ("status", "status_entered_at");

CREATE TABLE "fulfillment_case_resolutions" (
  "id"                      bigint NOT NULL,                     -- TSID（＝對外的 resolution_id，A-5 的 path param）
  "case_id"                 bigint NOT NULL,
  "resolution_type"         varchar(48) NOT NULL,
  "reason"                  text NOT NULL,
  -- 🔴 快照，不是即時算（A-4 request 逐字）。覆核當下重算並比對，不一致 ⇒ 9575。
  "impact_preview"          jsonb NOT NULL,
  "impact_computed_at"      timestamptz NOT NULL,
  "requires_review"         boolean NOT NULL,
  "status"                  varchar(24) NOT NULL DEFAULT 'AWAITING_REVIEW',
  "proposed_by_account_id"  bigint NOT NULL,                     -- ⚠️ account_id（ADM 操作者）
  "proposed_at"             timestamptz NOT NULL DEFAULT NOW(),
  "reviewed_by_account_id"  bigint,
  "review_comment"          text,
  "reviewed_at"             timestamptz,
  "created_at"              timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"              timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "fulfillment_case_resolutions_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_fcr_case" FOREIGN KEY ("case_id") REFERENCES "fulfillment_exception_cases" ("id") ON DELETE RESTRICT,
  CONSTRAINT "chk_fcr_type" CHECK ("resolution_type" IN (
      'CLAIM_SETTLE','CLAIM_DISMISS','SHIPMENT_CANCEL','SHIPMENT_CANCEL_CONFIRM',
      'SHIPMENT_CANCEL_AND_REFUND','ANOMALY_RESUME')),
  CONSTRAINT "chk_fcr_status" CHECK ("status" IN ('AWAITING_REVIEW','APPROVED','REJECTED')),
  -- 🔴 覆核者不得為提案者（沿用 chk_invmu_two_person 的形狀）。
  CONSTRAINT "chk_fcr_two_person" CHECK (
      "reviewed_by_account_id" IS NULL OR "reviewed_by_account_id" <> "proposed_by_account_id"),
  -- 只有 ANOMALY_RESUME 免覆核（§6.1.3 的 resolutionType 表）。
  CONSTRAINT "chk_fcr_review_required" CHECK ("requires_review" = ("resolution_type" <> 'ANOMALY_RESUME')),
  -- 已決 ⇔ 有決策時間。反向也要成立：AWAITING_REVIEW 卻掛著 reviewed_at 會讓
  -- 「什麼時候決的」在稽核上出現兩個互相矛盾的答案。
  CONSTRAINT "chk_fcr_decided_pair" CHECK (
      ("status" IN ('APPROVED','REJECTED')) = ("reviewed_at" IS NOT NULL)),
  -- 需要覆核的提案一旦被決，必須指得出第二個人。
  CONSTRAINT "chk_fcr_reviewer_present" CHECK (
      NOT "requires_review" OR "status" = 'AWAITING_REVIEW' OR "reviewed_by_account_id" IS NOT NULL),
  -- 駁回一定要有理由：沒有理由的駁回在事後無法還原「為什麼不放行」。
  CONSTRAINT "chk_fcr_reject_comment" CHECK (
      "status" <> 'REJECTED' OR ("review_comment" IS NOT NULL AND length("review_comment") >= 1)),
  CONSTRAINT "chk_fcr_reason_len" CHECK (length("reason") >= 1),
  CONSTRAINT "chk_fcr_impact_object" CHECK (jsonb_typeof("impact_preview") = 'object')
);

CREATE UNIQUE INDEX "uq_fcr_open_per_case" ON "fulfillment_case_resolutions" ("case_id") WHERE "status" = 'AWAITING_REVIEW';
CREATE INDEX "idx_fcr_case" ON "fulfillment_case_resolutions" ("case_id", "proposed_at" DESC);
CREATE INDEX "idx_fcr_awaiting" ON "fulfillment_case_resolutions" ("proposed_at") WHERE "status" = 'AWAITING_REVIEW';

CREATE TABLE "provider_settlement_lines" (
  "id"                    bigint NOT NULL,                       -- TSID
  "provider"              varchar(8) NOT NULL,                   -- ShippingMethod 碼值 '3003'／'3004'（同 ledger owner_ref_id 口徑）
  "statement_ref"         varchar(64) NOT NULL,
  "statement_period"      varchar(16) NOT NULL,                  -- 'YYYY-MM'
  "provider_shipment_ref" varchar(64),                           -- 彙總列可為 NULL
  "order_no"              varchar(32),
  "fee_type"              varchar(32) NOT NULL,                  -- CodeMeta ProviderFeeType（5 值）
  "amount"                bigint NOT NULL,                       -- 元，恆為正
  "recon_status"          varchar(16) NOT NULL,
  "ledger_txn_no"         varchar(32),
  "exception_case_id"     bigint,
  "imported_by"           bigint NOT NULL,                       -- ⚠️ ADM actor 的 account_id
  "imported_at"           timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "provider_settlement_lines_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_provsettle_case" FOREIGN KEY ("exception_case_id") REFERENCES "fulfillment_exception_cases" ("id") ON DELETE RESTRICT,
  CONSTRAINT "uq_provsettle_line" UNIQUE ("provider", "statement_ref", "provider_shipment_ref", "fee_type"),
  CONSTRAINT "chk_provsettle_amount" CHECK ("amount" > 0),
  CONSTRAINT "chk_provsettle_recon"  CHECK ("recon_status" IN ('UNMATCHED','MATCHED','DISPUTED','WRITTEN_OFF')),
  -- 雙向等式：MATCHED ⇔ 指得出是哪一筆分錄。單向會讓「已比對但指不出分錄」溜過去。
  CONSTRAINT "chk_provsettle_ledger" CHECK (("recon_status" = 'MATCHED') = ("ledger_txn_no" IS NOT NULL)),
  CONSTRAINT "chk_provsettle_fee_type" CHECK ("fee_type" IN ('DELIVERY','CANCELLATION','WAITING','RETURN','ADJUSTMENT'))
);

CREATE INDEX "idx_provsettle_statement" ON "provider_settlement_lines" ("provider", "statement_period", "imported_at");
CREATE INDEX "idx_provsettle_order" ON "provider_settlement_lines" ("order_no") WHERE "order_no" IS NOT NULL;
CREATE INDEX "idx_provsettle_aging" ON "provider_settlement_lines" ("recon_status", "imported_at");

-- -----------------------------------------------------------------------------
-- 3. INV-L1：每筆帳本交易的 Σ(DEBIT) = Σ(CREDIT)
-- -----------------------------------------------------------------------------
-- 🔴 裁決第 4 條：「不要因為 service 合併就降低 ADR-010 的財務約束」。
--    這道 trigger 原樣搬過來，一個字都不改。
--
-- 為什麼是 DEFERRABLE INITIALLY DEFERRED 的 CONSTRAINT TRIGGER 而不是普通 AFTER
-- TRIGGER：一筆交易的分錄是逐列 INSERT 的，第一列進來的當下必然不平衡。延後到
-- COMMIT 求值，才問得出「這筆交易寫完之後平不平」這個問題。
--
-- 為什麼不放在應用層：不平衡的分錄一旦落庫，帳本就永久失去可稽核性，而症狀是
-- 「某個月的對帳差 N 元」——那時已經無從得知是哪一筆。DB > 應用層收斂點 > gate > 指標。

CREATE OR REPLACE FUNCTION assert_ledger_balanced() RETURNS TRIGGER AS $$
DECLARE
  v_debit  bigint;
  v_credit bigint;
BEGIN
  SELECT
    COALESCE(SUM(amount) FILTER (WHERE direction = 'DEBIT'), 0),
    COALESCE(SUM(amount) FILTER (WHERE direction = 'CREDIT'), 0)
  INTO v_debit, v_credit
  FROM ledger_entries
  WHERE txn_id = NEW.txn_id;

  IF v_debit <> v_credit THEN
    RAISE EXCEPTION
      'ledger transaction % is unbalanced: debit=% credit=%', NEW.txn_id, v_debit, v_credit
      USING ERRCODE = 'check_violation',
            CONSTRAINT = 'trg_ledger_balanced';
  END IF;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ledger_balanced ON "ledger_entries";
CREATE CONSTRAINT TRIGGER trg_ledger_balanced
  AFTER INSERT ON "ledger_entries"
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION assert_ledger_balanced();
