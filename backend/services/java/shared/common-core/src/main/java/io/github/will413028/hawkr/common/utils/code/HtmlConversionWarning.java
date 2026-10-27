package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * HTML→blocks JSON 轉換時的降級警告類型，用於前端翻譯給賣家看。詳見 [40 號設計書 §12.5](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#125-警告類型對照)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum HtmlConversionWarning implements CodeMeta {

    STYLE_UNSUPPORTED_FLUTTER("STYLE_UNSUPPORTED_FLUTTER", "STYLE_UNSUPPORTED_FLUTTER", "樣式僅 Web 渲染", "例如 underline，Flutter 端會忽略"),
    HYPERLINK_NOT_SUPPORTED("HYPERLINK_NOT_SUPPORTED", "HYPERLINK_NOT_SUPPORTED", "超連結未保留", "僅保留文字"),
    TABLE_FLATTENED("TABLE_FLATTENED", "TABLE_FLATTENED", "表格已展平", "展平為純文字段落"),
    MEDIA_DROPPED("MEDIA_DROPPED", "MEDIA_DROPPED", "已忽略媒體元素", "video / audio / iframe"),
    UNSUPPORTED_TAG_DEGRADED("UNSUPPORTED_TAG_DEGRADED", "UNSUPPORTED_TAG_DEGRADED", "tag 降級", "不認的 tag 取 innerText"),
    COLOR_FORMAT_INVALID("COLOR_FORMAT_INVALID", "COLOR_FORMAT_INVALID", "顏色格式不支援", "僅接受 #RRGGBB"),
    FONT_SIZE_OUT_OF_RANGE("FONT_SIZE_OUT_OF_RANGE", "FONT_SIZE_OUT_OF_RANGE", "字級超出範圍", "已調整為 12~36");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    HtmlConversionWarning(String code, String value, String label, String description) {
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

    private static final Map<String, HtmlConversionWarning> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(HtmlConversionWarning::code, e -> e));

    private static final Map<String, HtmlConversionWarning> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(HtmlConversionWarning::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static HtmlConversionWarning fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static HtmlConversionWarning fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
