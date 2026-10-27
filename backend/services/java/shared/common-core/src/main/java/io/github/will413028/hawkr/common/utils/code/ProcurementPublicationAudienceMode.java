package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * publication 的受眾種類（issue #2450 Scope 2）。兩支 pipeline **不共用完成定義**（doc 64 §4.2）：Wish Pool 的完成是「已建索引」，Direct 的完成是「已投遞給指定對象」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementPublicationAudienceMode implements CodeMeta {

    WISH_POOL("WISH_POOL", "WISH_POOL", "許願池", "公開池，不指定對象；不需要 `candidateSet`，空 recipient list 不得冒充本模式"),
    DIRECT_RECIPIENTS("DIRECT_RECIPIENTS", "DIRECT_RECIPIENTS", "指定對象", "引用 `candidateSet` 與 ordered recipient 身分快照");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementPublicationAudienceMode(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementPublicationAudienceMode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementPublicationAudienceMode::code, e -> e));

    private static final Map<String, ProcurementPublicationAudienceMode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementPublicationAudienceMode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementPublicationAudienceMode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementPublicationAudienceMode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
