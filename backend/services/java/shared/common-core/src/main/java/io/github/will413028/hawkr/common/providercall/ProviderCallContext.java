package io.github.will413028.hawkr.common.providercall;

/**
 * 外部呼叫情境。記錄呼叫類型 + 對應資源 ID，用於日誌與例外回溯。
 *
 * <p>callKind 範例：{@code "printCvsLabel"} / {@code "callTcat"} / {@code "submitCvsToPayUni"}。
 *
 * <p>resourceId 為被操作的 DB 主鍵（如 shipment.id 或 setting.id）。
 *
 * <p>設計參見 06-Shipping §2.4.4。
 */
public record ProviderCallContext(String callKind, long resourceId) {

    public static ProviderCallContext of(String callKind, long resourceId) {
        return new ProviderCallContext(callKind, resourceId);
    }
}
