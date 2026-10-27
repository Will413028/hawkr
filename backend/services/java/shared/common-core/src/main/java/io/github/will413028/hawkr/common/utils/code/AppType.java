package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 請求所屬的 App 身份，決定 PayUni 回調完成後 deep link / web URL 該指向買家版或賣家版。與 `AccountType` 為不同維度：`AccountType` 是 JWT 帳號權限類別（含 `SELLER_AND_BUYER`、`ADMIN`），`AppType` 是運行時這個請求是從哪個 App / 網頁發起的二元選擇。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AppType implements CodeMeta {

    BUYER("BUYER", "BUYER", "買家", "買家版 App / 買家版網頁"),
    SELLER("SELLER", "SELLER", "賣家", "賣家版 App / 賣家版網頁");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AppType(String code, String value, String label, String description) {
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

    private static final Map<String, AppType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AppType::code, e -> e));

    private static final Map<String, AppType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AppType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AppType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AppType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
