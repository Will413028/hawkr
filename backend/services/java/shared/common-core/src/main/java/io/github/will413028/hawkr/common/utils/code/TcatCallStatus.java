package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 黑貓宅配叫車（PayUni `call_tcat` API）的外部呼叫四態狀態機。賣家點「呼叫黑貓」寫入 `REQUESTED`，PayUni 同步 API 成功 → `SUCCEEDED`、retry 後仍失敗 → `FAILED`（同時 `provider_message` 落地 PayUni 原始錯誤）。前端依此控制「再次預約」按鈕；reconciliation 不重打（PayUni 黑貓叫車為一次性同步 API，無 webhook，失敗只能由賣家重試）。詳見 06-Shipping 設計書 §2.4 / §5.4。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum TcatCallStatus implements CodeMeta {

    NONE("NONE", "4201", "未叫件", "宅配物流單建立後預設值；尚未呼叫黑貓"),
    REQUESTED("REQUESTED", "4202", "叫件中", "已呼叫 PayUni call_tcat，等待同步回應（短暫過渡狀態）"),
    SUCCEEDED("SUCCEEDED", "4203", "已叫件", "PayUni 回傳成功，黑貓司機已接單，`tcat_call_time` 同步寫入"),
    FAILED("FAILED", "4204", "叫件失敗", "PayUni 同步 API retry 後仍失敗（地址錯誤、假日、配額等），需賣家修正後重試");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    TcatCallStatus(String code, String value, String label, String description) {
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

    private static final Map<String, TcatCallStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(TcatCallStatus::code, e -> e));

    private static final Map<String, TcatCallStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(TcatCallStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static TcatCallStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static TcatCallStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
