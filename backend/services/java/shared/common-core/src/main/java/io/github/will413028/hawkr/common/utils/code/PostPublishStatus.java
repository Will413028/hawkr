package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 貼文的發布狀態。cms_post 原本只有 hidden_at 這個平台強制下架旗標、沒有發布狀態概念；代行營運要能建立草稿而不讓貼文當場進入買家 feed，故新增本欄。「已排程待發布」不是本 enum 的值，由 cms_post.schedule_state = pending 表達：publish_status 答「貼文現在對買家可不可見」、schedule_state 答「有沒有一個待執行的排程任務」，同一個事實不放兩個載體。碼段依據：4000~4399 已被 ProductStatus / LabelPrintStatus / TcatCallStatus / PayUniSubmitStatus 佔用，4400 起為最小的連續空段。詳見 [56 §6.5.3](../詳細設計書/56-ADM%20代行營運（Delegated%20Operations）詳細設計書.md)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PostPublishStatus implements CodeMeta {

    DRAFT("DRAFT", "4401", "未發布", "尚未發布，僅賣家與代行者可見"),
    PUBLISHED("PUBLISHED", "4402", "已發布", "買家可見；賣家自建貼文吃 DB DEFAULT 即落在此值"),
    UNPUBLISHED("UNPUBLISHED", "4403", "已下架", "曾發布後被撤下，與 hidden_at 的平台強制下架是兩回事");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PostPublishStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PostPublishStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PostPublishStatus::code, e -> e));

    private static final Map<String, PostPublishStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PostPublishStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PostPublishStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PostPublishStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
