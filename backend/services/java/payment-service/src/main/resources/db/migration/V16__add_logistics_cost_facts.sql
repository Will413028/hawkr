-- =============================================================================
-- V16 — logistics_cost_facts：provider 成本事實的持久化載體
--
-- 權威：70 §3.1.4（durable join）／§7.1（wire 契約）／SD-15（成本先記錄，歸屬未定）
--
-- 🔴 它補的是一個「設計書要求、但此前沒有載體」的缺口。
--    SD-15 逐字要求「成本已發生的事實**先記錄**，歸屬未定」，而 70 號全書
--    join／暫存／staging 皆 0 命中 ⇒ 那句話在實作上無處落地。
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 為什麼不能沿用現有表（逐一排除過，不是沒查就建新的）
-- -----------------------------------------------------------------------------
-- logistics_receivables      §3.1.1 明令「未 financially final 期間不得建立 Seller
--                            logistics receivable——receivable 只是還沒建立」。
--                            🔴 機械後果比語意更硬：PENDING 是提交 worker 的 claim
--                            述詞（idx_logrecv_submit_lease）⇒ 建了那一列就等於
--                            **開始向賣家收錢**，而此時 counterparty 還沒 resolved。
--
-- logistics_adjustments      §3.5.1 明令它不是收款載體；chk_logadj_amount_nonzero
--                            也擋「還沒有金額」的列。
--
-- provider_settlement_lines  它是**月結帳單的行**（statement_ref／statement_period／
--                            imported_by ＝ ADM actor）⇒ 拿來存即時成本事實要編造
--                            三個欄位，語意錯位。
--
-- 🔴 本表**不是第二套 receivable engine**（§14.1／§3.5.1 的射程）：
--    沒有狀態機、沒有收款嘗試、不參與任何收款流程，是 append-only 的事實表。
--    應收的唯一建立路徑仍然只有一條 —— cost incurred AND counterparty = SELLER。
--    ⚠️ 判準是「這張表壞掉會不會讓錢收錯」：它壞掉只會讓應收**建不出來**
--    （fail-closed），不會讓錢收到錯的人身上。
-- -----------------------------------------------------------------------------

CREATE TABLE "logistics_cost_facts" (
  "id"                    bigint NOT NULL,        -- TSID
  "provider"              varchar(8)  NOT NULL,   -- ShippingMethod 碼值（'3003'…）
  "provider_delivery_ref" varchar(64) NOT NULL,   -- 🔴 identity 的錨（§3.1.2）
  "store_id"              bigint      NOT NULL,
  "order_no"              varchar(32) NOT NULL,
  -- ⚠️ nullable：取消可能發生在 shipment 建立之前，這個事實真的可能沒有 shipment
  --    （70 §7.1 逐字）。正因為它可以是 NULL，它不能當 identity。
  "shipment_id"           bigint,
  -- 元整數。🔴 provider 的 minor unit 不得滲透進來——換算在 provider adapter 邊界完成。
  "estimated_cost"        bigint      NOT NULL,
  -- 🔴 事實發生的**業務時間**，不是收到事件的時間。用作 charge_basis_frozen_at 的來源：
  --    計費依據凍結在事實發生的那一刻，之後政策改了不回溯本列。
  "occurred_at"           timestamptz NOT NULL,
  -- L1 去重：broker 重投同一則事件時認得出來。
  "event_id"              varchar(64) NOT NULL,
  "created_at"            timestamptz NOT NULL DEFAULT NOW(),

  CONSTRAINT "logistics_cost_facts_pkey" PRIMARY KEY ("id"),

  -- 🔴 一條 provider fulfillment leg ＝ 一筆成本事實。
  --    與 logistics_receivables 的 uq_logrecv_delivery_ref 同口徑（單欄、不含 provider）
  --    ——兩張表對「什麼是同一條 leg」必須是同一個答案，否則 join 會對到錯的列。
  CONSTRAINT "uq_costfact_delivery_ref" UNIQUE ("provider_delivery_ref"),

  -- L1：同一則事件重投不產生第二列。
  CONSTRAINT "uq_costfact_event" UNIQUE ("event_id"),

  -- 金額恆為正：0 是「沒有成本」，那種情形不該發事件（見 CostCapability 的 empty 語意）。
  CONSTRAINT "chk_costfact_amount" CHECK ("estimated_cost" > 0),
  CONSTRAINT "chk_costfact_store"  CHECK ("store_id" > 0)
);

-- join 的查詢面：責任先到時，用 delivery_ref 回頭找成本事實。
-- （uq_costfact_delivery_ref 本身就是索引，不另建。）

-- 對帳／稽核面：某個賣家在某段期間有哪些成本事實。
CREATE INDEX "idx_costfact_store_time" ON "logistics_cost_facts" ("store_id", "occurred_at");

COMMENT ON TABLE "logistics_cost_facts" IS
  'provider 成本事實（70 §3.1.4 durable join 的成本側載體）。append-only、無狀態機、不參與收款。'
  'SD-15 逐字要求「成本已發生的事實先記錄，歸屬未定」，本表是那句話的載體。'
  '🔴 它不是 receivable 的替身：應收的唯一建立路徑仍是 cost incurred AND counterparty = SELLER。';

COMMENT ON COLUMN "logistics_cost_facts"."provider_delivery_ref" IS
  '🔴 identity 的錨（70 §3.1.2）。與 logistics_receivables.provider_delivery_ref 同口徑，'
  '正逆向兩條 leg 各自有值 ⇒ 天然各自一筆。';

COMMENT ON COLUMN "logistics_cost_facts"."estimated_cost" IS
  '新台幣元整數，恆為正。🔴 不是 provider 最終帳單：後續月結帳單走 actual_cost true-up '
  '同一筆 receivable（70 §6.2、§3.1.3），不得為 true-up 建立第二筆。';

COMMENT ON COLUMN "logistics_cost_facts"."occurred_at" IS
  '🔴 成本事實發生的業務時間，不得用收到事件的時間冒充（兩者在重投時會差很多）。';
