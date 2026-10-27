package io.github.will413028.hawkr.common.cache;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletionException;

/**
 * Frontend Next.js SSR cache invalidation 共用 client（後端服務責任邊界規範 §9）。
 *
 * <p>fire-and-forget 語意：mutation tx commit 之後 inline call，5 秒 timeout，
 * 失敗只走 {@link ObsLogger}，**不 throw**、不阻擋業務 response。cacheLife
 * profile 是 safety net（hawkr-website {@code next.config.ts}）。
 *
 * <p>Tag 命名（SSOT 在 frontend {@code lib/seo/tags.ts}）：
 * <ul>
 *     <li>{@code product:<id>} — PDP detail / product reviews list</li>
 *     <li>{@code seller:<id>} — seller profile / 商店列表 / 評論統計</li>
 * </ul>
 */
public class RevalidateClient {

    private static final Logger log = LoggerFactory.getLogger(RevalidateClient.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final WebRevalidateProperties properties;
    private final HttpClient httpClient;

    public RevalidateClient(WebRevalidateProperties properties, HttpClient httpClient) {
        this.properties = properties;
        this.httpClient = httpClient;
    }

    /**
     * tx commit 後再觸發 cache bust（caller 在 {@code @Transactional} 內呼叫的推薦入口）。
     *
     * <p>tx 已 active → afterCommit hook 觸發 {@link #bustAsync}；tx 不 active（或同事務
     * rollback）→ 立刻 {@link #bustAsync}（避免 rollback 後仍誤觸 bust）。
     */
    public void bustAfterCommit(List<String> tags) {
        if (tags == null || tags.isEmpty()) return;
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    bustAsync(tags);
                }
            });
        } else {
            bustAsync(tags);
        }
    }

    /**
     * 連線層 IOException 冪等重試上限（§9.5）：初次 + 最多 2 次重試 = 最多 3 次送出。
     * 只對「stale pooled connection」型連線失敗（fast-fail）重試，取 fresh connection；
     * timeout / non-2xx 不重試（見 {@link #shouldRetry}）。
     */
    private static final int MAX_ATTEMPTS = 3;

    /**
     * fire-and-forget 觸發 frontend cache bust。
     *
     * @param tags cache tags（如 {@code List.of("product:123", "seller:456")}）
     */
    public void bustAsync(List<String> tags) {
        if (tags == null || tags.isEmpty()) return;
        if (!properties.isEnabled()) {
            // 配置未填（如 local dev 不對接 web）→ noop 不報錯，避免本地誤觸警告
            return;
        }
        HttpRequest request;
        try {
            Map<String, Object> body = new LinkedHashMap<>();
            body.put("tags", tags);
            String json = MAPPER.writeValueAsString(body);
            request = HttpRequest.newBuilder()
                    .uri(URI.create(properties.getBaseUrl() + "/api/revalidate"))
                    .timeout(Duration.ofSeconds(properties.getTimeoutSeconds()))
                    .header("Content-Type", "application/json")
                    .header("x-revalidate-secret", properties.getSecret())
                    .POST(HttpRequest.BodyPublishers.ofString(json))
                    .build();
        } catch (Exception e) {
            log.warn("RevalidateClient.build.fail buildRequest failed: {}", e.getMessage());
            return;
        }
        sendWithRetry(tags, request, 1);
    }

    /**
     * 送出並在連線層 IOException（stale pooled connection）時重試取 fresh connection（§9.5，#1271）。
     * {@code revalidateTag} 冪等，重試安全。
     */
    private void sendWithRetry(List<String> tags, HttpRequest request, int attempt) {
        httpClient.sendAsync(request, HttpResponse.BodyHandlers.ofString())
                .whenComplete((resp, err) -> {
                    if (err != null) {
                        if (shouldRetry(err) && attempt < MAX_ATTEMPTS) {
                            // stale pooled connection：連線已從池移除，下次送出即開新連線
                            log.warn("RevalidateClient.send.retry tags={} attempt={} reason={}",
                                    tags, attempt, rootMessage(err));
                            sendWithRetry(tags, request, attempt + 1);
                        } else {
                            log.warn("RevalidateClient.send.error tags={} attempt={} reason={}",
                                    tags, attempt, rootMessage(err));
                        }
                        return;
                    }
                    if (resp.statusCode() >= 200 && resp.statusCode() < 300) {
                        log.info("RevalidateClient.send.success tags={} status={} attempt={}",
                                tags, resp.statusCode(), attempt);
                    } else {
                        // non-2xx（如 401 secret 不符）屬設定問題，不重試
                        log.warn("RevalidateClient.send.non2xx tags={} status={} body={}",
                                tags, resp.statusCode(), truncate(resp.body()));
                    }
                });
    }

    /**
     * 僅對「連線層 IOException」重試（如 stale pooled connection 造成的
     * {@code header parser received no bytes}）。timeout（{@link HttpTimeoutException}，
     * 含 connect timeout）不重試 —— 那是已延長的窗口，重試只拖長 ack 不確定時間。
     */
    private static boolean shouldRetry(Throwable err) {
        Throwable cause = err;
        while (cause instanceof CompletionException && cause.getCause() != null) {
            cause = cause.getCause();
        }
        return cause instanceof IOException && !(cause instanceof HttpTimeoutException);
    }

    private static String rootMessage(Throwable err) {
        Throwable cause = err;
        while (cause.getCause() != null && cause.getCause() != cause) {
            cause = cause.getCause();
        }
        return cause.getClass().getSimpleName() + ": " + cause.getMessage();
    }

    private static String truncate(String s) {
        if (s == null) return "";
        return s.length() > 200 ? s.substring(0, 200) + "..." : s;
    }
}
