package io.github.will413028.hawkr.order.checkout.client;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.coupon.grpc.*;
import org.springframework.stereotype.Component;
import java.util.concurrent.TimeUnit;

/**
 * Coupon Service gRPC Client（結帳模組用）
 *
 * couponTemplateId 非空時為硬依賴，失敗直接拋出例外（不降級為 discount=0）
 */
@Component
public class CheckoutCouponGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(CheckoutCouponGrpcClient.class);

    /**
     * issue #2294：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline 設定 ⇒ 下游若是「掛住」而非「失敗」，
     * blocking stub <b>永不返回</b>。本 client 被 {@code @Scheduled} worker 使用，而 Spring Boot
     * 的 {@code spring.task.scheduling.pool.size} 預設是 <b>1</b> ⇒ 一次掛住就讓該服務的
     * <b>所有</b>排程停擺且永不恢復；同時使排程搶佔的租約下界
     * （《多副本排程搶佔規範》§三第 5 步「租約必須 &gt; 單筆工作的最壞耗時」）無從建立。
     *
     * <p><b>10 秒怎麼來的</b>：這幾支都是下游服務的<b>本地 DB 讀取</b>（無第三方、無跨服務串接），
     * p99 為毫秒級；10 秒是兩個數量級以上的餘裕，只用來把「永遠不回」轉成「10 秒後失敗」。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}：後者是 observability-common
     * 的共用元件，動它會一次改掉全 repo 所有 Java gRPC 呼叫的逾時語意。
     * ⚠️ {@code withDeadlineAfter} 換算的是<b>絕對</b>時間，因此必須<b>每次呼叫時</b>套用；
     * 挪進建構子 ＝ 啟動 10 秒後全部逾時。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final CouponServiceGrpc.CouponServiceBlockingStub couponStub;

    public CheckoutCouponGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.couponStub = grpcStubFactory.blocking("coupon", CouponServiceGrpc::newBlockingStub);
    }

    /**
     * 驗證優惠券（Preview / Confirm 呼叫，不消耗）
     *
     * @param templateId      優惠券 Template ID
     * @param buyerId         買家 buyer_id
     * @param storeId         賣家 store_id surrogate
     * @param orderAmount     訂單金額（新台幣元）
     * @return 驗證結果
     * @throws RuntimeException Coupon Service 不可用或驗證失敗
     */
    public CouponValidateResult validateCoupon(String templateId, long buyerId,
                                                long storeId, long orderAmount) {
        log.info("CheckoutCouponGrpcClient.validateCoupon.start", "驗證優惠券",
                "templateId", templateId);

        ValidateCouponRequest request = ValidateCouponRequest.newBuilder()
                .setTemplateId(templateId)
                .setBuyerId(buyerId)
                .setStoreId(String.valueOf(storeId))
                .setOrderAmount(String.valueOf(orderAmount))
                .build();

        // coupon-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        ValidateCouponResponse response = couponStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .validateCoupon(request);
        CouponValidateResult result = response.getData();
        if (!result.getValid()) {
            throw new OrderBusinessException(GrpcResponseCode.COUPON_NOT_AVAILABLE, result.getDescription());
        }

        log.info("CheckoutCouponGrpcClient.validateCoupon.done", "優惠券驗證通過",
                "discountAmount", result.getDiscountAmount());
        return result;
    }

    /**
     * 鎖定優惠券（Confirm 階段，Validate 後呼叫）
     * 硬依賴：失敗直接拋異常
     *
     * @param templateId     優惠券 Template ID
     * @param buyerId 買家 buyer_id
     * @param orderNo        關聯訂單編號
     * @return 新建的 Instance ID（寫入 orders.coupon_instance_id）
     */
    public String reserveCoupon(String templateId, long buyerId, String orderNo) {
        log.info("CheckoutCouponGrpcClient.reserveCoupon.start", "鎖定優惠券",
                "templateId", templateId);

        ReserveCouponRequest request = ReserveCouponRequest.newBuilder()
                .setTemplateId(templateId)
                .setBuyerId(buyerId)
                .setOrderNo(orderNo)
                .build();

        // coupon-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        ReserveCouponResponse response = couponStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .reserveCoupon(request);

        if (!response.getSuccess()) {
            throw new OrderBusinessException(GrpcResponseCode.COUPON_NOT_AVAILABLE, "優惠券鎖定失敗");
        }

        String instanceId = response.getInstanceId();
        log.info("CheckoutCouponGrpcClient.reserveCoupon.done",
                "優惠券鎖定成功，orderNo=" + orderNo + ", instanceId=" + instanceId);
        return instanceId;
    }

    /**
     * 消耗優惠券（付款成功後標記已使用）
     * 軟依賴：失敗僅記錄警告，不阻斷主流程
     *
     * @param instanceId     優惠券 Instance ID
     * @param buyerId 買家 buyer_id
     * @param orderNo        訂單編號
     */
    public void consumeCoupon(String instanceId, long buyerId, String orderNo) {
        try {
            log.info("CheckoutCouponGrpcClient.consumeCoupon.start", "消耗優惠券",
                    "instanceId", instanceId);

            ConsumeCouponRequest request = ConsumeCouponRequest.newBuilder()
                    .setInstanceId(instanceId)
                    .setBuyerId(buyerId)
                    .setOrderNo(orderNo)
                    .build();

            ConsumeCouponResponse response = couponStub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .consumeCoupon(request);

            if (!response.getSuccess()) {
                log.warn("CheckoutCouponGrpcClient.consumeCoupon.bizError",
                        "消耗優惠券回傳非成功，instanceId=" + instanceId + ", orderNo=" + orderNo);
            } else {
                log.info("CheckoutCouponGrpcClient.consumeCoupon.done", "消耗優惠券完成",
                        "orderNo", orderNo);
            }
        } catch (Exception e) {
            log.warn("CheckoutCouponGrpcClient.consumeCoupon.error",
                    "消耗優惠券 gRPC 失敗（訂單已付款），instanceId=" + instanceId);
        }
    }

    /**
     * 釋放優惠券（取消/逾時）
     * 軟依賴：失敗僅記錄警告，不阻斷主流程
     *
     * @param instanceId 優惠券 Instance ID
     * @param orderNo    訂單編號（僅 match 此 order_no 才釋放）
     */
    public void releaseCoupon(String instanceId, String orderNo) {
        try {
            log.info("CheckoutCouponGrpcClient.releaseCoupon.start", "釋放優惠券",
                    "instanceId", instanceId);

            ReleaseCouponRequest request = ReleaseCouponRequest.newBuilder()
                    .setInstanceId(instanceId)
                    .setOrderNo(orderNo)
                    .build();

            ReleaseCouponResponse response = couponStub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .releaseCoupon(request);

            if (!response.getSuccess()) {
                log.warn("CheckoutCouponGrpcClient.releaseCoupon.bizError",
                        "釋放優惠券回傳非成功，instanceId=" + instanceId + ", orderNo=" + orderNo);
            } else {
                log.info("CheckoutCouponGrpcClient.releaseCoupon.done", "釋放優惠券完成",
                        "orderNo", orderNo);
            }
        } catch (Exception e) {
            log.warn("CheckoutCouponGrpcClient.releaseCoupon.error",
                    "釋放優惠券 gRPC 失敗（訂單已取消），instanceId=" + instanceId);
        }
    }
}
