package io.github.will413028.hawkr.common.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * 代行語境判定失敗（56 §6.2.4 S1a–S5、§7.1.2 Q4）。
 *
 * <p>不實作 {@code GrpcBusinessException}：那個介面在 observability-common，而該模組
 * 依賴 common-core，反向依賴會成環。⇒ 各 RPC 方法自行 catch 本例外並把 {@link #code()}
 * 填進回應，這也讓每一支的錯誤碼在呼叫處看得見。
 */
public class DelegationDeniedException extends RuntimeException {

    private static final long serialVersionUID = 1L;

    private final transient GrpcResponseCode responseCode;

    public DelegationDeniedException(GrpcResponseCode responseCode) {
        this(responseCode, responseCode.message());
    }

    public DelegationDeniedException(GrpcResponseCode responseCode, String message) {
        super(message);
        this.responseCode = responseCode;
    }

    public GrpcResponseCode responseCode() {
        return responseCode;
    }

    public int code() {
        return responseCode.code();
    }
}
