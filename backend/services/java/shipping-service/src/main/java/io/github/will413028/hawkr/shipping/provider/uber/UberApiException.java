package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * 一次 Uber 呼叫的失敗，攜帶 66 §4.5.4 的分類。
 *
 * <p>🔴 <b>攜帶 {@link UberFailureKind} 而不是只帶 HTTP status 的理由</b>：
 * 同一個 HTTP status 對應到完全相反的處置——400 可能是
 * {@code noncancelable_delivery}（留原狀態）也可能是其他錯誤（寫 {@code 990} ＋ 全額退款）；
 * 403 可能是 {@code customer_blocked}（標門市 ＋ 通知賣家）也可能是憑證問題（斷路器）。
 * 讓每個呼叫端各自去 parse {@code code} 字串 ＝ 那份 provider 知識有 N 份副本。
 */
public class UberApiException extends RuntimeException {

    private final UberFailureKind kind;
    private final int httpStatus;
    private final String uberCode;

    public UberApiException(UberFailureKind kind, int httpStatus, String uberCode, String message) {
        super(message);
        this.kind = kind;
        this.httpStatus = httpStatus;
        this.uberCode = uberCode;
    }

    public UberApiException(UberFailureKind kind, int httpStatus, String uberCode,
                            String message, Throwable cause) {
        super(message, cause);
        this.kind = kind;
        this.httpStatus = httpStatus;
        this.uberCode = uberCode;
    }

    public UberFailureKind kind() {
        return kind;
    }

    /** HTTP 狀態碼；{@code 0} 表示根本沒有收到回應（連線失敗／逾時）。 */
    public int httpStatus() {
        return httpStatus;
    }

    /** Uber 回應 body 裡的 {@code code} 欄位；沒有則為 null。 */
    public String uberCode() {
        return uberCode;
    }

    /**
     * 落 {@code shipments.provider_error_code} 的值。
     *
     * <p>⚠️ 優先用 Uber 的 {@code code}（那是對方的語彙，對帳時查得到），
     * 沒有才退到我方的分類名——兩者混在同一欄是刻意的：那一欄回答的是
     * 「provider 說了什麼」，而「對方連話都沒說」本身就是一種答案。
     */
    public String providerErrorCode() {
        return (uberCode != null && !uberCode.isBlank()) ? uberCode : kind.name();
    }
}
