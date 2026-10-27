package io.github.will413028.hawkr.common.safefetch;

import java.time.Duration;
import java.util.Set;

/**
 * SafeFetcher 請求參數。詳見 40 號設計書 §10.2。
 *
 * @param url 外部 URL（必須 https）
 * @param allowedContentTypes 允許的 Content-Type 白名單；空 = 不檢查
 * @param maxSizeBytes 最大允許下載 byte 數（超過立刻 abort）
 * @param connectTimeout 連線 timeout（建議 5s）
 * @param readTimeout 讀取 timeout（建議 30s）
 * @param userAgent User-Agent header
 * @param maxRedirectHops 最多 redirect 跳數（建議 3）
 */
public record SafeFetchRequest(
    String url,
    Set<String> allowedContentTypes,
    long maxSizeBytes,
    Duration connectTimeout,
    Duration readTimeout,
    String userAgent,
    int maxRedirectHops
) {
}
