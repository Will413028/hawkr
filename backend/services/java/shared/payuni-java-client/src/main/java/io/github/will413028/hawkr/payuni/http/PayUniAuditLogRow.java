package io.github.will413028.hawkr.payuni.http;

import java.util.Map;

/**
 * payuni_audit_logs 落庫列的中性 setter 介面（無任何 DB 依賴）。
 * <p>
 * payment / shipping 的 {@code PayUniAuditLogEntity}（Lombok {@code @Data} setter 簽章天然相符）
 * 實作本介面，使 {@link PayUniAuditMapper#applyTo} 能以單一實作完成
 * {@link PayUniAuditRecord} → entity 的 13 欄搬運，消除兩服務逐欄 set 重複。
 * {@code direction} / {@code createdAt} 不在此介面（前者由 logger 設定、後者由 MetaObjectHandler 填充）。
 */
public interface PayUniAuditLogRow {

    void setTradeNo(String tradeNo);

    void setRefundNo(String refundNo);

    void setMerId(String merId);

    void setApiPath(String apiPath);

    void setRequestParams(Map<String, Object> requestParams);

    void setResponseRaw(String responseRaw);

    void setResponseDecrypted(Map<String, Object> responseDecrypted);

    void setOuterStatus(String outerStatus);

    void setOuterMessage(String outerMessage);

    void setErrorCode(String errorCode);

    void setErrorMessage(String errorMessage);

    void setHttpStatus(Integer httpStatus);

    void setDurationMs(Long durationMs);
}
