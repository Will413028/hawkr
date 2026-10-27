package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.invoice.grpc.InvoiceServiceGrpc;
import io.github.will413028.hawkr.invoice.grpc.ResolveOrderInvoiceModeInternalRequest;
import io.github.will413028.hawkr.invoice.grpc.ResolveOrderInvoiceModeInternalResponse;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalRequest;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalResponse;
import org.springframework.stereotype.Component;
import java.util.concurrent.TimeUnit;

/**
 * Invoice Service gRPC Client（內部呼叫，58 號 §6.2.4 order 的第 ③ 項）。
 *
 * <p>兩支都接了：preview 的 {@code ResolveOrderInvoiceModeInternal}（②）與
 * confirm 的 {@code ValidateOrderInvoiceInputInternal}（③）。
 *
 * <p>兩者的失敗處置<b>刻意不同</b>：
 * <ul>
 *   <li><b>confirm</b>：打不到就<b>放行</b>——fail-closed 會讓 invoice-service 變成
 *       整條金流的硬依賴，而發票正確性的保護在 {@code order.paid} 那一側。</li>
 *   <li><b>preview</b>：打不到就回<b>空</b>，前端會顯示不了發票區——同樣不擋結帳。
 *       preview 是唯讀展示，讓它擋住整個結帳頁的載入是不成比例的。</li>
 * </ul>
 */
@Component
public class InvoiceGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(InvoiceGrpcClient.class);

    private final InvoiceServiceGrpc.InvoiceServiceBlockingStub stub;

    public InvoiceGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.stub = grpcStubFactory.blocking("invoice", InvoiceServiceGrpc::newBlockingStub);
    }

    /**
     * 結帳 confirm 前驗證買家的發票輸入。
     *
     * <p><b>回 null ＝ 這一輪不做驗證</b>（invoice-service 不可用）。呼叫端照常建單。
     *
     * <p>🔴 <b>為什麼是 fail-open 而不是 fail-closed</b>：發票的正確性保護<b>不依賴</b>
     * 這一格。§7.1.3 在 {@code order.paid} 當下才判定開票模式，非 {@code AMEGO_AUTO}
     * 一律落 {@code AWAITING_MANUAL} 或 {@code NOT_APPLICABLE}，不會開出錯的發票。
     * 這裡 fail-closed 的代價是：invoice-service 一有問題，<b>整個賣場的結帳全部停擺</b>
     * —— 把一個「開票前置檢查」升級成整條金流的硬依賴，代價與收益完全不成比例。
     *
     * <p>被放行的只有 9326（待審接單雙上限）這一格的短暫失效，而它本身是軟上限。
     */
    public ValidateOrderInvoiceInputInternalResponse validateOrderInvoiceInput(
            ValidateOrderInvoiceInputInternalRequest request) {
        try {
            return stub
                    .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .validateOrderInvoiceInputInternal(request);
        } catch (Exception e) {
            log.warn("InvoiceGrpcClient.validateOrderInvoiceInput.error",
                    "發票前置驗證不可用，本次結帳略過該檢查: " + e.getMessage(),
                    "orderNo", request.getOrderNo());
            return null;
        }
    }

    /**
     * 結帳 preview 解析本單的發票模式（§6.2.4 order ②、58A §B01）。
     *
     * <p><b>回 null ＝ 這一輪拿不到</b>。呼叫端不設 {@code invoice} 欄，前端據此
     * 不渲染發票區（而不是渲染一個空的）。
     *
     * <p>🔴 <b>不 fail-closed</b>：preview 是唯讀展示，讓它擋住整個結帳頁的載入
     * 與收益完全不成比例。真正的閘門在 confirm（那一支回非 0 業務碼會擋下建單）。
     */
    public ResolveOrderInvoiceModeInternalResponse resolveOrderInvoiceMode(
            ResolveOrderInvoiceModeInternalRequest request) {
        try {
            return stub
                    .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .resolveOrderInvoiceModeInternal(request);
        } catch (Exception e) {
            log.warn("InvoiceGrpcClient.resolveOrderInvoiceMode.error",
                    "發票模式解析不可用，本次 preview 不帶發票區: " + e.getMessage(),
                    "storeId", String.valueOf(request.getStoreId()));
            return null;
        }
    }
}
