package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.AccountType;

/**
 * §8.5 AccountType 業務規則 helper（跨 common-core / media / payment 服務共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 將 AccountType 內原有的 {@code covers()} 權限涵蓋判斷與
 * {@code fromValue/fromCode} 智慧解析移到此處。
 * <p>
 * GrpcAuthContext 也在 common-core，故 helper 必須放在 common-core
 * `utils/code/helper/` 子套件而非單一服務內。helper 為純函式，不引入任何外部依賴。
 */
public final class AccountTypeRules {

    private AccountTypeRules() {
    }

    /**
     * 檢查此帳號類型是否涵蓋指定的角色權限。
     * <p>
     * SELLER_AND_BUYER 同時涵蓋 BUYER 與 SELLER，其餘僅匹配自身。
     */
    public static boolean covers(AccountType actual, AccountType required) {
        if (actual == required) {
            return true;
        }
        if (actual == AccountType.SELLER_AND_BUYER) {
            return required == AccountType.BUYER || required == AccountType.SELLER;
        }
        return false;
    }

    /**
     * 智慧解析 AccountType 字串，支援多種格式：
     * <ul>
     *   <li>value：{@code "1","2","3","4"}</li>
     *   <li>code：{@code "BUYER","SELLER","SELLER_AND_BUYER","ADMIN"}</li>
     *   <li>小寫：{@code "buyer","seller","seller_and_buyer","admin"}</li>
     * </ul>
     */
    public static AccountType parse(String raw) {
        if (raw == null || raw.isBlank()) {
            throw new IllegalArgumentException("account-type 不可為空");
        }
        String trimmed = raw.trim();

        // 優先 value 查找（"1","2","3","4"）
        if (AccountType.isValidValue(trimmed)) {
            return AccountType.fromValue(trimmed);
        }
        // code 查找（"BUYER","SELLER"）
        if (AccountType.isValidCode(trimmed)) {
            return AccountType.fromCode(trimmed);
        }
        // 小寫名稱查找
        String lower = trimmed.toLowerCase();
        for (AccountType type : AccountType.values()) {
            if (type.code().toLowerCase().equals(lower)) {
                return type;
            }
        }
        throw new IllegalArgumentException("無法識別的 account-type: " + raw);
    }
}
