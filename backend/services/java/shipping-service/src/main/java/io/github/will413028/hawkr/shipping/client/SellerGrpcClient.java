package io.github.will413028.hawkr.shipping.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.seller.grpc.*;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);

    /**
     * issue #2294：內部讀取型 RPC 的 gRPC deadline（秒）。理由與取值同
     * {@link BuyerGrpcClient#INTERNAL_READ_DEADLINE_SEC}。本 client 的
     * {@code batchGetSellerMeta} 位於 §5.19 貨態補收排程的<b>租約區間內</b>
     * （{@code ShippingService.updateStatus} → {@code resolveSellerAccountId}），
     * 無 deadline 時「單筆最壞耗時 &lt; 租約」這條不變式無從建立。
     * ⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須每次呼叫時套用。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 批次取得賣家 Meta 資訊（含 merId、店名等）
     */
    public BatchGetSellerMetaResponse batchGetSellerMeta(List<Long> storeIds) {
        var stub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
        return stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetSellerMeta(BatchGetSellerMetaRequest.newBuilder()
                .addAllStoreIds(storeIds)
                .build());
    }

    /**
     * 取得賣家寄件人資訊與 PayUni 商家編號（僅供取得 contactEmail 使用）
     */
    public GetSellerShippingProfileResponse getSellerShippingProfile(long storeId) {
        var stub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
        return stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getSellerShippingProfile(GetSellerShippingProfileRequest.newBuilder()
                .setStoreId(storeId)
                .build());
    }
}
