package io.github.will413028.hawkr.common.cache;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.PropertySource;

import java.net.http.HttpClient;
import java.time.Duration;

/**
 * Frontend cache invalidation 共用自動設定（後端服務責任邊界規範 §9）。
 *
 * <p>提供 {@link WebRevalidateProperties} + {@link RevalidateClient} bean。微服務只需
 * 注入 {@code RevalidateClient} 即可在 mutation tx 完成後呼叫
 * {@link RevalidateClient#bustAfterCommit}。
 *
 * <p>未自行提供 {@code HttpClient} bean 時，由本設定建立 {@link #revalidateHttpClient()}。
 */
@AutoConfiguration
@EnableConfigurationProperties(WebRevalidateProperties.class)
@PropertySource("classpath:common-core-defaults.properties")
public class RevalidateAutoConfiguration {

    private static final Logger log = LoggerFactory.getLogger(RevalidateAutoConfiguration.class);

    @Bean
    @ConditionalOnMissingBean(HttpClient.class)
    public HttpClient revalidateHttpClient() {
        // 強制 HTTP/1.1：目標是 cleartext http:// 的 Next.js /api/revalidate，該 Node server
        // 不支援 HTTP/2 cleartext，收到 h2 即重置而靜默失敗；Java 預設 HTTP_2，必須顯式降級。
        return HttpClient.newBuilder()
            .version(HttpClient.Version.HTTP_1_1)
            .followRedirects(HttpClient.Redirect.NEVER)
            .connectTimeout(Duration.ofSeconds(5))
            .build();
    }

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
