-- V13__add_order_invoice_snapshot_revision_key.sql
--
-- 58 號 §6.2.5 `ReviseOrderInvoiceSnapshotInternal`（B06／R03 的執行面）的冪等欄。
--
-- 為什麼需要它：該 RPC 的定版 3 要求「同 idempotency_key 重送必須回**同一個**
-- snapshot_version、不新增列」。沒有這一欄，invoice-service 的 gRPC 重試（deadline
-- 逾時後重打）會每次 INSERT 一列新版本 ⇒ 版本號與實際更正次數脫鉤，而版本號正是
-- 開票冪等鍵的組成之一（§11 兩軸表）⇒ 重複開票的風險從這裡進來。
--
-- expand-contract：純新增 nullable 欄 ＋ partial unique index，無 backfill、無既有欄位
-- 變更。舊 pod 不知道這一欄的存在，回滾 image 之後它只是恆為 NULL。
--
-- 🔴 NULL 不參與唯一性（partial index 的 WHERE）：CheckoutConfirm 建 v1 時不帶這把鑰匙
--    （它的冪等由結帳流程自己的機制保證），若讓 NULL 參與唯一性，第二筆 NULL 就會撞鍵。
ALTER TABLE order_invoice_snapshots
    ADD COLUMN revision_idempotency_key VARCHAR(64) NULL;

COMMENT ON COLUMN order_invoice_snapshots.revision_idempotency_key IS
    'B06/R03 更正的冪等鍵＝invoice-service 的 correction_id；CheckoutConfirm 寫的 v1 恆為 NULL';

CREATE UNIQUE INDEX uk_ois_revision_idem
    ON order_invoice_snapshots (order_no, revision_idempotency_key)
    WHERE revision_idempotency_key IS NOT NULL;
