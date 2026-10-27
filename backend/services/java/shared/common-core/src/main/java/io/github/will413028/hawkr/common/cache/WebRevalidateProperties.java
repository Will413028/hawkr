package io.github.will413028.hawkr.common.cache;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * hawkr-website cache invalidation 共用配置（後端服務責任邊界規範 §9.7）。
 *
 * <p>backend mutation 完成後呼叫 {@code POST {baseUrl}/api/revalidate} 並帶
 * {@code x-revalidate-secret} header，觸發 frontend Next.js {@code revalidateTag}
 * 即時 bust SSR cache。
 *
 * <p>由 {@link RevalidateAutoConfiguration} 自動載入；微服務無需手動匯入。
 */
@ConfigurationProperties(prefix = "hawkr.web-revalidate")
public class WebRevalidateProperties {

    /** hawkr-website base URL（dev=https://web-dev.example.com, prod=https://example.com）。 */
    private String baseUrl = "";

    /** 與 hawkr-website .env REVALIDATE_SECRET 同值。空字串 → 不送 request（無設則 noop，不報錯）。 */
    private String secret = "";

    /** HTTP request timeout（秒）。 */
    private int timeoutSeconds = 5;

    public String getBaseUrl() {
        return baseUrl;
    }

    public void setBaseUrl(String baseUrl) {
        this.baseUrl = baseUrl;
    }

    public String getSecret() {
        return secret;
    }

    public void setSecret(String secret) {
        this.secret = secret;
    }

    public int getTimeoutSeconds() {
        return timeoutSeconds;
    }

    public void setTimeoutSeconds(int timeoutSeconds) {
        this.timeoutSeconds = timeoutSeconds;
    }

    /** 配置完整、可實際送 request 才回 true（base + secret 非空）。 */
    public boolean isEnabled() {
        return baseUrl != null && !baseUrl.isBlank()
                && secret != null && !secret.isBlank();
    }
}
