package io.github.will413028.hawkr.shipping.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.grpc.*;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

@Component
public class OrderGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(OrderGrpcClient.class);

    /**
     * issue #2294：內部讀取型 RPC 的 gRPC deadline（秒）。理由與取值同
     * {@link BuyerGrpcClient#INTERNAL_READ_DEADLINE_SEC}（共用元件 {@code GrpcStubFactory}
     * 全域零 deadline ⇒ 下游掛住時 blocking stub 永不返回，排程執行緒池預設只有 1 條）。
     * ⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須每次呼叫時套用。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public OrderGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 取得物流建單所需的訂單資訊（純內部讀）。
     *
     * <p>不帶 {@code store_id}：order-service 自 #1366 起改為純內部讀，授權由
     * HMAC internal-auth + netpol 保證，{@code GetOrderForShippingRequest.store_id}
     * 已不再用於歸屬校驗（見 {@code OrderGrpcService.getOrderForShipping} 內註解與
     * {@code OrderServiceGetOrderForShippingTest} 的 "regardless of store_id" 案例）。
     */
    public GetOrderForShippingResponse getOrderForShipping(String orderNo) {
        var stub = grpcStubFactory.blocking("order", OrderServiceGrpc::newBlockingStub);
        var resp = stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getOrderForShipping(GetOrderForShippingRequest.newBuilder()
                .setOrderNo(orderNo)
                .build());
        // 22 號設計書 §2.7 共通規範：非 SUCCESS 拋業務例外，避免 caller 僅以 !hasData() 防護不完整
        if (resp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_ORDER_QUERY_FAILED, resp.getMessage());
        }
        return resp;
    }
}
