package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni 信用卡、Apple Pay、Google Pay 與 ATM 支付通道開通狀態。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayuniPaymentChannelStatus implements CodeMeta {

    ACTIVE("ACTIVE", "1", "啟用", "PayUni 支付通道已啟用"),
    DISABLED("DISABLED", "2", "不啟用", "PayUni 支付通道不啟用"),
    NOT_APPLIED("NOT_APPLIED", "3", "未申請", "尚未申請此 PayUni 支付通道"),
    FORCED_CLOSED("FORCED_CLOSED", "4", "強制關閉", "PayUni 已強制關閉此支付通道"),
    UNDER_REVIEW("UNDER_REVIEW", "9", "審核中", "PayUni 支付通道審核中"),
    REJECTED("REJECTED", "92", "審核失敗", "PayUni 支付通道審核未通過"),
    OTHER_CLOSED("OTHER_CLOSED", "99", "其他關閉", "PayUni 支付通道因其他原因關閉");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayuniPaymentChannelStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PayuniPaymentChannelStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniPaymentChannelStatus::code, e -> e));

    private static final Map<String, PayuniPaymentChannelStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniPaymentChannelStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayuniPaymentChannelStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayuniPaymentChannelStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
