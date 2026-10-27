package io.github.will413028.hawkr.shipping.dto;

/**
 * Provider 狀態批次查詢結果行（收斂規範 §6.3）。
 * Why: 直接用 Map&lt;String, ?&gt; 作為 MyBatis result type 會觸發全域 JsonbMap TypeHandler，
 *      把欄位值當 JSON 解析而崩潰。獨立 DTO 避開此問題。
 */
public class ProviderStatusRow {
    private String key;
    private String providerStatus;
    private String providerMessage;

    public String getKey() { return key; }
    public void setKey(String key) { this.key = key; }

    public String getProviderStatus() { return providerStatus; }
    public void setProviderStatus(String providerStatus) { this.providerStatus = providerStatus; }

    public String getProviderMessage() { return providerMessage; }
    public void setProviderMessage(String providerMessage) { this.providerMessage = providerMessage; }
}
