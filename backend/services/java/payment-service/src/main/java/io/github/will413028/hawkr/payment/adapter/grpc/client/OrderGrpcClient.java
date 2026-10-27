package io.github.will413028.hawkr.payment.adapter.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.grpc.GetOrderForPaymentRequest;
import io.github.will413028.hawkr.order.grpc.GetOrderForPaymentResponse;
import io.github.will413028.hawkr.order.grpc.OrderServiceGrpc;
import io.github.will413028.hawkr.order.grpc.SnapshotPaymentInfoRequest;
import io.github.will413028.hawkr.order.grpc.SnapshotPaymentInfoResponse;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

/**
 * Order Service gRPC Client
 *
 * 提供訂單付款資訊查詢（供 Payment Service 建立付款交易時使用）
 */
@Component
public class OrderGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(OrderGrpcClient.class);

    /**
     * 本 client 兩支 RPC 的 gRPC deadline（秒）——issue #3332。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline 設定，而 Java 的 {@code ManagedChannelBuilder}
     * 預設無 keepalive ⇒ 沒有 deadline 就是「單筆最壞耗時無上界」（CLAUDE.md §12 禁止）。
     *
     * <p><b>10 秒的理由</b>：兩支下游都在 order-service 內收斂、<b>不觸及 PayUni</b>
     * （{@code getOrderForPayment} 是純 DB 讀；{@code snapshotPaymentInfo} 是付款方式快照的
     * DB 寫入）⇒ 最壞耗時是本地 DB 等級，取內部呼叫慣例 10 秒。
     *
     * <p>⚠️ <b>必須在每次呼叫時套用</b>：{@code withDeadlineAfter} 以「呼叫當下」為起點換算成
     * <b>絕對</b> deadline。本類的 stub 是<b>建構子時建立的欄位</b>，若把 deadline 綁在建構子，
     * 那個 deadline 會從<b>服務啟動那一刻</b>起算 ⇒ 啟動 10 秒後所有呼叫一律
     * {@code DEADLINE_EXCEEDED}。同時<b>不得</b>改共用的 {@code GrpcStubFactory}——那會一次改掉
     * 全 repo 所有 Java gRPC 呼叫的逾時語意。
     */
    public static final long INTERNAL_DEADLINE_SEC = 10L;

    private final OrderServiceGrpc.OrderServiceBlockingStub orderStub;

    public OrderGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.orderStub = grpcStubFactory.blocking("order", OrderServiceGrpc::newBlockingStub);
    }

    /**
     * 取得訂單付款資訊（內部服務間呼叫）
     *
     * @param orderNo 訂單編號
     * @return GetOrderForPaymentResponse（含 mer_id, amount, buyer_id, status, product_name, store_id）
     */
    public GetOrderForPaymentResponse getOrderForPayment(String orderNo) {
        log.info("OrderGrpcClient.getOrderForPayment.start", "取得訂單付款資訊",
                "orderNo", orderNo);

        GetOrderForPaymentRequest request = GetOrderForPaymentRequest.newBuilder()
                .setOrderNo(orderNo)
                .build();

        // 不能挪到建構子——理由見 INTERNAL_DEADLINE_SEC 的最後一段。
        GetOrderForPaymentResponse response = orderStub
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS)
                .getOrderForPayment(request);

        log.info("OrderGrpcClient.getOrderForPayment.done", "取得訂單付款資訊完成",
                "orderNo", orderNo);

        return response;
    }

    /**
     * §2A.7 訂單付款方式快照同步（best-effort，不阻斷主流程）。
     * <p>
     * 觸發點：handleSwitch / 重複付款翻轉 / ATM 復活 / webhook PAID 等。
     */
    public void snapshotPaymentInfo(String orderNo,
                                    String tradeNo,
                                    String paymentMethodValue,
                                    String atmBankCode,
                                    String atmVirtualAccount,
                                    String atmExpireDate) {
        try {
            SnapshotPaymentInfoRequest req = SnapshotPaymentInfoRequest.newBuilder()
                    .setOrderNo(orderNo == null ? "" : orderNo)
                    .setTradeNo(tradeNo == null ? "" : tradeNo)
                    .setPaymentMethod(paymentMethodValue == null ? "" : paymentMethodValue)
                    .setAtmBankCode(atmBankCode == null ? "" : atmBankCode)
                    .setAtmVirtualAccount(atmVirtualAccount == null ? "" : atmVirtualAccount)
                    .setAtmExpireDate(atmExpireDate == null ? "" : atmExpireDate)
                    .build();
            // 不能挪到建構子——理由見 INTERNAL_DEADLINE_SEC 的最後一段。
            SnapshotPaymentInfoResponse resp = orderStub
                    .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS)
                    .snapshotPaymentInfo(req);
            if (resp.getCode() != 0) {
                log.warn("OrderGrpcClient.snapshotPaymentInfo.bizFail",
                        "snapshotPaymentInfo 業務失敗 order_no=" + orderNo
                                + " code=" + resp.getCode() + " msg=" + resp.getMessage());
            }
        } catch (Exception e) {
            // best-effort：失敗不阻斷主流程，僅 log
            log.warn("OrderGrpcClient.snapshotPaymentInfo.failed",
                    "snapshotPaymentInfo gRPC 失敗 order_no=" + orderNo + " err=" + e.getMessage());
        }
    }
}
