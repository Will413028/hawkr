package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 社群領域 MQ 事件類型，由 social-service `publisher.go` 發布到 `hawkr.content` exchange。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `comment.created` / `post.liked` 等，寫於 `pkg/mq/exchanges.go`；event_type 僅作 metadata。對齊 OrderEventType / BuyerEventType 設計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SocialEventType implements CodeMeta {

    POST_CREATED("POST_CREATED", "POST_CREATED", "貼文已建立", "Routing key `post.created`；notification-service 對追蹤者推播"),
    POST_LIKED("POST_LIKED", "POST_LIKED", "貼文已按讚", "Routing key `post.liked`；notification-service 對作者推播"),
    POST_HIDDEN_BY_PLATFORM("POST_HIDDEN_BY_PLATFORM", "POST_HIDDEN_BY_PLATFORM", "貼文已被平台隱藏", "Routing key `post.hidden_by_platform`（既有 `pkg/mq/exchanges.go::RoutingKeyPostHiddenByPlatform`）；admin 隱藏審核違規內容"),
    COMMENT_CREATED("COMMENT_CREATED", "COMMENT_CREATED", "留言已建立（top-level）", "Routing key `comment.created`；notification 對貼文作者推播 + gateway SSE 推到所有訂閱該 post 的 client；payload 含 nested CommentDTO（fat event）"),
    COMMENT_REPLIED("COMMENT_REPLIED", "COMMENT_REPLIED", "留言已回覆", "Routing key `comment.replied`；notification 對被回覆者推播 + gateway SSE；payload 同 fat"),
    COMMENT_UPDATED("COMMENT_UPDATED", "COMMENT_UPDATED", "留言已編輯", "Routing key `comment.updated`；gateway SSE 推到該 post 訂閱者；無 notification consumer"),
    COMMENT_DELETED("COMMENT_DELETED", "COMMENT_DELETED", "留言已刪除", "Routing key `comment.deleted`；gateway SSE 推到該 post 訂閱者；無 notification consumer"),
    COMMENT_LIKE_CHANGED("COMMENT_LIKE_CHANGED", "COMMENT_LIKE_CHANGED", "留言按讚數變化（含 LIKE / UNLIKE）", "Routing key `comment.like_changed`；**取代 COMMENT_LIKED**（2026-06 直接刪除舊 enum 無過渡期）；gateway SSE 推 likeCount + action；notification 改訂閱本 routing key 過濾 action=LIKE 才推播"),
    COUNTER_DELTA("COUNTER_DELTA", "COUNTER_DELTA", "非同步 counter 維護（post/comment 的 comment_count / like_count / reply_count +1/-1）", "Routing key `counter.delta`（2026-07-31 新增）；social-service 自身消費（queue `social.counter.delta`），取代原本同步 Incr/Decr 呼叫；**非冪等**，10 分鐘 reconciler 兜底");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SocialEventType(String code, String value, String label, String description) {
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

    private static final Map<String, SocialEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SocialEventType::code, e -> e));

    private static final Map<String, SocialEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SocialEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SocialEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SocialEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
