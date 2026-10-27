package io.github.will413028.hawkr.common.safefetch;

/**
 * SSRF-safe 外部 URL fetcher。詳見 40 號設計書 §10。
 *
 * <p>實作端必須完整執行 §10.1 的 12 條守則：協定白名單、DNS IP 黑名單、自家網域黑名單、
 * DNS rebinding 防護、redirect 限制、size 邊讀邊計、connect/read timeout、TLS 驗證、
 * User-Agent、同 IP rate limit、Content-Type 白名單、size 上限。</p>
 *
 * <p>違反任一守則時拋 {@link SafeFetchException}，caller 依 {@link SafeFetchException#reason()}
 * 對應業務碼（ImportAssetFailureReason 等）。</p>
 */
public interface SafeFetcher {

    /**
     * 對外部 URL 執行安全的 HTTP GET。
     *
     * @param request 請求參數
     * @return 下載結果
     * @throws SafeFetchException 違反任一 §10.1 守則時拋出
     */
    SafeFetchResponse fetch(SafeFetchRequest request);
}
