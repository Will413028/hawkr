package io.github.will413028.hawkr.webhook.config;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.model.MerchantCredential;
import io.github.will413028.hawkr.payuni.resolver.MerchantSecretResolver;
import io.github.will413028.hawkr.payuni.webhook.PayUniWebhookParser;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * PayUni SDK 配置
 *
 * 實作 MerchantSecretResolver：代理商模式下所有子商家使用同一組 Key/IV
 */
@Configuration
public class PayUniConfig {

    @Value("${payuni.agent.key}")
    private String agentKey;

    @Value("${payuni.agent.iv}")
    private String agentIv;

    /**
     * 代理商金鑰解析器 — 所有 merId 共用同一組代理商 Key/IV
     */
    @Bean
    public MerchantSecretResolver merchantSecretResolver() {
        return merId -> new MerchantCredential(agentKey, agentIv);
    }

    @Bean
    public PayUniCryptoService payUniCryptoService() {
        return new PayUniCryptoService(agentKey, agentIv);
    }

    @Bean
    public PayUniWebhookParser payUniWebhookParser() {
        return new PayUniWebhookParser(payUniCryptoService());
    }
}
