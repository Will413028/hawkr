package io.github.will413028.hawkr.payuni.http;

/**
 * PayUni API 呼叫例外
 */
public class PayUniApiException extends RuntimeException {

    private final String errorCode;
    private final int httpStatus;

    public PayUniApiException(String errorCode, String message, int httpStatus) {
        super(message);
        this.errorCode = errorCode;
        this.httpStatus = httpStatus;
    }

    public PayUniApiException(String message, Throwable cause) {
        super(message, cause);
        this.errorCode = "IO_ERROR";
        this.httpStatus = 0;
    }

    public String getErrorCode() {
        return errorCode;
    }

    public int getHttpStatus() {
        return httpStatus;
    }
}
