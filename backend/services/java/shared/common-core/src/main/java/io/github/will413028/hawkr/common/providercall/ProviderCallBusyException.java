package io.github.will413028.hawkr.common.providercall;

/**
 * acquireRequested 失敗時拋出：表示該資源正被另一個執行緒占用（status 為 REQUESTED），
 * 或已是終態 SUCCEEDED 不允許重新發起。
 *
 * <p>呼叫端可選擇等待 / 回傳 {@code SHIPMENT_PROVIDER_CALL_IN_PROGRESS} 業務碼。
 *
 * <p>設計參見 06-Shipping §2.4.4。
 */
public class ProviderCallBusyException extends RuntimeException {

    private final String callKind;
    private final long resourceId;

    public ProviderCallBusyException(ProviderCallContext ctx) {
        super("Provider call busy: kind=" + ctx.callKind() + ", resourceId=" + ctx.resourceId());
        this.callKind = ctx.callKind();
        this.resourceId = ctx.resourceId();
    }

    public String getCallKind() {
        return callKind;
    }

    public long getResourceId() {
        return resourceId;
    }
}
