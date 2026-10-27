-- =====================================================================
-- V5：PayUni 互動 audit 專表（payuni_audit_logs）
-- =====================================================================
-- 物流爭議稽核專表，與 payment-service 同構（欄位語意權威見 05 §11.2）。
-- 承載出向（我方 → PayUni）物流互動：超商/宅配建單、print_label、cat_file_download、query 等。
-- 稽核原文不截斷、保留 7 年、append-only（只 INSERT，清理由保留期排程統一 DELETE）。
-- 收件人 PII（Consignee / ConsigneeMobile / ConsigneeAddress）明文保留（爭議標的本身，見 06 §10.5）；
-- 憑證欄位（CustomerToken / Hash）仍遮罩。詳見 06 §10。
-- =====================================================================

CREATE TABLE payuni_audit_logs (
    id                  BIGSERIAL PRIMARY KEY,              -- 自增主鍵（內部稽核表）
    direction           VARCHAR(10) NOT NULL,               -- OUTBOUND / INBOUND（CodeMeta PayUniAuditDirection）
    trade_no            VARCHAR(32),                        -- 出向自 request params ShipTradeNo / MerTradeNo；query 類無單號 → null
    refund_no           VARCHAR(32),                        -- 退貨物流相關填
    mer_id              VARCHAR(20),                        -- PayUni 商家編號
    api_path            VARCHAR(100) NOT NULL,              -- 如 /api/logistics/print_label、cat_file_download

    -- 稽核內容（原文不截斷）
    request_params      JSONB,                              -- 出向業務參數明文（憑證欄位遮罩；收件人 PII 明文保留）；入向解密後 payload
    response_raw        TEXT,                               -- 原始回應全文（含 HTML form 全文、加密原文）；二進位檔（PDF）不落內容
    response_decrypted  JSONB,                              -- 解密後內層（PayUni 原生 PascalCase key，opaque passthrough）
    outer_status        VARCHAR(20),                        -- 外層 Status；parse/decrypt/hash 失敗記 PARSE_ERROR / HASH_INVALID
    outer_message       TEXT,                               -- 外層 Message；二進位下載記 metadata（型別/bytes）
    error_code          VARCHAR(50),                        -- 傳輸層錯誤
    error_message       TEXT,                               -- 傳輸層錯誤訊息
    http_status         INTEGER,                            -- HTTP 狀態碼
    duration_ms         BIGINT,                             -- 呼叫耗時（毫秒）

    -- 時間戳記
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payuni_audit_trade_no ON payuni_audit_logs(trade_no);
-- refund_no 多數列為 NULL，partial index 省空間
CREATE INDEX idx_payuni_audit_refund_no ON payuni_audit_logs(refund_no) WHERE refund_no IS NOT NULL;
CREATE INDEX idx_payuni_audit_created_at ON payuni_audit_logs(created_at);
