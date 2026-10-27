package io.github.will413028.hawkr.common.utils.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.grpc.Status;

/**
 * 將 gRPC Status.Code 轉換為內部 GrpcResponseCode。
 *
 * 對應規則參考 gRPC 官方規範：https://grpc.io/docs/guides/error/
 *
 * 設計與規範請見 docs/開發規範/Java gRPC 錯誤處理規範.md §7.6.2
 */
public final class GrpcStatusToResponseCode {

    private GrpcStatusToResponseCode() {}

    public static GrpcResponseCode map(Status.Code grpcCode) {
        return switch (grpcCode) {
            case OK -> GrpcResponseCode.SUCCESS;
            case INVALID_ARGUMENT, OUT_OF_RANGE -> GrpcResponseCode.INVALID_PARAM;
            case NOT_FOUND -> GrpcResponseCode.NOT_FOUND;
            case ALREADY_EXISTS, FAILED_PRECONDITION, ABORTED -> GrpcResponseCode.CONFLICT;
            case UNAUTHENTICATED -> GrpcResponseCode.UNAUTHORIZED;
            case PERMISSION_DENIED -> GrpcResponseCode.PERMISSION_DENIED;
            case UNAVAILABLE, DEADLINE_EXCEEDED -> GrpcResponseCode.SERVICE_UNAVAILABLE;
            // INTERNAL, UNKNOWN, DATA_LOSS, RESOURCE_EXHAUSTED, UNIMPLEMENTED, CANCELLED
            default -> GrpcResponseCode.INTERNAL_ERROR;
        };
    }
}
