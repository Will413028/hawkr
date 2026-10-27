package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * ADM 駁回（`REJECTED`）或撤銷（`REVOKED`）認證申請時必填的結構化原因碼，兩個動作共用同一值域。⚠️ 與 `public_reason` 是兩件事：本值域是**可篩選、可統計、前端可自行翻譯**的碼，`public_reason` 是 ADM 自由填寫、直接展示給賣家的補充說明；只有後者不足以做治理分析，只有前者則說不清個案。缺 `reason_code` 時回 `3028 CERT_REVIEW_REASON_REQUIRED`。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationRejectReason implements CodeMeta {

    CERT_NOT_FOUND("CERT_NOT_FOUND", "CERT_NOT_FOUND", "查無此證書", "政府資料庫查無此編號（`gov_verify_state = NOT_FOUND`），或 ADM 人工查證後確認不存在"),
    CERT_EXPIRED("CERT_EXPIRED", "CERT_EXPIRED", "證書已失效", "證書已過期、終止或暫停（`gov_verify_state = EXPIRED`）"),
    CERT_PRODUCT_MISMATCH("CERT_PRODUCT_MISMATCH", "CERT_PRODUCT_MISMATCH", "證書與商品不符", "證書涵蓋的品項不含本商品。3703 CAS 的典型情境：`Emblem_ID` 是工廠標誌編號，查得到只證明該工廠有 CAS 資格"),
    CERT_HOLDER_MISMATCH("CERT_HOLDER_MISMATCH", "CERT_HOLDER_MISMATCH", "證書非本店所有", "證書持有人不是本賣家。政府 API 只驗「號碼存在」不驗「歸屬」，這一類只能靠人工比對業者名"),
    CERT_EVIDENCE_UNCLEAR("CERT_EVIDENCE_UNCLEAR", "CERT_EVIDENCE_UNCLEAR", "資料不清", "提供的編號或佐證資料不足以判定，需賣家補充後重送"),
    OTHER("OTHER", "OTHER", "其他原因", "不屬於上列任一類；ADM 必須另填 `public_reason` 說明");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationRejectReason(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, ProductCertificationRejectReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationRejectReason::code, e -> e));

    private static final Map<String, ProductCertificationRejectReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationRejectReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationRejectReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationRejectReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
