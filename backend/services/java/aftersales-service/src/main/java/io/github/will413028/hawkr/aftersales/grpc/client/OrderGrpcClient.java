package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.grpc.*;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

@Component
public class OrderGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(OrderGrpcClient.class);

    /**
     * 本 client 所有 RPC 的 gRPC deadline（秒）——issue #3332。
     *
     * <p>兩支下游都在 order-service 內收斂、<b>不觸及第三方</b>：
     * <ul>
     *   <li>{@code getOrderForAfterSales} → 純 DB 讀。</li>
     *   <li>{@code cancelOrderInternal} → {@code OrderService.cancelOrderInternalInTx}
     *       （狀態 CAS ＋ {@code order.cancelled} outbox 同事務落地）＋ TX 外 best-effort 釋券
     *       ＋ MQ emit。<b>沒有</b>任何同步 PayUni 呼叫（付款關閉走 outbox／事件，不在本 RPC 內）。</li>
     * </ul>
     * ⇒ 最壞耗時是本地 DB 等級，取內部呼叫慣例 10 秒。
     *
     * <p>⚠️ {@code cancelOrderInternal} 在 order 端會再往 coupon 發一次 gRPC（釋券，best-effort）。
     * 那一跳的時間上界由 order-service 自己的 client deadline 負責；本 deadline 只保證
     * <b>aftersales 這一側</b>不會無限等待。
     *
     * <p>⚠️ 必須<b>每次呼叫時</b>套用（{@code withDeadlineAfter} 換算的是絕對時間）；
     * 不得改共用的 {@code GrpcStubFactory}。
     */
    public static final long INTERNAL_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public OrderGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    public GetOrderForAfterSalesResponse getOrderForAfterSales(String orderNo) {
        log.info("OrderGrpcClient.getOrderForAfterSales.start", "查詢售後訂單資訊", "orderNo", orderNo);
        var stub = grpcStubFactory.blocking("order", OrderServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.getOrderForAfterSales(
            GetOrderForAfterSalesRequest.newBuilder().setOrderNo(orderNo).build()
        );
        log.info("OrderGrpcClient.getOrderForAfterSales.done", "查詢售後訂單資訊完成, code=" + resp.getCode(), "orderNo", orderNo);
        // 22 號設計書 §2.7 共通規範：caller 不可僅依 typed data 預設值判斷成敗，
        // 必須先檢 response.code；非 SUCCESS 拋業務例外以避免空 OrderAfterSalesView 觸發後續 parseLongRequired 噴 INVALID_PARAM
        if (resp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new AfterSalesBusinessException(GrpcResponseCode.ORDER_QUERY_FAILED, resp.getMessage());
        }
        return resp;
    }

    public CancelOrderInternalResponse cancelOrderInternal(String orderNo, String reason, String remark, String cancelByRole) {
        log.info("OrderGrpcClient.cancelOrderInternal.start", "售後內部取消訂單", "orderNo", orderNo);
        var stub = grpcStubFactory.blocking("order", OrderServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.cancelOrderInternal(
            CancelOrderInternalRequest.newBuilder()
                .setOrderNo(orderNo)
                .setCancelReason(reason)
                .setCancelRemark(remark)
                .setCancelByRole(cancelByRole)
                .build()
        );
        log.info("OrderGrpcClient.cancelOrderInternal.done", "售後內部取消訂單完成, code=" + resp.getCode(), "orderNo", orderNo);
        if (resp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new AfterSalesBusinessException(GrpcResponseCode.ORDER_QUERY_FAILED, resp.getMessage());
        }
        return resp;
    }
}
