package io.github.will413028.hawkr.common.safefetch;

/**
 * SafeFetcher 回應結果。詳見 40 號設計書 §10.2。
 *
 * @param content 下載到的 binary 內容
 * @param contentType 實際 Content-Type（response header）
 * @param actualSizeBytes 實際下載 byte 數
 * @param finalUrl 跟完 redirect 後的最終 URL
 */
public record SafeFetchResponse(
    byte[] content,
    String contentType,
    long actualSizeBytes,
    String finalUrl
) {
}
