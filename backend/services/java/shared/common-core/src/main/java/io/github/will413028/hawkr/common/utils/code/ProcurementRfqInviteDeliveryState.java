package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 單一 recipient（invite）的投遞狀態（issue #2450 Slice 2b）。這是「有沒有交出去」的投遞軸，與 `ProcurementRfqPublicationApprovalState`（核准／發布軸）是不同實體、不同生命週期，不得合併或互相轉譯。**`SENT` 也不是收件人有沒有看到／回應**——delivered／seen 屬 #2539，是另一條尚未存在的回饋軸。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementRfqInviteDeliveryState implements CodeMeta {

    NOT_SENT("NOT_SENT", "NOT_SENT", "尚未送出", "這條 lane 目前沒有真實投遞管道（EXTERNAL，T17 之前），系統尚未嘗試送出"),
    QUEUED("QUEUED", "QUEUED", "待送出", "投遞任務已排入 outbox，尚未交給投遞管道"),
    SENT("SENT", "SENT", "已送出", "已交給投遞管道。**不代表賣家已收到，更不代表已看到**——delivered／seen 屬 #2539"),
    FAILED("FAILED", "FAILED", "送出失敗", "投遞重試耗盡轉終態");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementRfqInviteDeliveryState(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementRfqInviteDeliveryState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRfqInviteDeliveryState::code, e -> e));

    private static final Map<String, ProcurementRfqInviteDeliveryState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRfqInviteDeliveryState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementRfqInviteDeliveryState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementRfqInviteDeliveryState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
