-- Plan C 系列收尾：coupon_templates.issuer_store_id 語意混合欄位拆分。
-- docs/superpowers/specs/2026-08-06-coupon-issuer-identity-schema-split-design.md
--
-- V3 migration（V3__seller_account_id_to_store_id.sql）把這個欄位 rename 成
-- issuer_store_id，但值仍是 ADMIN 模板存 admin account_id、SELLER 模板存
-- store_id 的混合語意，靠 issuer_type 欄位當判別式。這次徹底拆成兩個各自誠實
-- 的欄位。
--
-- 本檔是 expand-contract 的 expand 階段（docs/開發規範/DB Migration 相容性規範
-- （expand-contract）.md §4）：只加 nullable 欄位 + 放寬既有欄位的 NOT NULL +
-- 回填，全部是舊 code 在新 schema 上仍能正常寫入/讀取的安全變更。刻意*不*在此檔加
-- CHECK constraint（`issuer_type` 與非 NULL 欄位互斥的約束）——本 release 部署時，
-- 這支 migration 在 Spring Boot 啟動當下就套用，但 surge 滾動更新（maxSurge:1,
-- maxUnavailable:0）視窗內仍有舊 pod 在跑：舊 Java code 建立 ADMIN 模板時只寫
-- issuer_store_id、完全不知道 issuer_account_id 這個新欄位存在，若此時就有 CHECK
-- constraint 在場，舊 pod 的每一筆 ADMIN 建券請求都會被 DB 拒絕、對外變成 500，直到
-- 該 pod 被新版汰換為止。CHECK constraint 留給下一個「contract」release：確認本次
-- release 的 pod 已全數滾動完成（線上不再有只認識單一 issuer_store_id 欄位的舊
-- code）後，再用獨立的 migration 補上
-- `ALTER TABLE coupon_templates ADD CONSTRAINT chk_tpl_issuer_identity CHECK (...)`。
-- 在 contract release 上線前，兩欄互斥仍只是「新 code 的寫入路徑保證如此」，DB 層
-- 尚未真的擋。

ALTER TABLE coupon_templates
  ADD COLUMN issuer_account_id BIGINT,
  ALTER COLUMN issuer_store_id DROP NOT NULL;

UPDATE coupon_templates
  SET issuer_account_id = issuer_store_id,
      issuer_store_id   = NULL
  WHERE issuer_type = '1'; -- ADMIN

COMMENT ON COLUMN coupon_templates.issuer_account_id IS
  '建立者 admin accountId（issuer_type=ADMIN 專用，SELLER 模板為 NULL，由新 code 寫入路徑保證）- V6 split from issuer_store_id。DB 層 CHECK constraint 待 contract release 補上，見本檔頭註解';
COMMENT ON COLUMN coupon_templates.issuer_store_id IS
  '建立者 store_id（issuer_type=SELLER 專用，ADMIN 模板為 NULL，由新 code 寫入路徑保證）- V6 起不再是通用欄位。DB 層 CHECK constraint 待 contract release 補上，見本檔頭註解';
