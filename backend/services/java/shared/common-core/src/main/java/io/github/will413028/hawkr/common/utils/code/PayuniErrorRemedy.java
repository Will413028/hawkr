package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni 申請失敗時賣家該採取的動作類別，決定 App 顯示哪一種行動入口
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayuniErrorRemedy implements CodeMeta {

    SELLER_FIX("SELLER_FIX", "SELLER_FIX", "可自行修正", "賣家改得掉的資料問題，App 顯示重新提交並跳到對應補件步驟"),
    CONTACT_SUPPORT("CONTACT_SUPPORT", "CONTACT_SUPPORT", "需聯繫客服", "平台端設定或 PayUni 帳戶狀態問題，賣家自行操作無法解決"),
    RETRY_LATER("RETRY_LATER", "RETRY_LATER", "稍後再試", "PayUni 端暫時性失敗，稍後重新確認申請狀態即可");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayuniErrorRemedy(String code, String value, String label, String description) {
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

    private static final Map<String, PayuniErrorRemedy> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniErrorRemedy::code, e -> e));

    private static final Map<String, PayuniErrorRemedy> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayuniErrorRemedy::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayuniErrorRemedy fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayuniErrorRemedy fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
