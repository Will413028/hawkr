package io.github.will413028.hawkr.common.cache;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.AutoConfigureAfter;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.PropertySource;

import java.net.http.HttpClient;

/**
 * Frontend cache invalidation 共用自動設定（後端服務責任邊界規範 §9）。
 *
 * <p>提供 {@link WebRevalidateProperties} + {@link RevalidateClient} bean。微服務只需
 * 注入 {@code RevalidateClient} 即可在 mutation tx 完成後呼叫
 * {@link RevalidateClient#bustAfterCommit}。
 *
 * <p>依賴 {@code HttpClient} bean（由 common-core SafeFetchAutoConfiguration 提供），
 * 故 {@code @AutoConfigureAfter} 確保 SafeFetch 先註冊。
 */
@AutoConfiguration
@AutoConfigureAfter(io.github.will413028.hawkr.common.safefetch.SafeFetchAutoConfiguration.class)
@EnableConfigurationProperties(WebRevalidateProperties.class)
@PropertySource("classpath:common-core-defaults.properties")
public class RevalidateAutoConfiguration {

    private static final Logger log = LoggerFactory.getLogger(RevalidateAutoConfiguration.class);

    @Bean
    @ConditionalOnMissingBean(RevalidateClient.class)
    public RevalidateClient revalidateClient(WebRevalidateProperties properties, HttpClient httpClient) {
        // 部署漏設 WEB_REVALIDATE_* 時 client 靜默 no-op → 前端快取永不失效卻無訊號。
        // 啟動時發一次 WARN（不 fail startup，本地無此 config 仍可開發）。§9.5
        if (!properties.isEnabled()) {
            log.warn("RevalidateClient.disabled hawkr.web-revalidate.base-url / secret 未設定，前端快取將不失效");
        }
        return new RevalidateClient(properties, httpClient);
    }
}
