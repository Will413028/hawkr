package io.github.will413028.hawkr.coupon.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.grpc.GetBuyerMerchantSummaryRequest;
import io.github.will413028.hawkr.order.grpc.GetBuyerMerchantSummaryResponse;
import io.github.will413028.hawkr.order.grpc.GetOrderForAfterSalesRequest;
import io.github.will413028.hawkr.order.grpc.GetOrderForAfterSalesResponse;
import io.github.will413028.hawkr.order.grpc.OrderServiceGrpc;
import org.springframework.stereotype.Component;

import java.util.Collections;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

/**
 * Order Service gRPC Client
 *
 * 用於 MEMBER target_type 資格判定：查詢買家有消費紀錄的賣家列表
 */
@Component
public class OrderGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(OrderGrpcClient.class);

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * {@code getOrderStatus} 由 {@code CouponReconciliationJob}（{@code @Scheduled}）逐筆呼叫，
     * 而 Spring Boot 的 {@code spring.task.scheduling.pool.size} 預設是 <b>1</b>
     * ⇒ 一次掛住就讓本服務的<b>所有</b>排程停擺且永不恢復；兩支的降級（空集合／null）
     * 在「永不返回」下都到不了。
     *
     * <p><b>10 秒怎麼來的</b>：兩支在 order-service 內都是<b>本地 DB 讀取</b>，p99 毫秒級；
     * 取值與 {@code shipping-service} 的 {@code OrderGrpcClient.INTERNAL_READ_DEADLINE_SEC} 一致。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;
    private final GrpcStubFactory grpcStubFactory;

    public OrderGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 取得買家有消費紀錄的所有賣家商店 ID
     *
     * @param buyerId gateway 已解析的 buyer_id（order-service 的 orders 以 buyer_id 建鍵）
     * @return 賣家 store_id 集合（讀 GetBuyerMerchantSummary 回應的 store_id 欄位），
     *         失敗時回傳空集合（降級）
     */
    public Set<Long> getBuyerMemberSellerIds(Long buyerId) {
        try {
            var stub = grpcStubFactory.blocking("order", OrderServiceGrpc::newBlockingStub);
            var request = GetBuyerMerchantSummaryRequest.newBuilder()
                    .setBuyerId(buyerId)
                    .build();

            GetBuyerMerchantSummaryResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getBuyerMerchantSummary(request);

            return response.getDataList().stream()
                    .map(item -> {
                        try {
                            return Long.parseLong(item.getStoreId());
                        } catch (NumberFormatException e) {
                            return null;
                        }
                    })
                    .filter(id -> id != null)
                    .collect(Collectors.toSet());
        } catch (Exception e) {
            log.error("OrderGrpcClient.getBuyerMemberSellerIds.fail", "Order Service 呼叫失敗，降級為空", e);
            return Collections.emptySet();
        }
    }

    /**
     * 取得訂單真實狀態（供 CouponReconciliationJob 對帳用）
     *
     * @return OrderStatus value（如 "0"=PENDING_PAYMENT / "10"=PAID / "80"=CANCELLED），
     *         查無訂單或失敗回傳 null（呼叫端依此決定動作）
     */
    public String getOrderStatus(String orderNo) {
        try {
            var stub = grpcStubFactory.blocking("order", OrderServiceGrpc::newBlockingStub);
            GetOrderForAfterSalesResponse resp = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getOrderForAfterSales(
                            GetOrderForAfterSalesRequest.newBuilder().setOrderNo(orderNo).build());
            if (resp.getCode() != 0 || !resp.hasData()) {
                return null;
            }
            return resp.getData().getStatus();
        } catch (Exception e) {
            log.warn("OrderGrpcClient.getOrderStatus.fail",
                    "查訂單狀態失敗 orderNo=" + orderNo + " err=" + e.getMessage());
            return null;
        }
    }
}
