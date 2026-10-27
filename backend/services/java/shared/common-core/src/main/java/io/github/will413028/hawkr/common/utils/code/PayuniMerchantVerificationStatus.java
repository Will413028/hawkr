package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni 商店（merchant）審核狀態。沿用既有 Go `domain.PayuniMerStatus`
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayuniMerchantVerificationStatus implements CodeMeta {

    ACTIVE("ACTIVE", "1", "啟用", "PayUni 商店已啟用，可收款"),
    CLOSED("CLOSED", "2", "關閉", "PayUni 商店已關閉"),
    DENIED("DENIED", "8", "拒絕", "PayUni 拒絕商店申請（與 92 同樣為終結退件）"),
    UNDER_REVIEW("UNDER_REVIEW", "9", "審核中", "PayUni 審核中"),
    PENDING_INFO("PENDING_INFO", "91", "待補資料", "PayUni 要求補充資料"),
    REJECTED("REJECTED", "92", "審核失敗", "PayUni 審核未通過"),
    OTHER_CLOSED("OTHER_CLOSED", "99", "其它關閉", "PayUni 商店關閉（其他原因）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayuniMerchantVerificationStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PayuniMerchantVerificationStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniMerchantVerificationStatus::code, e -> e));

    private static final Map<String, PayuniMerchantVerificationStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniMerchantVerificationStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayuniMerchantVerificationStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayuniMerchantVerificationStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
