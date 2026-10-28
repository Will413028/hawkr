# Product re-key backfill gate（② seller/store aggregate re-key — W1 step 3 前置）

> 對應 design doc `docs/superpowers/specs/2026-07-11-seller-store-aggregate-rekey-design.md` §3.2（**未入版控，僅本機**；身分模型的版控權威為[44-帳號與賣家買家身份模型](../詳細設計書/44-帳號與賣家買家身份模型%EF%BC%88Account%20&%20Seller-Buyer%20Identity%EF%BC%89詳細設計書.md)）。
> **何時跑**：部署 product-service V8（owner column）之前（G1/G2）與之後、解除 write-freeze 之前（G3 + repair）。
> **欄位名**：本文一律使用現行名 `store_id`（`sellers` 與 `product` 兩表皆是；V8 建立時原名 `seller_account_id`，`product` 於 V9、`sellers` 於 migration `20260711155335` 更名，值不變）。
> **DB 連線**：透過 bastion + 1Password 取憑證（同 food-safety runbook §2 慣例）；dev 與 stage 各跑一輪。
> **兩個 DB**：`hawkr_seller`（sellers 表）與 `hawkr_product`（product 表）— 跨 DB，無法在單一 migration 內判定，故為明確的人工 gate（stated, not assumed）。

## 0. 部署順序（write-freeze cut）

1. **凍結**：dev/stage 把 product-service scale 到 0（Flyway 跑在 pod 啟動，freeze 保證 V8 的 backfill + NOT NULL 與新 code 原子上線、無舊 pod 寫入）。
2. 跑 **G1 / G2**（下方）。G1 = 0 rows 且 G2 = 0 rows → 簡單 backfill `:= created_by` 精確，V8 直接過，跳到步驟 5。
3. 部署（V8 於啟動時執行）→ **立刻再 scale 回 0 或維持單 pod 停流量**（repair 要在解凍前完成）。
4. G1 有 rows → 對每個 revived seller 跑 **G3** 分類，依 decision table 執行 **R1 / R2**。
5. 解除凍結（scale 回正常 replicas），把 G1–G3 輸出貼進 PR 描述存證。

## 1. 偵測

### G1 — revived sellers（`hawkr_seller`）

reshape（W1 step 2）後 schema：

```sql
-- current stint 的 surrogate 已與登入 account_id 分岔 = 曾 revive 過
SELECT account_id,
       store_id   AS current_surrogate,
       created_at AS current_stint_started_at,
       status
FROM sellers
WHERE is_current
  AND store_id <> account_id;
```

（若在 reshape 部署前提早跑：同句拿掉 `is_current` 條件 — 舊 schema 單 row per account。）

**預期：0 rows**（dev revive 功能 2026-07-08 才上線，截至 gate 撰寫日未知有無實際 revive）。

### G2 — `created_by` NULL 孤兒（`hawkr_product`）

V8 `UPDATE ... SET store_id = created_by` 後 `SET NOT NULL` 會被 NULL row 弄死：

```sql
SELECT id, name, status, created_at
FROM product
WHERE created_by IS NULL;
```

**預期：0 rows**。有 rows = 壞資料，逐筆決定補 owner 或軟刪（dev-data cleanup，記錄於 PR）。

### G3 — revived seller 名下商品分類（`hawkr_product`，V8 之後）

對 G1 每一筆（`:account_id`、`:current_surrogate`、`:stint_started_at`）：

```sql
SELECT id, status, created_at, seller_deleted_at,
       CASE
         WHEN seller_deleted_at IS NOT NULL          THEN 'KEEP'
         WHEN created_at >= :stint_started_at        THEN 'REPAIR'
         ELSE 'MANUAL'
       END AS action
FROM product
WHERE created_by = :account_id
  AND deleted_at IS NULL
ORDER BY created_at;
```

## 2. Repair decision table

| 商品條件 | `store_id` 應為 | 動作 |
|---|---|---|
| `seller_deleted_at IS NOT NULL`（Part A stamp = 舊化身、已 off-shelf） | `created_by`（= 舊化身 owner，**deliberately dangling** — isolation-correct） | 不動（V8 backfill 已正確） |
| unstamped 且 `created_at >= current_stint_started_at`（現行化身建立） | current surrogate | **R1** |
| unstamped 且 `created_at < current_stint_started_at`（舊化身建立但漏 stamp — revive 早於 #1407 stamp 上線的窗口） | 舊 owner + 補 stamp，或直接 wipe（軟刪） | **R2**（per-product 明確決定，記錄於 PR） |

## 3. Repair SQL（transaction + RETURNING，核對筆數後才 COMMIT — 同 food-safety runbook §2.2）

```sql
BEGIN;
-- R1: 現行化身商品 → 指到 current surrogate
UPDATE product
   SET store_id = :current_surrogate
 WHERE created_by = :account_id
   AND deleted_at IS NULL
   AND seller_deleted_at IS NULL
   AND created_at >= :stint_started_at
RETURNING id, store_id;
-- 筆數必須 == G3 的 REPAIR 列數，否則 ROLLBACK
COMMIT;
```

```sql
BEGIN;
-- R2: 漏 stamp 的舊化身商品 → 補 stamp（owner 保持 dangling 舊值）
UPDATE product
   SET seller_deleted_at = NOW(), updated_at = NOW()
 WHERE id IN (/* G3 MANUAL 中決定 re-stamp 的 id，逐一列出 */)
   AND seller_deleted_at IS NULL
RETURNING id, seller_deleted_at;
COMMIT;
```

## 4. Orphan check（W3 Part A retirement 的 hard gate，本窗口先跑一次留 baseline）

```sql
-- hawkr_product：所有存活 owner key
SELECT DISTINCT store_id FROM product
WHERE deleted_at IS NULL AND seller_deleted_at IS NULL
ORDER BY 1;
```
```sql
-- hawkr_seller：全部 stint surrogates（含歷史 row，不濾 is_current）
SELECT store_id FROM sellers ORDER BY 1;
```

前者必須是後者的子集（`comm -23 product_keys.txt seller_keys.txt` 應為空）。stamped（`seller_deleted_at IS NOT NULL`）商品**允許** dangling — 那是刻意的舊化身隔離；reconciliation 需逐 seller 比對 ownership 對到 current surrogate（count-equality 抓不到 mis-attachment），mismatch 阻擋 W3。
