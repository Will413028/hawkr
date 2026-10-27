package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 申訴案件時間軸的事件種類（**7 值**）。⚠️ **不是 MQ 事件**（MQ 事件在 InvoiceEventType），是 append-only 時間軸表的分類欄；S09／A07 兩份 UI 分頁都從這張表投影，不各自維護狀態。⚠️ **`REOPENED` 在這裡是「事件」不是「狀態」**——`InvoiceComplaintStatus` 已依 58 §7.3.5 刪除 `REOPENED` 狀態（理由：它與 `OPEN` 行為完全相同），但「買家重開過」這件事必須在時間軸上留痕，`invoice_complaints.reopen_count`（上限 2）配這一列事件即為其載體。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceComplaintEventType implements CodeMeta {

    CREATED("CREATED", "CREATED", "買家提出申訴", "actor_type = BUYER"),
    SELLER_REPLY("SELLER_REPLY", "SELLER_REPLY", "賣家回覆", "actor_type = SELLER；action 細分見下方註"),
    ESCALATED("ESCALATED", "ESCALATED", "升級平台處理", "actor_type = SELLER（請求協助，K03）／BUYER（不接受回覆）／SYSTEM（逾期 24 小時自動升級，K05）。**對應狀態 `ESCALATED_TO_ADM`**"),
    PLATFORM_DECISION("PLATFORM_DECISION", "PLATFORM_DECISION", "平台判定", "actor_type = ADMIN；必填理由"),
    REOPENED("REOPENED", "REOPENED", "買家重開案件", "actor_type = BUYER；reopen_count 上限 2"),
    WITHDRAWN("WITHDRAWN", "WITHDRAWN", "買家撤回", "actor_type = BUYER"),
    SLA_OVERDUE("SLA_OVERDUE", "SLA_OVERDUE", "逾 3 工作天未回覆", "actor_type = SYSTEM；由排程寫入，只發生一次");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceComplaintEventType(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceComplaintEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceComplaintEventType::code, e -> e));

    private static final Map<String, InvoiceComplaintEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceComplaintEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceComplaintEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceComplaintEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
