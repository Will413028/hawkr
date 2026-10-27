package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 請求端類型。前端發起付款 / ATM 退款頁時必須帶入，後端據以決定 PayUni 回調完成後的「最終出口」（行動 App 走 deep link / 網頁走 WEB_FRONTEND_URL）。詳見 [05-支付服務設計書 §5.9](../詳細設計書/05-支付服務（Payment%20Service）詳細設計書.md#59-付款頁面導回流程)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ClientType implements CodeMeta {

    MOBILE_APP("MOBILE_APP", "MOBILE_APP", "行動 App", "iOS 原生 App（含內嵌 WebView 走 deep link 攔截）"),
    WEB_BROWSER("WEB_BROWSER", "WEB_BROWSER", "網頁瀏覽器", "PC / 行動瀏覽器（直接走 https，不可解析 deep link scheme）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ClientType(String code, String value, String label, String description) {
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

    private static final Map<String, ClientType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ClientType::code, e -> e));

    private static final Map<String, ClientType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ClientType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ClientType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ClientType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
