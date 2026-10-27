package io.github.will413028.hawkr.common.safefetch;

/**
 * SafeFetcher 抛出的 SSRF / 違反守則異常。Reason 用於 caller 對應 ImportAssetFailureReason 等業務碼。
 * 詳見 40 號設計書 §10.2。
 */
public class SafeFetchException extends RuntimeException {

    public enum Reason {
        PROTOCOL_NOT_ALLOWED,
        IP_BLOCKED_PRIVATE,
        IP_BLOCKED_LOOPBACK,
        IP_BLOCKED_LINK_LOCAL,
        IP_BLOCKED_MULTICAST,
        DOMAIN_BLOCKED_INTERNAL,
        REDIRECT_LIMIT_EXCEEDED,
        REDIRECT_CROSS_DOMAIN,
        SIZE_EXCEEDED,
        TIMEOUT_CONNECT,
        TIMEOUT_READ,
        TLS_VERIFY_FAILED,
        CONTENT_TYPE_NOT_ALLOWED,
        NOT_FOUND,
        FETCH_ERROR
    }

    private final Reason reason;

    public SafeFetchException(Reason reason, String message) {
        super(message);
        this.reason = reason;
    }

    public SafeFetchException(Reason reason, String message, Throwable cause) {
        super(message, cause);
        this.reason = reason;
    }

    public Reason reason() {
        return reason;
    }
}
