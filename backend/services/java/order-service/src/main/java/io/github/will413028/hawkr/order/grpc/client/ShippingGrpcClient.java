package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.grpc.*;
import org.springframework.stereotype.Component;
import java.util.concurrent.TimeUnit;

/**
 * Shipping Service gRPC Client
 *
 * 提供：
 * - GetShipmentByOrderNo：物流單查詢（聚合訂單詳情用）
 * - GetShippingFeeConfigs：取得全部運費配置（供結帳本地快取）
 *
 * 內部呼叫，x-internal-service 自動注入
 */
@Component
public class ShippingGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ShippingGrpcClient.class);

    /**
     * Uber 即時報價的 gRPC deadline（秒）。
     *
     * <p>🔴 <b>不能沿用 {@link ProductGrpcClient#INTERNAL_READ_DEADLINE_SEC}（10 秒）</b>：
     * 那個值的理由寫在 {@link BuyerGrpcClient#INTERNAL_READ_DEADLINE_SEC} 的 javadoc，
     * 前提是「下游服務的<b>本地 DB 讀取</b>，無第三方」。本支不是——它在 shipping-service 內
     * 會打 Uber 的 Create Quote（E1）。
     *
     * <p><b>35 秒怎麼來的</b>：{@code UberApiClient} 的 {@code E1_QUOTE_DEADLINE} 是 8 秒，
     * 無副作用退避階梯 {@code READ_ONLY_BACKOFF_MS} 是 {@code {1s, 3s}} ⇒ 最多 3 次嘗試
     * ⇒ 8 + 1 + 8 + 3 + 8 = <b>28 秒</b>是 Uber 那一段的上界；加上 OAuth 取 token、
     * 取件點解析、分箱與報價落庫的本地 DB 往返，取 35 秒。
     *
     * <p>⚠️ 這個值<b>寧可偏大</b>：報價在 shipping-service 側是<b>先落庫再回傳</b>
     * （{@code UberQuoteWriter}），deadline 砍在回程上只會製造一筆沒人引用的 quote 列，
     * 而買家看到的是「Uber 算不出價」——一個本來會成功的呼叫被我方切斷。
     */
    public static final long UBER_QUOTE_DEADLINE_SEC = 35L;

    private final ShippingServiceGrpc.ShippingServiceBlockingStub shippingStub;

    public ShippingGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.shippingStub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub);
    }

    /**
     * 依訂單編號查詢物流單（聚合訂單詳情用）
     *
     * @param orderNo 訂單編號
     * @return GetShipmentResponse
     */
    public GetShipmentResponse getShipmentByOrderNo(String orderNo) {
        log.info("ShippingGrpcClient.getShipment.start", "查詢物流單", "orderNo", orderNo);
        GetShipmentByOrderNoRequest request = GetShipmentByOrderNoRequest.newBuilder()
                .setOrderNo(orderNo)
                .build();
        GetShipmentResponse response = shippingStub
                .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getShipmentByOrderNo(request);
        log.info("ShippingGrpcClient.getShipment.done",
                "查詢物流單完成 code=" + response.getCode(), "orderNo", orderNo);
        return response;
    }

    /**
     * 批次查詢物流單（賣家列表頁用，避免 N+1）
     *
     * @param orderNos 訂單編號清單（同一頁的訂單）
     * @return GetShipmentsByOrderNosResponse，data 中只包含查得到的 shipment
     */
    public GetShipmentsByOrderNosResponse getShipmentsByOrderNos(java.util.List<String> orderNos) {
        if (orderNos == null || orderNos.isEmpty()) {
            return GetShipmentsByOrderNosResponse.newBuilder().setCode(0).build();
        }
        log.info("ShippingGrpcClient.getShipmentsBatch.start",
                "批次查詢物流單", "count", String.valueOf(orderNos.size()));
        GetShipmentsByOrderNosRequest request = GetShipmentsByOrderNosRequest.newBuilder()
                .addAllOrderNos(orderNos)
                .build();
        GetShipmentsByOrderNosResponse response = shippingStub
                .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getShipmentsByOrderNos(request);
        log.info("ShippingGrpcClient.getShipmentsBatch.done",
                "批次查詢完成 code=" + response.getCode() + " size=" + response.getDataCount());
        return response;
    }

    /**
     * 查詢賣家物流開通狀態與寄件郵遞區號
     * 供結帳流程取得 sender_postal_code 推導運費路線，以及驗證賣家配送方式開通狀態
     *
     * @param storeId 賣家 store_id
     * @return GetSellerShippingConfigResponse
     */
    public GetSellerShippingConfigResponse getSellerShippingConfig(long storeId) {
        log.info("ShippingGrpcClient.getSellerShippingConfig.start",
                "查詢賣家物流設定", "storeId", String.valueOf(storeId));
        GetSellerShippingConfigRequest request = GetSellerShippingConfigRequest.newBuilder()
                .setStoreId(storeId)
                .build();
        GetSellerShippingConfigResponse response = shippingStub
                .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getSellerShippingConfig(request);
        log.info("ShippingGrpcClient.getSellerShippingConfig.done",
                "查詢賣家物流設定完成 code=" + response.getCode()
                        + " configs=" + response.getConfigsCount(),
                "storeId", String.valueOf(storeId));
        return response;
    }

    /**
     * 取得全部運費配置（configs + tiers）
     * 供 ShippingFeeGrpcFetcher 在 cache TTL 過期時 lazy refresh 使用（Resilience4j CB 保護）
     */
    public GetShippingFeeConfigsResponse getShippingFeeConfigs() {
        log.info("ShippingGrpcClient.getShippingFeeConfigs.start", "取得運費配置");
        GetShippingFeeConfigsRequest request = GetShippingFeeConfigsRequest.newBuilder().build();
        GetShippingFeeConfigsResponse response = shippingStub
                .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getShippingFeeConfigs(request);
        log.info("ShippingGrpcClient.getShippingFeeConfigs.done",
                "取得運費配置完成 code=" + response.getCode()
                        + " configs=" + response.getMethodConfigsCount()
                        + " tiers=" + response.getFeeTiersCount());
        return response;
    }

    /**
     * 取得（或沿用）一筆 Uber 即時報價（Stage 8B W2-2）。
     *
     * <p>🔴 <b>回應恆為 {@code code=0}</b>：業務不可用（賣家未開通 / 地址解不出 / 承載超限 /
     * Uber 報價失敗）由 {@code data.available=false} ＋ {@code unavailable_reason} 表達，
     * 不是錯誤碼。呼叫端把非零 code 當失敗會讓整個選項計算一起垮掉
     * （{@code ShippingGrpcServiceImpl.getUberQuoteForCheckout} 的 javadoc 逐字寫明這件事）。
     * 真正的例外（shipping-service 掛了、逾時）才會以 {@link io.grpc.StatusRuntimeException} 拋出。
     *
     * @param request 已組好的報價請求（dropoff ＋ 逐項 items；欄位多且含 repeated message，
     *                由呼叫端組裝比在這裡攤成 5 個參數清楚）
     */
    public GetUberQuoteForCheckoutResponse getUberQuoteForCheckout(GetUberQuoteForCheckoutRequest request) {
        log.info("ShippingGrpcClient.getUberQuote.start",
                "Uber 即時報價 itemCount=" + request.getItemsCount(),
                "storeId", String.valueOf(request.getSellerStoreId()));
        GetUberQuoteForCheckoutResponse response = shippingStub
                .withDeadlineAfter(UBER_QUOTE_DEADLINE_SEC, TimeUnit.SECONDS)
                .getUberQuoteForCheckout(request);
        log.info("ShippingGrpcClient.getUberQuote.done",
                "Uber 即時報價完成 code=" + response.getCode()
                        + " available=" + response.getData().getAvailable()
                        + " reason=" + response.getData().getUnavailableReason());
        return response;
    }

    /**
     * 重驗一筆 Uber 報價（{@code accept=false}）／capture 後把它標成已確認（{@code accept=true}）。
     *
     * <p>🔴 <b>回應同樣恆為 {@code code=0}</b>：不可用（過期／換手／金額變動／賣家未開通）由
     * {@code data.valid=false} ＋ {@code price_changed} ／ {@code unavailable_reason} 表達。
     * 與 {@link #getUberQuoteForCheckout} 同形，理由逐字相同。
     *
     * <p>⚠️ deadline 沿用 {@link #UBER_QUOTE_DEADLINE_SEC}：{@code UberQuoteService.revalidate()}
     * 內部<b>會呼叫 {@code quote()}</b>（hash 不同或剩餘 ≤180 秒就重報）⇒ 最壞情況包含一次完整的
     * Uber Create Quote 退避階梯，與報價那支同一個上界。給它一個較短的 deadline 會在
     * 「正好需要重報」的那些請求上切斷一個本來會成功的呼叫。
     *
     * @param request 已組好的重驗請求；{@code accept=true} 時 {@code order_no} 必填
     */
    public ValidateUberQuoteForCheckoutResponse validateUberQuoteForCheckout(
            ValidateUberQuoteForCheckoutRequest request) {
        log.info("ShippingGrpcClient.validateUberQuote.start",
                "Uber 報價重驗 accept=" + request.getAccept(),
                "quoteId", request.getQuoteId());
        ValidateUberQuoteForCheckoutResponse response = shippingStub
                .withDeadlineAfter(UBER_QUOTE_DEADLINE_SEC, TimeUnit.SECONDS)
                .validateUberQuoteForCheckout(request);
        log.info("ShippingGrpcClient.validateUberQuote.done",
                "Uber 報價重驗完成 code=" + response.getCode()
                        + " valid=" + response.getData().getValid()
                        + " priceChanged=" + response.getData().getPriceChanged()
                        + " reason=" + response.getData().getUnavailableReason());
        return response;
    }

    /** 批次查 shipments provider_status / provider_message（收斂規範 §6.3） */
    public BatchGetProviderStatusByOrderIdsResponse batchGetProviderStatusByOrderIds(
            java.util.List<String> orderIds, boolean isReturn) {
        return shippingStub
                .withDeadlineAfter(ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetProviderStatusByOrderIds(
                BatchGetProviderStatusByOrderIdsRequest.newBuilder()
                        .addAllOrderIds(orderIds)
                        .setIsReturn(isReturn)
                        .build());
    }
}
