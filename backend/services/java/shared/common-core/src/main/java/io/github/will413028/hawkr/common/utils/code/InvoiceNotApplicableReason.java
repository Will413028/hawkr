package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 平台不為這張訂單開立發票的原因。每個值對應履約模式決策樹的一個 NOT_APPLICABLE 葉節點，**不多不少**（多一個＝死碼，少一個＝該分支寫不進 DB）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceNotApplicableReason implements CodeMeta {

    ZERO_AMOUNT("ZERO_AMOUNT", "ZERO_AMOUNT", "訂單金額為零", "全額折抵（PaymentMethod ZERO_AMOUNT 2900）；仍照發 order.paid，但不開票"),
    SELF_ISSUE("SELF_ISSUE", "SELF_ISSUE", "賣家自行開立", "賣家已具結待審（SELF_ISSUE_PENDING）；賣家回報號碼後轉 RESOLVED_EXTERNALLY"),
    TAX_PROFILE_NOT_APPROVED("TAX_PROFILE_NOT_APPROVED", "TAX_PROFILE_NOT_APPROVED", "賣家稅務資格未核准", "review_status 落在 DRAFT／SUBMITTED／UNDER_REVIEW／SUPPLEMENT_REQUIRED／REJECTED／EXPIRED／REVOKED"),
    PROVIDER_NOT_ACTIVE("PROVIDER_NOT_ACTIVE", "PROVIDER_NOT_ACTIVE", "服務商連線未啟用", "provider_account.status 不在 {ACTIVE, DEGRADED}；含 ADM 停用與金鑰作廢"),
    CUTOVER_NOT_REACHED("CUTOVER_NOT_REACHED", "CUTOVER_NOT_REACHED", "自動化切點前的訂單", "automation_cutover_at 未設定，或發票日期早於切點");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceNotApplicableReason(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceNotApplicableReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceNotApplicableReason::code, e -> e));

    private static final Map<String, InvoiceNotApplicableReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceNotApplicableReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceNotApplicableReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceNotApplicableReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
