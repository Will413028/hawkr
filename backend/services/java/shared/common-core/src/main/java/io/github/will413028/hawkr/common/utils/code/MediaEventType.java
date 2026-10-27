package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Media Service publish 給其他服務的事件類型。對齊 `{publisher_service}.{domain}.{action}` 命名規則。Routing key 為點分式 `import.asset.downloaded` 寫於 Media 端 `MqConstants`，本 enum 作 ObsLogger event name 與 trace metadata。本期首次啟用 `hawkr.media` exchange（36 號既有「宣告但未實際使用」）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum MediaEventType implements CodeMeta {

    MEDIA_IMPORT_ASSET_DOWNLOADED("MEDIA_IMPORT_ASSET_DOWNLOADED", "MEDIA_IMPORT_ASSET_DOWNLOADED", "import_asset 下載完成（async callback）", "Routing key `import.asset.downloaded`；Media 內部下載 worker 完成後 publish，product-import-service `MediaCallbackListener` 消費");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    MediaEventType(String code, String value, String label, String description) {
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

    private static final Map<String, MediaEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(MediaEventType::code, e -> e));

    private static final Map<String, MediaEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(MediaEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static MediaEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static MediaEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
