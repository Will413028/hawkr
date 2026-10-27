package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一次 RFQ 發布的狀態。⛔ `PUBLISHED` **只代表 server transaction 完成**，不代表 Wish Pool 已建索引、也不代表賣家已收到或已看到（issue #2450 Scope 9、doc 64 §4.2）；投遞與索引的真相由 #2539 的 typed evidence 更新。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementPublicationState implements CodeMeta {

    PUBLISHED("PUBLISHED", "PUBLISHED", "已建立發布", "server transaction 完成；**不代表已投遞或已被看到**"),
    SUPERSEDED("SUPERSEDED", "SUPERSEDED", "已被後續發布取代", "同一 request 的新一次發布取代本次"),
    CANCELLED("CANCELLED", "CANCELLED", "已取消", "買家撤回本次發布");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementPublicationState(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementPublicationState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementPublicationState::code, e -> e));

    private static final Map<String, ProcurementPublicationState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementPublicationState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementPublicationState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementPublicationState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
