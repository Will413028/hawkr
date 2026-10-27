package io.github.will413028.hawkr.payment.config;

import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipBillingRepository;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderAccountProfile;
import io.github.will413028.hawkr.payment.provider.payuni.PayUniRecurringWebhookNormalizer;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * PayUni 金流 SDK 配置（payment-service 專用）
 *
 * 統一使用 shared/payuni-java-client 提供的 PaymentClient，
 * 不再重複實作 client / cryptoService。
 *
 * 對應審計報告 §4：移除 payment-service 內 660 行的 PayUniClient
 * 與 38 行薄包裝 PayUniCryptoService，全面切到 shared SDK。
 */
@Configuration
public class PayUniConfig {

    @Value("${payuni.agent.key}")
    private String agentKey;

    @Value("${payuni.agent.iv}")
    private String agentIv;

    @Value("${payuni.api.base-url}")
    private String baseUrl;

    @Value("${payuni.callback.notify-url:}")
    private String notifyUrl;

    @Value("${payuni.callback.return-url:}")
    private String returnUrl;

    @Value("${payuni.agent.id:HAWK}")
    private String agentId;

    @Bean
    public PayUniCryptoService payUniCryptoService() {
        return new PayUniCryptoService(agentKey, agentIv);
    }

    @Bean
    public PayUniHttpClient payUniHttpClient(PayUniCryptoService cryptoService, PayUniAuditLogger auditLogger) {
        PayUniHttpClient client = new PayUniHttpClient(cryptoService, baseUrl);
        // 05 §11 audit logging：注入 onExchange logger 將每次 PayUni exchange 寫入 payuni_audit_logs 專表
        client.setRequestLogger(auditLogger);
        return client;
    }

    /**
     * PayUni 金流 API 客戶端 — payment-service 唯一對 PayUni 通訊入口
     */
    @Bean
    public PaymentClient paymentClient(PayUniHttpClient httpClient) {
        return new PaymentClient(httpClient, baseUrl, notifyUrl, returnUrl);
    }

    /**
     * Recurring webhook normalization is provider-specific. MerID is resolved through the
     * payment-owned provider-account allowlist; a payload value is never accepted as an account
     * alias merely because it is non-blank. Unknown, disabled, or non-PayUni profiles fail closed.
     */
    @Bean
    public PayUniRecurringWebhookNormalizer payUniRecurringWebhookNormalizer(
            MembershipBillingRepository billingRepository
    ) {
        return new PayUniRecurringWebhookNormalizer(merId -> billingRepository.findProviderAccount(merId)
                .filter(ProviderAccountProfile::isActive)
                .filter(profile -> "PAYUNI".equals(profile.provider()))
                .map(ProviderAccountProfile::accountRef)
                .orElse(""),
                PayUniRecurringWebhookNormalizer.DEFAULT_CURRENCY);
    }

    public String getAgentId() {
        return agentId;
    }

    public String getBaseUrl() {
        return baseUrl;
    }
}
