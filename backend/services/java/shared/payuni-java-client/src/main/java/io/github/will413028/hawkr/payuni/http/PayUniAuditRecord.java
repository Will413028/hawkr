package io.github.will413028.hawkr.payuni.http;

import java.util.Map;

/**
 * PayUni 稽核中性欄位 record（helper 產物，供各服務 logger 直接映射落庫）。
 * <p>
 * 由 {@link PayUniAuditMapper#toAuditRecord} 從 {@link PayUniExchangeContext} 純函式映射而成，
 * 憑證欄位已遮罩、VARCHAR 欄已 clamp。不含 {@code direction}（由落庫端 logger 依出向/入向設定）
 * 與 {@code createdAt}（由 MetaObjectHandler 自動填充）。欄位語意對齊 payment/shipping 同構表
 * {@code payuni_audit_logs}；逐欄搬運用 {@link PayUniAuditMapper#applyTo}。
 *
 * @param tradeNo           交易單號（出向自 request params MerTradeNo / ShipTradeNo；查詢類 API 無單號 → null）
 * @param refundNo          退款/退貨單號（無則 null）
 * @param merId             PayUni 商家編號（request params MerID 優先，無則外層 merId/agentId）
 * @param apiPath           API 路徑
 * @param requestParams     業務參數（憑證欄位已遮罩）
 * @param responseRaw       原始回應全文（不截斷；HTML 內憑證 input 已遮罩；二進位檔為 null）
 * @param responseDecrypted 解密後內層（PayUni 原生 PascalCase key；憑證欄位已遮罩）
 * @param outerStatus       外層 Status（含 PARSE_ERROR / HASH_INVALID）
 * @param outerMessage      外層 Message
 * @param errorCode         傳輸層錯誤碼
 * @param errorMessage      傳輸層錯誤訊息
 * @param httpStatus        HTTP 狀態碼（IO 錯誤時 0）
 * @param durationMs        呼叫耗時（毫秒）
 */
public record PayUniAuditRecord(
        String tradeNo,
        String refundNo,
        String merId,
        String apiPath,
        Map<String, Object> requestParams,
        String responseRaw,
        Map<String, Object> responseDecrypted,
        String outerStatus,
        String outerMessage,
        String errorCode,
        String errorMessage,
        Integer httpStatus,
        Long durationMs
) {
}
