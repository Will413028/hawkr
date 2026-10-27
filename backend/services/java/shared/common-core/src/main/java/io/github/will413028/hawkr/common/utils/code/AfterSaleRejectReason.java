package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後拒絕原因
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleRejectReason implements CodeMeta {

    IMPROPER_STORAGE("IMPROPER_STORAGE", "9101", "買家保存不當", "商品因買家保存不當導致問題"),
    EXPIRED_CLAIM("EXPIRED_CLAIM", "9102", "超過申訴時效", "超過可申訴的時間限制"),
    INSUFFICIENT_EVIDENCE("INSUFFICIENT_EVIDENCE", "9103", "證據不足", "提供的證據不足以證明問題"),
    NORMAL_CONDITION("NORMAL_CONDITION", "9104", "商品狀態正常", "商品經確認狀態正常"),
    SELLER_NO_RESPONSE("SELLER_NO_RESPONSE", "9105", "賣家逾時未回應", "賣家未在處理期限內回應，由 `CaseProcessTimeoutWorker` 在 `timeout-policy=AUTO_REJECT` 下系統代為拒絕（§5.10）"),
    OTHER("OTHER", "9199", "其他原因", "其他拒絕原因");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleRejectReason(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleRejectReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleRejectReason::code, e -> e));

    private static final Map<String, AfterSaleRejectReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleRejectReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleRejectReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleRejectReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
