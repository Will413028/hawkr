package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Shared billing plane 與 domain owner 之間的 machine-readable error symbol。這些值不是對外 HTTP 數字碼；若需前端可見業務碼，必須另在 `錯誤碼.md` 分配數字並同步 Gateway。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum MembershipBillingErrorCode implements CodeMeta {

    BILLING_SUBJECT_INVALID("BILLING_SUBJECT_INVALID", "BILLING_SUBJECT_INVALID", "Billing subject 無效", "subject type／ID 不符合 domain contract"),
    BILLING_OFFER_INVALID("BILLING_OFFER_INVALID", "BILLING_OFFER_INVALID", "Billing offer 無效", "offer 不存在、已封存或 snapshot 不一致"),
    BILLING_QUOTE_INVALID("BILLING_QUOTE_INVALID", "BILLING_QUOTE_INVALID", "Billing quote 無效", "quote 過期、hash／subject／membership 綁定不一致或 caller 不合法"),
    BILLING_AMOUNT_MISMATCH("BILLING_AMOUNT_MISMATCH", "BILLING_AMOUNT_MISMATCH", "Billing 金額不一致", "provider 回報金額與 immutable offer snapshot 不一致"),
    BILLING_COMMAND_CONFLICT("BILLING_COMMAND_CONFLICT", "BILLING_COMMAND_CONFLICT", "Billing command 衝突", "相同冪等鍵搭配不同 request hash"),
    BILLING_SUBSCRIPTION_CONFLICT("BILLING_SUBSCRIPTION_CONFLICT", "BILLING_SUBSCRIPTION_CONFLICT", "Billing subscription 衝突", "同 subject／kind 已有 active subscription 或 provider identity 衝突"),
    PROVIDER_CAPABILITY_UNSUPPORTED("PROVIDER_CAPABILITY_UNSUPPORTED", "PROVIDER_CAPABILITY_UNSUPPORTED", "Provider 能力不支援", "provider 不支援要求的 lifecycle action"),
    PROVIDER_REJECTED("PROVIDER_REJECTED", "PROVIDER_REJECTED", "Provider 拒絕", "provider 明確拒絕請求，交由 domain policy 處理"),
    PROVIDER_RESULT_CONFLICT("PROVIDER_RESULT_CONFLICT", "PROVIDER_RESULT_CONFLICT", "Provider 結果衝突", "同一期已有相反 authoritative result，只留 conflict audit"),
    BILLING_UNAVAILABLE("BILLING_UNAVAILABLE", "BILLING_UNAVAILABLE", "Billing 暫時不可用", "billing plane 暫時不可用，可重送但不授予 entitlement");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    MembershipBillingErrorCode(String code, String value, String label, String description) {
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

    private static final Map<String, MembershipBillingErrorCode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(MembershipBillingErrorCode::code, e -> e));

    private static final Map<String, MembershipBillingErrorCode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(MembershipBillingErrorCode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static MembershipBillingErrorCode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static MembershipBillingErrorCode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
