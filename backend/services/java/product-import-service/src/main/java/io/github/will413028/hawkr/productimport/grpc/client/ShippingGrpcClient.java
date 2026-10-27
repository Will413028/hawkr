package io.github.will413028.hawkr.productimport.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.grpc.ListSellerShippingMethodsRequest;
import io.github.will413028.hawkr.shipping.grpc.ListSellerShippingMethodsResponse;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingMethodView;
import io.github.will413028.hawkr.shipping.grpc.ShippingServiceGrpc;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

@Component
public class ShippingGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ShippingGrpcClient.class);
    private static final String TARGET = "shipping";
    private static final int ACCOUNT_TYPE_SELLER = 2;

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 由匯入 pipeline 呼叫，沒有上界等於把該條 worker 永久吃掉。
     *
     * <p><b>10 秒怎麼來的</b>：實查 shipping-service 的 {@code ListSellerShippingMethods}
     * 只做 {@code onboardingService.listSellerShippingMethods} 這一次<b>本地 DB 讀取</b>，
     * <b>不</b>經 PayUni 物流 API、不再跨服務。取值與 {@code shipping-service} 自己的
     * {@code INTERNAL_READ_DEADLINE_SEC} 一致。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public ShippingGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    private ShippingServiceGrpc.ShippingServiceBlockingStub stub() {
        return grpcStubFactory.blocking(TARGET, ShippingServiceGrpc::newBlockingStub);
    }

    public List<SellerShippingMethodView> listSellerShippingMethods(long storeId) {
        try {
            ListSellerShippingMethodsResponse resp = stub()
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .listSellerShippingMethods(
                    ListSellerShippingMethodsRequest.newBuilder()
                            .setStoreId(storeId)
                            .setAccountType(ACCOUNT_TYPE_SELLER)
                            .build());
            return resp.getMethodsList();
        } catch (StatusRuntimeException e) {
            log.error("ShippingGrpcClient.listSellerShippingMethods.error",
                    "ListSellerShippingMethods failed: " + e.getStatus(), e);
            throw e;
        }
    }
}
