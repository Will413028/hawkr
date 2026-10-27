package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 檢舉原因。權威值域是 DB CHECK `chk_reports_reason_code`；`social.proto` 的註解曾列出 `HATE`／`ADULT` 兩個從未存在於 CHECK 的值，是「照抄 proto 註解就會抄到死值」的現成案例。哪些原因適用於哪種 `ReportTargetType` 是**業務規則不是碼表**（例如檢舉「使用者」時不提供食安疑慮），依 §8 放在呼叫端的 helper，不得寫進本 enum。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ReportReasonCode implements CodeMeta {

    SPAM("SPAM", "SPAM", "垃圾訊息 / 廣告", "重複張貼、與內容無關的推銷"),
    SEXUAL("SEXUAL", "SEXUAL", "色情 / 不當內容", "情色露骨或不適合公開展示的內容"),
    FRAUD("FRAUD", "FRAUD", "詐騙", "假交易、假優惠、導向站外付款"),
    HARASSMENT("HARASSMENT", "HARASSMENT", "騷擾 / 仇恨言論", "針對特定個人或群體的攻擊"),
    FOOD_SAFETY("FOOD_SAFETY", "FOOD_SAFETY", "食品安全疑慮", "與食品衛生、標示、保存有關的疑慮；不適用於 `USER` 類檢舉"),
    POLITICAL("POLITICAL", "POLITICAL", "政治敏感", "政治宣傳或爭議性動員"),
    FAKE_NEWS("FAKE_NEWS", "FAKE_NEWS", "不實資訊", "可查證為假的內容"),
    OTHER("OTHER", "OTHER", "其他", "不屬於上列任一類；前端可自行在標籤後附加「（請說明）」之類的表單提示，該提示是呈現層文案不是碼表的一部分");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ReportReasonCode(String code, String value, String label, String description) {
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

    private static final Map<String, ReportReasonCode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportReasonCode::code, e -> e));

    private static final Map<String, ReportReasonCode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReportReasonCode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ReportReasonCode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ReportReasonCode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
