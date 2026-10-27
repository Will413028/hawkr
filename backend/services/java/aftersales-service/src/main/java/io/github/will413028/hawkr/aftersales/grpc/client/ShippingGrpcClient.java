package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.grpc.*;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

@Component
public class ShippingGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ShippingGrpcClient.class);

    /**
     * 建立<b>退貨物流單</b>（超商／宅配）的 gRPC deadline（秒）——issue #3332。
     *
     * <p>🔴 <b>這兩支是本 client 唯一真的同步打第三方的路徑，值刻意取大</b>，理由與
     * {@code PaymentGrpcClient.CREATE_REFUND_DEADLINE_SEC} 同源：deadline 若小於第三方的最壞往返，
     * 一筆<b>其實已在 PayUni 物流端建立成功</b>的退貨單會在我方變成 {@code DEADLINE_EXCEEDED}，
     * 而 shipping-service 那側的 shipment 仍會落地 ⇒ 售後案件停在原狀、物流單無人認領。
     *
     * <p><b>190 秒的推導（取最壞路徑）</b>：
     * <ol>
     *   <li>shipping-service 的 {@code createReturnCvsShipment} / {@code createReturnHomeShipment}
     *       同步呼叫 {@code ReturnCapability.createReturnShipment}
     *       （{@code PayUniCvsAdapter} / {@code PayUniHomeAdapter}）。</li>
     *   <li>該 adapter 依「收斂規範 §8.1：同步 retry 最多一次」<b>連續呼叫 PayUni 兩次</b>
     *       （{@code resp = shippingClient.createXxx(...); if (!resp.isSuccess()) resp = shippingClient.createXxx(...);}）。</li>
     *   <li>單次交換上界由共用 {@code PayUniHttpClient} 決定：connect 30s ＋ write 30s ＋ read 30s
     *       ＝ <b>90 秒</b>（{@code payuni-java-client/.../PayUniHttpClient.java}）。</li>
     *   <li>2 × 90 ＝ 180 秒，再加 shipping 端本地 DB 與 gRPC 傳輸餘裕 10 秒 ⇒ <b>190 秒</b>。</li>
     * </ol>
     *
     * <p>⚠️ <b>不可套用 100 秒的「打 PayUni 慣例值」</b>：那個值的推導是<b>單次</b>交換（90＋10），
     * 而這條路徑是<b>兩次</b>。照抄 100 會讓第二次嘗試必定被砍斷，正是上面要避免的情形。
     *
     * <p>無租約上界衝突：呼叫端 {@code AfterSaleResolutionService.handleReturnRefund} 是賣家同步操作，
     * 不在任何 {@code @Scheduled} 租約內（對照 {@code AfterSaleLabelRetryService} 的 6 分鐘租約）。
     */
    public static final long RETURN_SHIPMENT_DEADLINE_SEC = 190L;

    /**
     * 本 client 其餘 RPC 的 gRPC deadline（秒）——issue #3332。
     *
     * <p>{@code batchGetProviderStatusByOrderIds} / {@code getSellerReturnConfig} 是純 DB 讀；
     * {@code retryReturnPrintLabel} 在 shipping 端只做一次 {@code shipmentMapper.selectOne} 後即
     * {@code CompletableFuture.runAsync} 觸發列印（<b>provider 呼叫不在同步區段內</b>）⇒ 三支都是
     * 本地 DB 等級，取內部呼叫慣例 10 秒。
     *
     * <p>{@code retryReturnPrintLabel} 的呼叫端 {@code AfterSaleLabelRetryService} 持有
     * {@code LABEL_RETRY_LEASE_MINUTES = 6} 分鐘（360 秒）租約，10 秒遠小於它，符合
     * 《多副本排程搶佔規範》§三第 5 步「租約長度 &gt; 單筆最壞耗時」。
     *
     * <p>⚠️ 必須<b>每次呼叫時</b>套用（{@code withDeadlineAfter} 換算的是絕對時間）；
     * 不得改共用的 {@code GrpcStubFactory}。
     */
    public static final long INTERNAL_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public ShippingGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    public CreateReturnShipmentResponse createReturnCvsShipment(String orderNo, String storeId,
                                                                 String merId, String buyerName,
                                                                 String buyerPhone, String temperature,
                                                                 String tradeAmt) {
        log.info("ShippingGrpcClient.createReturnCvsShipment.start", "建立超商退貨物流", "orderNo", orderNo);
        var stub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub)
                .withDeadlineAfter(RETURN_SHIPMENT_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.createReturnCvsShipment(
            CreateReturnCvsShipmentRequest.newBuilder()
                .setOrderNo(orderNo)
                .setStoreId(storeId)
                .setMerId(merId)
                .setBuyerName(buyerName)
                .setBuyerPhone(buyerPhone)
                .setTemperature(temperature)
                .setTradeAmt(tradeAmt)
                .build()
        );
        log.info("ShippingGrpcClient.createReturnCvsShipment.done", "建立超商退貨物流完成, code=" + resp.getCode(), "orderNo", orderNo);
        return resp;
    }

    public CreateReturnShipmentResponse createReturnHomeShipment(String orderNo, String storeId,
                                                                  String merId, String buyerName,
                                                                  String buyerPhone, String buyerCity,
                                                                  String buyerDistrict, String buyerAddress,
                                                                  String buyerPostalCode, String temperature) {
        log.info("ShippingGrpcClient.createReturnHomeShipment.start", "建立宅配退貨物流", "orderNo", orderNo);
        var stub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub)
                .withDeadlineAfter(RETURN_SHIPMENT_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.createReturnHomeShipment(
            CreateReturnHomeShipmentRequest.newBuilder()
                .setOrderNo(orderNo)
                .setStoreId(storeId)
                .setMerId(merId)
                .setBuyerName(buyerName)
                .setBuyerPhone(buyerPhone)
                .setBuyerCity(buyerCity)
                .setBuyerDistrict(buyerDistrict)
                .setBuyerAddress(buyerAddress)
                .setBuyerPostalCode(buyerPostalCode)
                .setTemperature(temperature)
                .build()
        );
        log.info("ShippingGrpcClient.createReturnHomeShipment.done", "建立宅配退貨物流完成, code=" + resp.getCode(), "orderNo", orderNo);
        return resp;
    }

    public RetryReturnPrintLabelResponse retryReturnPrintLabel(String shipTradeNo) {
        log.info("ShippingGrpcClient.retryReturnPrintLabel.start", "退貨列印重試", "shipTradeNo", shipTradeNo);
        var stub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.retryReturnPrintLabel(
            RetryReturnPrintLabelRequest.newBuilder()
                .setShipTradeNo(shipTradeNo)
                .build()
        );
        log.info("ShippingGrpcClient.retryReturnPrintLabel.done", "退貨列印重試完成, code=" + resp.getCode(), "shipTradeNo", shipTradeNo);
        return resp;
    }

    /** 批次查詢 shipments provider_status / provider_message（收斂規範 §6.3）。 */
    public BatchGetProviderStatusByOrderIdsResponse batchGetProviderStatusByOrderIds(
            java.util.List<String> orderIds, boolean isReturn) {
        var stub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        return stub.batchGetProviderStatusByOrderIds(
                BatchGetProviderStatusByOrderIdsRequest.newBuilder()
                        .addAllOrderIds(orderIds)
                        .setIsReturn(isReturn)
                        .build());
    }

    public GetSellerReturnConfigResponse getSellerReturnConfig(String storeId, String shippingType) {
        log.info("ShippingGrpcClient.getSellerReturnConfig.start", "查詢賣家退貨設定", "storeId", storeId);
        var stub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.getSellerReturnConfig(
            GetSellerReturnConfigRequest.newBuilder()
                .setStoreId(storeId)
                .setShippingType(shippingType)
                .build()
        );
        log.info("ShippingGrpcClient.getSellerReturnConfig.done", "查詢賣家退貨設定完成, code=" + resp.getCode(), "storeId", storeId);
        return resp;
    }
}
