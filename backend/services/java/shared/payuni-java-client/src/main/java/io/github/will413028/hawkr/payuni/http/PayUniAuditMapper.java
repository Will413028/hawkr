package io.github.will413028.hawkr.payuni.http;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;

/**
 * PayUni 稽核純函式 helper：{@link PayUniExchangeContext} → {@link PayUniAuditRecord}（含憑證遮罩、欄寬 clamp），
 * 以及 {@link PayUniAuditRecord} → {@link PayUniAuditLogRow} 的 13 欄搬運（{@link #applyTo}）。
 * <p>
 * 無狀態、無 DB 依賴。各服務 logger（payment / shipping）呼叫本 helper 取得遮罩後中性 record，
 * 再補上 direction 落庫，維持「共用模組不依賴服務 DB layer」邊界（見 11 §5.5）。
 * <p>
 * 遮罩對象為可重放的機敏憑證（token / hash / 卡號），非爭議標的本身；收件人 PII 由落庫端決策是否明文保留
 * （shipping 明文保留，見 06 §10.5），故本 helper 不遮罩 PII。
 */
public final class PayUniAuditMapper {

    private PayUniAuditMapper() {
    }

    /** 交易單號候選 key（依序取第一個非空）。 */
    private static final String[] TRADE_NO_KEYS = {"MerTradeNo", "ShipTradeNo"};
    /** 退款/退貨單號候選 key。 */
    private static final String[] REFUND_NO_KEYS = {"RefundNo", "MerRefundNo"};

    /** 憑證類 key（小寫比對，命中即遮罩）——可重放機敏值，遮罩防洩漏。 */
    private static final Set<String> CREDENTIAL_KEYS_LOWER = Set.of(
            "customertoken", "hash", "hashinfo", "token",
            "cardno", "card", "cvc", "cvv", "pan");

    /**
     * HTML hidden input 憑證遮罩 pattern：命中 {@code name='<憑證欄名>' value='...'}（單/雙引號皆可），
     * 遮罩 value 屬性值。PayUni HTML form 為 name-先-value-後格式（見 ShipmentLabelService 註解）。
     */
    private static final Pattern HTML_CREDENTIAL_INPUT = Pattern.compile(
            "(name\\s*=\\s*['\"](?:" + CREDENTIAL_KEYS_LOWER.stream()
                    .map(Pattern::quote).collect(Collectors.joining("|"))
                    + ")['\"]\\s+value\\s*=\\s*['\"])([^'\"]*)(['\"])",
            Pattern.CASE_INSENSITIVE);

    /**
     * 將可變 exchange context 映射為遮罩後、欄寬 clamp 後的中性稽核 record。
     * <p>
     * VARCHAR 欄超寬時截斷（trade_no/refund_no≤32、mer_id≤20、outer_status≤20、error_code≤50、
     * api_path≤100），保證稽核列 INSERT 必成功（「稽核列必落」不變式）。
     */
    public static PayUniAuditRecord toAuditRecord(PayUniExchangeContext ctx) {
        Map<String, String> req = ctx.getRequestParams();
        // mer_id 語意：業務參數內層 MerID 優先（代理商 API 外層是 AgentID 固定配置，內層才是子商家）
        String merId = req != null && req.get("MerID") != null && !req.get("MerID").isBlank()
                ? req.get("MerID") : ctx.getMerId();
        return new PayUniAuditRecord(
                clamp(firstNonBlank(req, TRADE_NO_KEYS), 32),
                clamp(firstNonBlank(req, REFUND_NO_KEYS), 32),
                clamp(merId, 20),
                clamp(ctx.getPath(), 100),
                maskCredentials(req),
                maskHtmlCredentials(ctx.getRawResponseBody()),
                maskCredentials(ctx.getDecryptedPayload()),
                clamp(ctx.getOuterStatus(), 20),
                ctx.getOuterMessage(),
                clamp(ctx.getErrorCode(), 50),
                ctx.getErrorMessage(),
                ctx.getHttpStatus(),
                ctx.getDurationMs()
        );
    }

    /**
     * 13 欄搬運：record → 落庫列（direction / createdAt 不在此，由 logger / MetaObjectHandler 負責）。
     */
    public static void applyTo(PayUniAuditRecord record, PayUniAuditLogRow row) {
        row.setTradeNo(record.tradeNo());
        row.setRefundNo(record.refundNo());
        row.setMerId(record.merId());
        row.setApiPath(record.apiPath());
        row.setRequestParams(record.requestParams());
        row.setResponseRaw(record.responseRaw());
        row.setResponseDecrypted(record.responseDecrypted());
        row.setOuterStatus(record.outerStatus());
        row.setOuterMessage(record.outerMessage());
        row.setErrorCode(record.errorCode());
        row.setErrorMessage(record.errorMessage());
        row.setHttpStatus(record.httpStatus());
        row.setDurationMs(record.durationMs());
    }

    /**
     * 回傳遮罩憑證欄位後的新 Map（保留原 key 順序，值型別放寬為 Object 供 JSONB 落庫）；輸入 null 回 null。
     */
    public static Map<String, Object> maskCredentials(Map<String, String> source) {
        if (source == null) {
            return null;
        }
        Map<String, Object> masked = new LinkedHashMap<>(source.size());
        for (Map.Entry<String, String> e : source.entrySet()) {
            String key = e.getKey();
            String value = e.getValue();
            if (key != null && CREDENTIAL_KEYS_LOWER.contains(key.toLowerCase())) {
                masked.put(key, mask(value));
            } else {
                masked.put(key, value);
            }
        }
        return masked;
    }

    /**
     * HTML 回應內憑證遮罩：對 {@code <input name='CustomerToken' value='...'>} 等 hidden input 的
     * value 屬性值遮罩（callApiHtml 的 HTML 全文落 response_raw 時防憑證洩漏）。
     * JSON body 無此 pattern，原樣通過；輸入 null 回 null。
     */
    public static String maskHtmlCredentials(String html) {
        if (html == null || html.isEmpty()) {
            return html;
        }
        Matcher m = HTML_CREDENTIAL_INPUT.matcher(html);
        if (!m.find()) {
            return html;
        }
        StringBuilder sb = new StringBuilder();
        do {
            m.appendReplacement(sb, Matcher.quoteReplacement(m.group(1) + mask(m.group(2)) + m.group(3)));
        } while (m.find());
        m.appendTail(sb);
        return sb.toString();
    }

    /** 遮罩單一值：長度 ≤8 全遮；>8 保留前後各 2 碼，中段以 {@code *} 取代。 */
    static String mask(String value) {
        if (value == null || value.isEmpty()) {
            return value;
        }
        int len = value.length();
        if (len <= 8) {
            return "*".repeat(len);
        }
        return value.substring(0, 2) + "*".repeat(len - 4) + value.substring(len - 2);
    }

    /** VARCHAR 欄寬防護：超過 maxLen 截斷（null 安全）。 */
    public static String clamp(String value, int maxLen) {
        if (value == null || value.length() <= maxLen) {
            return value;
        }
        return value.substring(0, maxLen);
    }

    private static String firstNonBlank(Map<String, String> params, String[] keys) {
        if (params == null) {
            return null;
        }
        for (String key : keys) {
            String v = params.get(key);
            if (v != null && !v.isBlank()) {
                return v;
            }
        }
        return null;
    }
}
