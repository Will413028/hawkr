package io.github.will413028.hawkr.common.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.grpc.StatusRuntimeException;

/**
 * gRPC 服務共用輔助工具。
 * <p>
 * 提供錯誤訊息提取、狀態碼映射等跨服務通用方法，
 * 減少各微服務 gRPC Service 的重複實作。
 * </p>
 */
public final class GrpcServiceHelper {

    private GrpcServiceHelper() {
    }

    /**
     * 安全提取異常訊息。
     * null/空訊息時回傳類別名稱，避免前端收到無用的空字串。
     */
    public static String getErrorMessage(Exception e) {
        if (e == null) {
            return "未知錯誤";
        }
        String message = e.getMessage();
        if (message == null || message.isEmpty()) {
            return e.getClass().getSimpleName();
        }
        return message;
    }

    /**
     * 組裝系統錯誤訊息（含異常詳情）。
     * 格式：「系統錯誤: {異常訊息}」，為 gRPC 系統錯誤訊息的統一格式。
     */
    public static String formatSystemError(Exception e) {
        return "系統錯誤: " + getErrorMessage(e);
    }

    /**
     * StatusRuntimeException → GrpcResponseCode 映射。
     * 將 gRPC 框架層的狀態碼轉為業務回應碼。
     */
    public static int mapStatusCode(StatusRuntimeException e) {
        return switch (e.getStatus().getCode()) {
            case INVALID_ARGUMENT -> GrpcResponseCode.INVALID_PARAM.code();
            case UNAUTHENTICATED -> GrpcResponseCode.UNAUTHORIZED.code();
            case PERMISSION_DENIED -> GrpcResponseCode.PERMISSION_DENIED.code();
            case NOT_FOUND -> GrpcResponseCode.NOT_FOUND.code();
            case ALREADY_EXISTS -> GrpcResponseCode.CONFLICT.code();
            case UNAVAILABLE -> GrpcResponseCode.SERVICE_UNAVAILABLE.code();
            default -> GrpcResponseCode.INTERNAL_ERROR.code();
        };
    }

    /**
     * 取得 StatusRuntimeException 的描述訊息。
     * null 時回傳通用錯誤訊息。
     */
    public static String getStatusDescription(StatusRuntimeException e) {
        String desc = e.getStatus().getDescription();
        return desc != null ? desc : "系統錯誤";
    }

    /**
     * 判斷是否為 gRPC 狀態異常（通常來自下游服務呼叫）。
     */
    public static boolean isStatusRuntimeException(Exception e) {
        return e instanceof StatusRuntimeException;
    }
}
