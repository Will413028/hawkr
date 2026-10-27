-- =============================================================================
-- V14 — 責任判定案件的 immutable event context（WP2A-R2 PM Decision）
--
-- 權威：36 §3.3.1 N16／N17 payload；65 §5.5.2 E0.1；PM 裁決（本輪）
--
-- 🔴 本檔要關掉的缺陷：**同一個 leg 的 N17 與 N16 可以帶不同的 order_no，而沒有任何守衛。**
--
-- 舊形狀：
--   open      收 orderNo／shipmentId／shippingMethod → **只拿去組 N17，然後丟掉**
--   （中間隔著數小時～數天與一次人工覆核）
--   finalize  呼叫端**再傳一次**同樣三欄 → 拿去組 N16
--   ⇒ server 端沒有任何一行把兩者比對。
--
-- 為什麼這是嚴重的：`order_no` 正是 order-service 把訂單從 SHIPPING_ABNORMAL(35)
-- 推到 DELIVERY_FAILED_NO_REFUND(54) 的 **CAS 錨**（36 §3.3.1）。兩筆事件帶不同的
-- order_no 時，那道 CAS 永遠 rows=0 ⇒ **Order 永遠停在 35，而全鏈路沒有任何錯誤訊息**。
--
-- ⇒ 本檔把這三欄變成**案件自己的、建立當下凍結的事實**：
--    open 寫入一次，N17 與 N16 都從**同一列**取值，呼叫端在 finalize 時無從指定。
--
-- 🔴 本檔**不動** V9／V12／V13 任何既有 invariant（純加強）：
--    chk_exc_status／chk_exc_resolved／chk_exc_source／uq_exc_open_dedup 原樣
--    uq_exc_responsibility_subject／chk_exc_responsibility_no_dismiss 原樣
--    chk_exc_responsibility_resolved_cause 原樣
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. 三個 context 欄位
-- -----------------------------------------------------------------------------
-- ⚠️ 型別跟齊同一批表的既有 SSOT（V9）：
--      order_no        varchar(32)  ← logistics_receivables:61／provider_settlement_lines:444
--      shipment_id     bigint       ← logistics_receivables:62（TSID，內部用原生型別）
--      shipping_method varchar(8)   ← 與 logistics_receivables:63 的 "provider" **同一個碼域**
--                                     （ShippingMethod 碼值 '3003'／'3004'）
-- 🔴 欄名用 shipping_method 而不是 provider：N16／N17 的 wire key 就是 shipping_method
--    （36 §3.3.1），而本欄存在的唯一理由就是餵那兩個 payload。兄弟表叫 provider 是因為
--    它們描述的是「provider 應收／對帳」那條軸，不是事件欄位。
--
-- 🔴 三欄都 nullable：generic exception case（理賠／對帳未匹配…）本來就沒有這些值，
--    把它們設成 NOT NULL 會讓**既有的 generic 建案流程全部建不出來**
--    ——責任判定的新需求不得破壞不相干的異常流。責任軸的必填由第 2 項的 CHECK 單獨要求。
ALTER TABLE "fulfillment_exception_cases"
  ADD COLUMN "order_no"        varchar(32),
  ADD COLUMN "shipment_id"     bigint,
  ADD COLUMN "shipping_method" varchar(8);

-- 🔴 **不回填**，而且這是刻意的：
--    這三欄的權威值只存在於「建案當下 shipping 側終局處置格傳進來的那一次呼叫」
--    （66 §7.3.4），案件建立之後**系統裡沒有任何地方推導得回來**
--    （65 §5.5.2 已明文排除 logistics_receivables 當來源：責任案件存在期間依定義沒有應收；
--      36 §3.3.1 禁止在業務交易內做 gRPC enrich）。
--    ⇒ 對既有的責任案件列，任何「回填」都只能是**猜**，而猜錯的後果是 order-service
--      的 35→54 CAS 打在錯的訂單上。若存在這類列，下面的 CHECK 會讓 migration **失敗**，
--      由人去決定怎麼處理——那正是我們要的行為。
--
--    實測 2026-09-17（sync 最新 dev 之後重查）：dev 的 fulfillment_exception_cases
--    與 fulfillment_case_resolutions **皆 0 列**；prod 的 chk_fcr_type 尚未含
--    RESPONSIBILITY_ATTRIBUTE（V12 未上 main）⇒ 結構上不可能有責任案件。

-- -----------------------------------------------------------------------------
-- 2. 責任軸的必填；generic 軸完全不受影響
-- -----------------------------------------------------------------------------
-- 🔴 述詞寫成「非責任 category 一律放行」而不是雙向等式——與 V13 的
--    chk_fcr_proposed_cause_binding 形狀**刻意不同**，理由是這兩欄的語意不同：
--      proposed_responsibility_cause：只有責任軸**才有**這個概念 ⇒ 反向也要擋
--      order_no／shipping_method   ：generic 案件**將來也可能**想記它們 ⇒ 不得反向封死
--
-- 🔴 shipment_id **不在必填內**：責任判定案件可能在 shipment 建立之前就產生
--    （建單失敗 CREATE_FAILED(990) 也是一種履約失敗）⇒ 強制它非空會讓那一類案件建不出來，
--    而那正是最需要判責任的情形之一。N16／N17 的 shipment_id 本來就是 nullable String。
ALTER TABLE "fulfillment_exception_cases"
  ADD CONSTRAINT "chk_exc_responsibility_context" CHECK (
      "category" <> 'RESPONSIBILITY_UNRESOLVED'
      OR ("order_no" IS NOT NULL AND "shipping_method" IS NOT NULL));

-- -----------------------------------------------------------------------------
-- 3. 溯源查詢索引
-- -----------------------------------------------------------------------------
-- 與兄弟表同一個形狀（idx_ledgtxn_order／idx_provsettle_order 都是 partial）：
-- 絕大多數 generic 案件此欄為 NULL，full index 只是在存無用的 NULL。
CREATE INDEX "idx_exc_order" ON "fulfillment_exception_cases" ("order_no")
  WHERE "order_no" IS NOT NULL;
