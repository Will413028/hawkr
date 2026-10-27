package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 被檢舉對象的類型，決定 social-service 要去哪張表取快照、處置時要下架哪一種內容（`internal/usecase/dispatcher.go` 依本值分派）。權威值域是 DB CHECK 而非 `api/proto/social/social.proto` 的註解。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ReportTargetType implements CodeMeta {

    REVIEW("REVIEW", "REVIEW", "商品評論", "商品評論（review-service 擁有，social 只持 target_id）"),
    POST("POST", "POST", "貼文", "賣家貼文 `cms_post`"),
    POST_COMMENT("POST_COMMENT", "POST_COMMENT", "貼文留言", "貼文底下的留言 `cms_comment`"),
    SELLER_FORUM("SELLER_FORUM", "SELLER_FORUM", "討論區", "賣家討論區內容"),
    CHAT_MESSAGE("CHAT_MESSAGE", "CHAT_MESSAGE", "聊天訊息", "IM 訊息（im-service 擁有）"),
    USER("USER", "USER", "使用者", "檢舉對象是人而非單一則內容；此類不下架內容，只走警告／停權");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ReportTargetType(String code, String value, String label, String description) {
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

    private static final Map<String, ReportTargetType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportTargetType::code, e -> e));

    private static final Map<String, ReportTargetType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportTargetType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ReportTargetType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ReportTargetType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
