package io.github.will413028.hawkr.payuni.http;

import java.util.Map;

/**
 * 單次 PayUni exchange 的聚合 context（client → 落庫端）。
 * <p>
 * 採「可變累加器」設計：{@link PayUniHttpClient} 於 exchange 開始時建立、沿途填欄位，
 * 於 {@code finally} 統一觸發一次 {@link PayUniRequestLogger#onExchange}，
 * 保證成功與各類失敗路徑皆恰好一次、不遺漏、不重複。
 * <p>
 * 憑證欄位（如 {@code CustomerToken} / {@code Hash}）不在此遮罩——由落庫端純函式 helper 統一遮罩後才寫庫。
 */
public class PayUniExchangeContext {

    /** 外層 MerID / AgentID。 */
    private final String merId;
    /** API 路徑（如 {@code /api/trade/grant}）。 */
    private final String path;
    /** 加密前業務參數（明文；憑證欄位由落庫端遮罩）。 */
    private final Map<String, String> requestParams;

    /** 原始回應 body 全文（JSON 或 HTML；不截斷）。 */
    private String rawResponseBody;
    /** 外層 {@code Status}；parse/decrypt/hash 失敗記 {@code PARSE_ERROR} / {@code HASH_INVALID}。 */
    private String outerStatus;
    /** 外層 {@code Message}。 */
    private String outerMessage;
    /** 解密後內層 Map（PayUni 原生 PascalCase key；可為 null）。 */
    private Map<String, String> decryptedPayload;
    /** 傳輸層錯誤碼（IOException / 非 200 / EMPTY response）。 */
    private String errorCode;
    /** 傳輸層錯誤訊息。 */
    private String errorMessage;
    /** HTTP 狀態碼（IO 錯誤時 0）。 */
    private Integer httpStatus;
    /** 呼叫耗時（毫秒）。 */
    private Long durationMs;

    public PayUniExchangeContext(String merId, String path, Map<String, String> requestParams) {
        this.merId = merId;
        this.path = path;
        this.requestParams = requestParams;
    }

    public String getMerId() { return merId; }
    public String getPath() { return path; }
    public Map<String, String> getRequestParams() { return requestParams; }

    public String getRawResponseBody() { return rawResponseBody; }
    public void setRawResponseBody(String rawResponseBody) { this.rawResponseBody = rawResponseBody; }

    public String getOuterStatus() { return outerStatus; }
    public void setOuterStatus(String outerStatus) { this.outerStatus = outerStatus; }

    public String getOuterMessage() { return outerMessage; }
    public void setOuterMessage(String outerMessage) { this.outerMessage = outerMessage; }

    public Map<String, String> getDecryptedPayload() { return decryptedPayload; }
    public void setDecryptedPayload(Map<String, String> decryptedPayload) { this.decryptedPayload = decryptedPayload; }

    public String getErrorCode() { return errorCode; }
    public void setErrorCode(String errorCode) { this.errorCode = errorCode; }

    public String getErrorMessage() { return errorMessage; }
    public void setErrorMessage(String errorMessage) { this.errorMessage = errorMessage; }

    public Integer getHttpStatus() { return httpStatus; }
    public void setHttpStatus(Integer httpStatus) { this.httpStatus = httpStatus; }

    public Long getDurationMs() { return durationMs; }
    public void setDurationMs(Long durationMs) { this.durationMs = durationMs; }
}
