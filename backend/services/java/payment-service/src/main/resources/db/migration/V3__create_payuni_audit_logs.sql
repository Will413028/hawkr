-- =====================================================================
-- V3：PayUni 互動 audit 專表（payuni_audit_logs）
-- =====================================================================
-- 金流爭議稽核專表，承載出向（我方 → PayUni）與入向（PayUni → 我方）完整對話。
-- 稽核原文不截斷、保留 7 年、append-only（只 INSERT，清理由保留期排程統一 DELETE）。
-- 廢除舊 payment_logs 上的 PAYUNI_* 五型稽核，payment_logs 回歸純業務日誌。
-- 詳見 05 §11（表結構 §11.2、opaque passthrough 例外 §11.2.1）。
-- =====================================================================

CREATE TABLE payuni_audit_logs (
    id                  BIGSERIAL PRIMARY KEY,              -- 自增主鍵（內部稽核表）
    direction           VARCHAR(10) NOT NULL,               -- OUTBOUND / INBOUND（CodeMeta PayUniAuditDirection）
    trade_no            VARCHAR(32),                        -- 出向自 request params MerTradeNo；查詢類 API 無單號 → null
    refund_no           VARCHAR(32),                        -- 退款相關呼叫填
    mer_id              VARCHAR(20),                        -- PayUni 商家編號（HAWK<account_id>）
    api_path            VARCHAR(100) NOT NULL,              -- 如 /api/trade/grant

    -- 稽核內容（原文不截斷）
    request_params      JSONB,                              -- 出向業務參數明文（憑證欄位遮罩）；入向解密後 payload
    response_raw        TEXT,                               -- 原始回應 body 全文（含加密原文 / HTML 全文）；二進位不落內容
    response_decrypted  JSONB,                              -- 解密後內層（PayUni 原生 PascalCase key，opaque passthrough）
    outer_status        VARCHAR(20),                        -- 外層 Status；parse/decrypt/hash 失敗記 PARSE_ERROR / HASH_INVALID
    outer_message       TEXT,                               -- 外層 Message；二進位下載記 metadata
    error_code          VARCHAR(50),                        -- 傳輸層錯誤（IOException / 非 200 / EMPTY response）
    error_message       TEXT,                               -- 傳輸層錯誤訊息
    http_status         INTEGER,                            -- HTTP 狀態碼（IO 錯誤時 0）
    duration_ms         BIGINT,                             -- 呼叫耗時（毫秒）

    -- 時間戳記
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payuni_audit_trade_no ON payuni_audit_logs(trade_no);
-- refund_no 多數列為 NULL，partial index 省空間
CREATE INDEX idx_payuni_audit_refund_no ON payuni_audit_logs(refund_no) WHERE refund_no IS NOT NULL;
CREATE INDEX idx_payuni_audit_created_at ON payuni_audit_logs(created_at);
