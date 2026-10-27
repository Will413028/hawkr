package io.github.will413028.hawkr.common.utils.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;

import java.util.function.BiFunction;

/**
 * 將 gRPC {@link StatusRuntimeException} 轉換為呼叫端服務的業務異常。
 *
 * 用法：
 * <pre>
 *     try {
 *         var resp = stub.someRpc(req);
 *         return resp.getData();
 *     } catch (StatusRuntimeException e) {
 *         throw GrpcClientErrorTranslator.translate(e, OrderBusinessException::new);
 *     }
 * </pre>
 *
 * 設計與規範請見 docs/開發規範/Java gRPC 錯誤處理規範.md §7.6.3
 */
public final class GrpcClientErrorTranslator {

    private GrpcClientErrorTranslator() {}

    /**
     * 將 StatusRuntimeException 轉換為本服務的業務異常。
     *
     * @param e                gRPC StatusRuntimeException
     * @param exceptionFactory 本服務業務異常建構子（必須提供 (GrpcResponseCode, String) 建構子）
     */
    public static <T extends RuntimeException> T translate(
            StatusRuntimeException e,
            BiFunction<GrpcResponseCode, String, T> exceptionFactory) {
        Status status = e.getStatus();
        GrpcResponseCode code = GrpcStatusToResponseCode.map(status.getCode());
        String description = status.getDescription() != null ? status.getDescription() : code.message();
        return exceptionFactory.apply(code, description);
    }
}
