package io.github.will413028.hawkr.media.exception;

import io.grpc.Status;
import lombok.Getter;

/**
 * Media Service 業務例外
 */
@Getter
public class MediaServiceException extends RuntimeException {

    /**
     * gRPC 狀態碼
     */
    private final Status.Code grpcStatusCode;

    /**
     * HTTP 狀態碼
     */
    private final int httpStatusCode;

    public MediaServiceException(String message, Status.Code grpcStatusCode, int httpStatusCode) {
        super(message);
        this.grpcStatusCode = grpcStatusCode;
        this.httpStatusCode = httpStatusCode;
    }

    public MediaServiceException(String message, Throwable cause, Status.Code grpcStatusCode, int httpStatusCode) {
        super(message, cause);
        this.grpcStatusCode = grpcStatusCode;
        this.httpStatusCode = httpStatusCode;
    }

    /**
     * INVALID_ARGUMENT (400)
     */
    public static MediaServiceException invalidArgument(String message) {
        return new MediaServiceException(message, Status.Code.INVALID_ARGUMENT, 400);
    }

    /**
     * NOT_FOUND (404)
     */
    public static MediaServiceException notFound(String message) {
        return new MediaServiceException(message, Status.Code.NOT_FOUND, 404);
    }

    /**
     * PERMISSION_DENIED (403)
     */
    public static MediaServiceException permissionDenied(String message) {
        return new MediaServiceException(message, Status.Code.PERMISSION_DENIED, 403);
    }

    /**
     * FAILED_PRECONDITION (409)
     */
    public static MediaServiceException failedPrecondition(String message) {
        return new MediaServiceException(message, Status.Code.FAILED_PRECONDITION, 409);
    }

    /**
     * INTERNAL (500)
     */
    public static MediaServiceException internal(String message) {
        return new MediaServiceException(message, Status.Code.INTERNAL, 500);
    }

    /**
     * INTERNAL (500) with cause
     */
    public static MediaServiceException internal(String message, Throwable cause) {
        return new MediaServiceException(message, cause, Status.Code.INTERNAL, 500);
    }
}
