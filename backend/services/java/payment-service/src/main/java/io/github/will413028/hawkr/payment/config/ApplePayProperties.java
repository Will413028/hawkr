package io.github.will413028.hawkr.payment.config;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import jakarta.annotation.PostConstruct;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

/**
 * Apple Pay on Web Merchant Validation 配置
 *
 * 設計參考：docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md §5.11
 */
@Component
@ConfigurationProperties(prefix = "hawkr.payment.apple-pay")
public class ApplePayProperties {

    private static final ObsLogger log = ObsLogger.forClass(ApplePayProperties.class);

    /** Apple Developer 後台註冊的 Merchant ID。POST Apple 時作為 merchantIdentifier。 */
    private String merchantId;

    /** Apple Pay sheet 顯示的商家名稱。POST Apple 時作為 displayName。 */
    private String displayName;

    /** Apple 簽發的 X.509 Merchant Identity Certificate PEM 絕對路徑（`BEGIN CERTIFICATE`）。 */
    private String merchantCertPath;

    /** 對應 Merchant Identity 的 PKCS#8 Private Key PEM 絕對路徑（`BEGIN PRIVATE KEY`）。 */
    private String merchantKeyPath;

    /** 允許發起付款的網域清單（YAML list 或逗號分隔），驗證 ValidateApplePayMerchantRequest.domain_name 入參。 */
    private List<String> domainAllowlist = List.of();

    /** Apple gateway connect timeout（秒） */
    private int connectTimeoutSeconds = 5;

    /** Apple gateway request timeout（秒） */
    private int requestTimeoutSeconds = 10;

    public String getMerchantId() { return merchantId; }
    public void setMerchantId(String merchantId) { this.merchantId = merchantId; }

    public String getDisplayName() { return displayName; }
    public void setDisplayName(String displayName) { this.displayName = displayName; }

    public String getMerchantCertPath() { return merchantCertPath; }
    public void setMerchantCertPath(String merchantCertPath) { this.merchantCertPath = merchantCertPath; }

    public String getMerchantKeyPath() { return merchantKeyPath; }
    public void setMerchantKeyPath(String merchantKeyPath) { this.merchantKeyPath = merchantKeyPath; }

    public List<String> getDomainAllowlist() { return domainAllowlist; }
    public void setDomainAllowlist(List<String> domainAllowlist) { this.domainAllowlist = domainAllowlist; }

    public int getConnectTimeoutSeconds() { return connectTimeoutSeconds; }
    public void setConnectTimeoutSeconds(int connectTimeoutSeconds) { this.connectTimeoutSeconds = connectTimeoutSeconds; }

    public int getRequestTimeoutSeconds() { return requestTimeoutSeconds; }
    public void setRequestTimeoutSeconds(int requestTimeoutSeconds) { this.requestTimeoutSeconds = requestTimeoutSeconds; }

    /**
     * 啟動時檢查 Apple Pay 配置完整性，缺漏欄位印 WARNING（不 fail-fast）。
     *
     * 設計選擇：採 lazy load 而非 @Validated+@NotBlank — 沒打算用 Apple Pay 的開發者
     * 本地仍能正常啟動 payment-service 跑信用卡 / ATM 流程；正式環境若配置漏，
     * 此 warn-log 讓 ops 在啟動時立刻看到問題，毋需等使用者觸發 8029 才察覺。
     *
     * 詳見 05 號設計書 §5.11.6 / reviewer §12 取捨記錄。
     */
    @PostConstruct
    void warnIfIncomplete() {
        List<String> missing = new ArrayList<>();
        if (isBlank(merchantId)) missing.add("merchantId");
        if (isBlank(displayName)) missing.add("displayName");
        if (isBlank(merchantCertPath)) missing.add("merchantCertPath");
        if (isBlank(merchantKeyPath)) missing.add("merchantKeyPath");
        if (domainAllowlist == null || domainAllowlist.isEmpty()) missing.add("domainAllowlist");

        if (missing.isEmpty()) {
            log.info("ready", "Apple Pay properties loaded: merchantId=" + merchantId
                    + " domains=" + domainAllowlist);
        } else {
            log.warn("incomplete", "Apple Pay properties incomplete - validate_merchant endpoint "
                    + "will return APPLE_PAY_CERT_LOAD_FAILED (8029) until fixed. Missing: " + missing);
        }
    }

    private static boolean isBlank(String s) {
        return s == null || s.isBlank();
    }
}
