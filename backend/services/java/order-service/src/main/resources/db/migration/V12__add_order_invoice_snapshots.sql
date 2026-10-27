-- V12__add_order_invoice_snapshots.sql
--
-- 58 號《電子發票服務》§5.4.3：order-service 側唯一必要的既有服務 schema 變更。
-- orders 與 order_items 一欄都不動。
--
-- 這張表是「買家在結帳當下宣告的發票開立選擇」的不可變快照，由 CheckoutConfirm 在
-- 建單交易內寫入（付款之前），供 invoice-service 的 order.paid consumer 透過
-- GetOrderInvoiceSnapshotInternal 讀取。
--
-- 零 PII：載具明碼（手機條碼／自然人憑證）一律不落本表，密文留在 invoice-service 的
-- buyer_invoice_preferences；本表只帶 buyer_invoice_preference_id 這個指標。
--
-- expand-contract：純新增表、無 NOT NULL 新欄位於既有表、無 backfill ⇒ 舊 pod 完全
-- 不知道這張表的存在，回滾 image 之後它只是沒有人寫。
CREATE TABLE order_invoice_snapshots (
    id                          BIGINT       PRIMARY KEY,       -- TSID（@TableId(type = IdType.INPUT)）
    order_no                    VARCHAR(64)  NOT NULL,
    snapshot_version            INTEGER      NOT NULL DEFAULT 1,
    invoice_kind                VARCHAR(16)  NOT NULL,          -- B2C / B2B
    carrier_selection_mode      VARCHAR(24)  NOT NULL,          -- USE_BUYER_DEFAULT / EXPLICIT_PREFERENCE / NONE / DONATION
    buyer_invoice_preference_id BIGINT       NULL,              -- invoice-service buyer_invoice_preferences.id（跨服務不建 FK）
    buyer_tax_id                VARCHAR(8)   NULL,              -- 公司統編（公開營業登記資訊，非 PII）
    buyer_title                 VARCHAR(60)  NULL,              -- 買方抬頭。AMEGO BuyerName 上限 60 字元
    donation_code               VARCHAR(7)   NULL,              -- 捐贈碼（公告代碼，非 PII）
    -- 發票通知 email。§5.4.3 的 DDL 逐欄漏了這一欄，但 §6.2.4（order-service ⑤）與
    -- §7.1.2 步驟 1 都明列要採集它，且 proto OrderInvoiceSnapshotData.notify_email 已定版
    -- ⇒ 缺了它 invoice_documents.recipient_email 恆為空、開票通知寄不出去而**不報錯**。
    -- 長度取 254：RFC 5321 §4.5.3.1.3 的 reverse/forward-path 上限（256 含尖括號）。
    -- 這裡刻意不比 buyers.email 更嚴，採集端只擋明顯不成立的輸入，權威格式驗證在應用層
    -- （InvoiceIntakeResolver）＋ invoice-service 寄送前。
    notify_email                VARCHAR(254) NULL,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_order_invoice_snapshots UNIQUE (order_no, snapshot_version),
    CONSTRAINT chk_ois_kind CHECK (invoice_kind IN ('B2C', 'B2B')),
    CONSTRAINT chk_ois_mode CHECK (carrier_selection_mode IN ('USE_BUYER_DEFAULT', 'EXPLICIT_PREFERENCE', 'NONE', 'DONATION')),
    CONSTRAINT chk_ois_b2b CHECK (invoice_kind <> 'B2B' OR (COALESCE(buyer_tax_id, '') ~ '^[0-9]{8}$' AND buyer_title IS NOT NULL AND carrier_selection_mode = 'NONE')),
    CONSTRAINT chk_ois_b2c_no_taxid CHECK (invoice_kind <> 'B2C' OR buyer_tax_id IS NULL),
    CONSTRAINT chk_ois_pref_pair CHECK ((carrier_selection_mode = 'EXPLICIT_PREFERENCE') = (buyer_invoice_preference_id IS NOT NULL)),
    CONSTRAINT chk_ois_donation_pair CHECK ((carrier_selection_mode = 'DONATION') = (donation_code IS NOT NULL)),
    CONSTRAINT chk_ois_donation_fmt CHECK (donation_code IS NULL OR donation_code ~ '^[0-9]{3,7}$'),
    -- notify_email：CHECK 在運算式為 NULL 時放行（§5.5 通則），故顯式寫成 IS NULL 分支。
    -- 只擋「明顯不是 email」（無 @、含空白、無網域點），不做完整 RFC 驗證——過嚴的 CHECK
    -- 會在採集端擋掉合法信箱，而那是使用者看得到的失敗。
    CONSTRAINT chk_ois_notify_email CHECK (
        notify_email IS NULL
        OR notify_email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
    -- 統編／載具／捐贈三選一，在採集端就擋住（AMEGO 不擋）
    CONSTRAINT chk_ois_exclusive CHECK (
        (CASE WHEN buyer_tax_id IS NOT NULL THEN 1 ELSE 0 END)
      + (CASE WHEN buyer_invoice_preference_id IS NOT NULL THEN 1 ELSE 0 END)
      + (CASE WHEN donation_code IS NOT NULL THEN 1 ELSE 0 END) <= 1),
    CONSTRAINT chk_ois_version CHECK (snapshot_version >= 1)
);

-- ⚠️ **刻意不建 `(order_no)` 的單欄索引**：`uk_order_invoice_snapshots (order_no, snapshot_version)`
-- 已經是以 order_no 起首的 btree，PostgreSQL 對 `WHERE order_no = ?` 直接用得上它的前綴。
-- 再建一個窄索引只是每次結帳多一次寫入放大（這張表在結帳 hot path 上），讀取零收益。

COMMENT ON TABLE  order_invoice_snapshots IS '訂單的發票開立選擇快照；零 PII，密文一律留在 invoice-service';
COMMENT ON COLUMN order_invoice_snapshots.buyer_invoice_preference_id IS 'invoice-service buyer_invoice_preferences.id；該表 append-only，故此 id 恆指向下單當下的值';
COMMENT ON COLUMN order_invoice_snapshots.carrier_selection_mode IS 'USE_BUYER_DEFAULT 由 invoice-service 在建立文件時解析預設，不在結帳同步呼叫';
COMMENT ON COLUMN order_invoice_snapshots.notify_email IS '發票開立通知信箱；買家未指定時為 NULL，由 invoice-service 依 D6 fallback 鏈解析';
