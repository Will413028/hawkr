package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import net.devh.boot.grpc.server.advice.GrpcAdvice;
import net.devh.boot.grpc.server.advice.GrpcExceptionHandler;

/**
 * gRPC 全局異常處理器
 *
 * 捕獲所有未處理的異常，記錄詳細日誌並返回適當的 gRPC 錯誤碼
 */
@GrpcAdvice
public class GrpcExceptionAdvice {

    private static final ObsLogger log = ObsLogger.forClass(GrpcExceptionAdvice.class);

    /** Preserve the shared machine-readable billing symbol in google.rpc.ErrorInfo details. */
    @GrpcExceptionHandler(BillingValidationException.class)
    public StatusRuntimeException handleBillingValidationException(BillingValidationException e) {
        log.warn("GrpcExceptionAdvice.billingValidation", e.getMessage());
        return MembershipBillingGrpcError.toStatusRuntimeException(e);
    }

    /**
     * 處理 IllegalArgumentException
     */
    @GrpcExceptionHandler(IllegalArgumentException.class)
    public Status handleIllegalArgumentException(IllegalArgumentException e) {
        log.error("GrpcExceptionAdvice.illegalArgument", "參數錯誤: " + e.getMessage(), e);
        return Status.INVALID_ARGUMENT.withDescription(e.getMessage()).withCause(e);
    }

    /**
     * 處理 IllegalStateException
     */
    @GrpcExceptionHandler(IllegalStateException.class)
    public Status handleIllegalStateException(IllegalStateException e) {
        log.error("GrpcExceptionAdvice.illegalState", "狀態錯誤: " + e.getMessage(), e);
        return Status.FAILED_PRECONDITION.withDescription(e.getMessage()).withCause(e);
    }

    /**
     * 處理 RuntimeException
     */
    @GrpcExceptionHandler(RuntimeException.class)
    public Status handleRuntimeException(RuntimeException e) {
        log.error("GrpcExceptionAdvice.runtimeException", "運行時錯誤: " + e.getMessage(), e);
        return Status.INTERNAL.withDescription(e.getMessage()).withCause(e);
    }

    /**
     * 處理所有其他異常
     */
    @GrpcExceptionHandler(Exception.class)
    public Status handleException(Exception e) {
        log.error("GrpcExceptionAdvice.exception", "未預期的錯誤: " + e.getMessage(), e);
        return Status.UNKNOWN.withDescription(e.getMessage()).withCause(e);
    }
}
