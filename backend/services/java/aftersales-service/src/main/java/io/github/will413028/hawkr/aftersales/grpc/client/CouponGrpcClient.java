package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.coupon.grpc.*;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

@Component
public class CouponGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(CouponGrpcClient.class);

    /**
     * {@code grantSellerCompensationCoupon} 的 gRPC deadline（秒）——issue #3332。
     *
     * <p>下游是 coupon-service 的發券寫入（本地 DB），<b>不觸及第三方</b>
     * ——補償券是平台自己發的，沒有金流或物流商參與 ⇒ 取內部呼叫慣例 10 秒。
     *
     * <p>⚠️ 必須<b>每次呼叫時</b>套用（{@code withDeadlineAfter} 換算的是絕對時間）；
     * 不得改共用的 {@code GrpcStubFactory}。
     */
    public static final long INTERNAL_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public CouponGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * @param sourceId 售後單 id（{@code after_sale_cases.id}）：coupon 端以 (AFTERSALES_CASE, sourceId) 去重，
     *                 重送同一張售後單只發一張券
     */
    public GrantSellerCompensationCouponResponse grantSellerCompensationCoupon(String storeId,
                                                                               String buyerId,
                                                                               String amount,
                                                                               String minOrderAmount,
                                                                               String message,
                                                                               String refCaseNo,
                                                                               String sourceId) {
        if (sourceId == null || sourceId.isBlank()) {
            // source_id 是補償券的冪等鍵（D4）：空值不能送出去，否則重送會重複發券。
            throw new IllegalArgumentException("source_id（售後單 id）必填");
        }
        log.info("CouponGrpcClient.grantSellerCompensationCoupon.start", "發放補償優惠券", "caseNo", refCaseNo);
        var stub = grpcStubFactory.blocking("coupon", CouponServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.grantSellerCompensationCoupon(
            GrantSellerCompensationCouponRequest.newBuilder()
                .setStoreId(storeId)
                .setBuyerId(buyerId)
                .setAmount(amount)
                .setMinOrderAmount(minOrderAmount)
                .setMessage(message)
                .setRefCaseNo(refCaseNo)
                .setSourceId(sourceId)
                .build()
        );
        log.info("CouponGrpcClient.grantSellerCompensationCoupon.done", "發放補償優惠券完成, code=" + resp.getCode(), "caseNo", refCaseNo);
        return resp;
    }
}
