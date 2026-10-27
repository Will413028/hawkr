package io.github.will413028.hawkr.common.safefetch;

import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.net.http.HttpClient;
import java.time.Duration;

/**
 * SafeFetcher Spring Boot AutoConfiguration。任何依賴 common-core 的微服務可直接
 * {@code @Autowired SafeFetcher safeFetcher;}。
 *
 * <p>HttpClient policy=NEVER：自行處理 redirect 以做每跳 SSRF 重驗（守則 5）。</p>
 *
 * <p>詳見 40 號設計書 §10.2、08 號設計書 §7.7。</p>
 */
@Configuration
public class SafeFetchAutoConfiguration {

    @Bean
    @ConditionalOnMissingBean(HttpClient.class)
    public HttpClient safeFetchHttpClient() {
        return HttpClient.newBuilder()
            // 強制 HTTP/1.1：本 client 共用於 RevalidateClient（打 cleartext http:// 的 Next.js
            // /api/revalidate，該 Node server 不支援 HTTP/2 cleartext，收到 h2 即重置 →
            // "header parser received no bytes" 靜默失敗）與 SafeFetcher（抓外部來源，亦可能遇
            // cleartext http:// 的 HTTP/1.1 server）。Java 預設 HTTP_2，必須顯式降為 HTTP_1_1。
            // 對 https:// 目標僅失去 HTTP/2 多路複用（單次 fetch 無實質效益），有意接受。
            .version(HttpClient.Version.HTTP_1_1)
            .followRedirects(HttpClient.Redirect.NEVER)
            .connectTimeout(Duration.ofSeconds(5))
            .build();
    }

    @Bean
    @ConditionalOnMissingBean(SafeFetcher.class)
    public SafeFetcher safeFetcher(HttpClient safeFetchHttpClient) {
        return new SafeFetcherImpl(safeFetchHttpClient);
    }
}
