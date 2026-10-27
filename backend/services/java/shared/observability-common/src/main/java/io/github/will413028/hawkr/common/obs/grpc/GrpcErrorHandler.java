package io.github.will413028.hawkr.common.obs.grpc;

import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import com.google.protobuf.Descriptors;
import com.google.protobuf.GeneratedMessageV3;
import io.grpc.stub.StreamObserver;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;

/**
 * gRPC catch-all 錯誤集中處理工具。
 *
 * <p>統一處理 RPC 方法中未被業務異常捕獲的例外：
 * <ul>
 *   <li>{@code IllegalArgumentException} → {@code INVALID_PARAM(400)} + {@code e.getMessage()}</li>
 *   <li>其他 {@code Exception} → 指定的系統錯誤碼 + {@code GrpcResponseCode.message()}</li>
 * </ul>
 *
 * <p>日誌格式與 {@code ObsLogger} 一致，手動設定 MDC {@code event} 欄位：
 * <ul>
 *   <li>參數錯誤：event = {@code {className}.{method}.invalidParam}</li>
 *   <li>系統錯誤：event = {@code {className}.{method}.error}</li>
 * </ul>
 *
 * <p>使用方式：
 * <pre>{@code
 * } catch (Exception e) {
 *     GrpcErrorHandler.handleException(responseObserver, XxxResponse.newBuilder(),
 *             e, "OrderGrpcService", "createOrder", GrpcResponseCode.ORDER_SYSTEM_ERROR);
 * }
 * }</pre>
 *
 * @see GrpcResponseCode
 */
public final class GrpcErrorHandler {

    private GrpcErrorHandler() {
    }

    /**
     * 處理 catch-all 例外（系統錯誤碼預設 {@code INTERNAL_ERROR(500)}）。
     */
    @SuppressWarnings("unchecked")
    public static <T extends GeneratedMessageV3, B extends GeneratedMessageV3.Builder<B>>
    void handleException(StreamObserver<T> responseObserver, B builder, Exception e,
                         String className, String method) {
        handleException(responseObserver, builder, e, className, method, GrpcResponseCode.INTERNAL_ERROR);
    }

    /**
     * 處理 catch-all 例外（指定系統錯誤碼）。
     *
     * @param className       gRPC Service 類別名稱（如 {@code "OrderGrpcService"}）
     * @param method          RPC 方法名稱（如 {@code "createOrder"}）
     * @param systemErrorCode 系統錯誤時使用的錯誤碼（如 {@code ORDER_SYSTEM_ERROR(5000)}）
     */
    @SuppressWarnings("unchecked")
    public static <T extends GeneratedMessageV3, B extends GeneratedMessageV3.Builder<B>>
    void handleException(StreamObserver<T> responseObserver, B builder, Exception e,
                         String className, String method,
                         GrpcResponseCode systemErrorCode) {
        Logger logger = LoggerFactory.getLogger(className);
        try {
            if (e instanceof io.grpc.StatusRuntimeException sre) {
                // gRPC 狀態異常（權限/下游服務）— 直接透傳
                responseObserver.onError(sre);
                return;
            } else if (e instanceof GrpcBusinessException biz) {
                // 業務異常（ShippingBusinessException、OrderBusinessException 等）— 回傳業務碼 + 具體訊息
                // 日誌一律帶 exception class；若業務異常包裝了下游真實失敗（cause 非 null）則一併印 stack，
                // 純業務拒絕（無 cause）不印 stack，避免常規流程刷 noise。
                Throwable cause = e.getCause();
                withEvent(className + "." + method + ".businessError", () -> {
                    if (cause != null) {
                        logger.warn("{}: {}", e.getClass().getSimpleName(), e.getMessage(), e);
                    } else {
                        logger.warn("{}: {}", e.getClass().getSimpleName(), e.getMessage());
                    }
                });
                setResponse(builder, biz.code(), e.getMessage());
            } else if (e instanceof IllegalArgumentException) {
                withEvent(className + "." + method + ".invalidParam", () ->
                        logger.warn(e.getMessage()));
                setResponse(builder, GrpcResponseCode.INVALID_PARAM.code(), e.getMessage());
            } else {
                withEvent(className + "." + method + ".error", () ->
                        logger.error(method + " 失敗", e));
                setResponse(builder, systemErrorCode.code(), systemErrorCode.message());
            }
            responseObserver.onNext((T) builder.build());
            responseObserver.onCompleted();
        } catch (Exception ex) {
            responseObserver.onError(io.grpc.Status.INTERNAL
                    .withDescription(GrpcResponseCode.INTERNAL_ERROR.message())
                    .asRuntimeException());
        }
    }

    /**
     * 設定 MDC event 欄位並執行日誌動作，結束後清除。
     * 與 {@code ObsLogger} 的 MDC 管理機制一致。
     */
    private static void withEvent(String event, Runnable logAction) {
        MDC.put(ObsKeys.EVENT, event);
        try {
            logAction.run();
        } finally {
            MDC.remove(ObsKeys.EVENT);
        }
    }

    private static void setResponse(GeneratedMessageV3.Builder<?> builder, int code, String message) {
        Descriptors.FieldDescriptor codeField = builder.getDescriptorForType().findFieldByName("code");
        Descriptors.FieldDescriptor messageField = builder.getDescriptorForType().findFieldByName("message");
        if (codeField != null) {
            builder.setField(codeField, code);
        }
        if (messageField != null) {
            builder.setField(messageField, message);
        }
    }
}
