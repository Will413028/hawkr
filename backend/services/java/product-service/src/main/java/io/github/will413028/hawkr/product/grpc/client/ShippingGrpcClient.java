package io.github.will413028.hawkr.product.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.shipping.grpc.*;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

/**
 * Shipping Service gRPC Client
 *
 * 提供 CalculateShippingOptions：計算可用配送選項與運費
 * 提供 ValidateProductShipping：商品配送方式合法性驗證
 * 內部呼叫，x-internal-service 自動注入
 */
@Component
public class ShippingGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ShippingGrpcClient.class);

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 走在商品上架驗證與 {@code /public/products} 讀路徑上（02 §2.7.2），
     * 且 {@code validateProductShipping} 是 {@code ProductPublishValidator} 的同步依賴。
     *
     * <p><b>10 秒怎麼來的</b>：實查這三支在 shipping-service 內都只做<b>本地 DB 讀取 ＋ 本地運算</b>
     * （{@code onboardingService.listSellerShippingMethods} / {@code shippingFeeService.*}），
     * <b>不</b>經 PayUni 物流 API、不再跨服務。取值與 {@code shipping-service} 自己的
     * {@code INTERNAL_READ_DEADLINE_SEC} 一致。
     *
     * <p>⚠️ 本類的 stub 是<b>建構子時建立的欄位</b>，而 {@code withDeadlineAfter} 換算的是
     * <b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；挪進建構子 ＝ 啟動 10 秒後全部
     * {@code DEADLINE_EXCEEDED}。也<b>不得</b>改共用的 {@code GrpcStubFactory}。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final ShippingServiceGrpc.ShippingServiceBlockingStub shippingStub;

    public ShippingGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.shippingStub = grpcStubFactory.blocking("shipping", ShippingServiceGrpc::newBlockingStub);
    }

    /**
     * 計算可用配送選項與運費
     * 給定包裹尺寸重量，回傳所有可用配送方式（按溫層分組）
     * 傳入 storeId 時只回傳賣家已開通的配送方式
     */
    public CalculateShippingOptionsResponse calculateShippingOptions(
            String lengthCm, String widthCm, String heightCm,
            String weightGrams, String deliveryZone, long storeId) {
        log.info("ShippingGrpcClient.calculateShippingOptions.start",
                "計算配送選項 zone=" + deliveryZone + " seller=" + storeId);

        CalculateShippingOptionsRequest.Builder builder = CalculateShippingOptionsRequest.newBuilder()
                .setLengthCm(lengthCm)
                .setWidthCm(widthCm)
                .setHeightCm(heightCm)
                .setWeightGrams(weightGrams)
                .setDeliveryZone(deliveryZone != null ? deliveryZone : "");

        if (storeId > 0) {
            builder.setStoreId(storeId);
        }

        CalculateShippingOptionsResponse response = shippingStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .calculateShippingOptions(builder.build());
        log.info("ShippingGrpcClient.calculateShippingOptions.done",
                "計算配送選項完成 code=" + response.getCode()
                        + " groups=" + response.getTemperatureGroupsCount());
        return response;
    }

    /**
     * 取得賣家當下已開通的配送方式（含 enabled flag）。
     * 02 §2.7.2：公開商品 read path 過濾 dangling shipping_methods 用。
     * fail-close：caller 應在 RPC 失敗或 code!=0 時主動處理（不 fail-open 把 dangling 回前端）。
     */
    public GetSellerShippingConfigResponse getSellerShippingConfig(long storeId) {
        log.info("ShippingGrpcClient.getSellerShippingConfig.start",
                "查詢賣家物流設定 seller=" + storeId);
        GetSellerShippingConfigRequest request = GetSellerShippingConfigRequest.newBuilder()
                .setStoreId(storeId)
                .build();
        GetSellerShippingConfigResponse response = shippingStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getSellerShippingConfig(request);
        log.info("ShippingGrpcClient.getSellerShippingConfig.done",
                "查詢賣家物流設定完成 code=" + response.getCode() + " configs=" + response.getConfigsCount());
        return response;
    }

    /**
     * 驗證商品配送方式合法性（賣家開通狀態 + 材積/重量限制）
     * <p>
     * response.code 必須為 {@link GrpcResponseCode#SUCCESS}；非 SUCCESS（如 INVALID_PARAM）
     * 視為「呼叫失敗」，拋 {@link RuntimeException} 由 caller 走 degraded 路徑。
     * 22 號設計書 §2.7：禁止僅以 allValid / resultsCount 推斷「無 violation」。
     */
    public ValidateProductShippingResponse validateProductShipping(
            long storeId, List<ValidateShippingItem> items) {
        log.info("ShippingGrpcClient.validateProductShipping.start",
                "驗證商品配送 seller=" + storeId + " items=" + items.size());

        ValidateProductShippingRequest request = ValidateProductShippingRequest.newBuilder()
                .setStoreId(storeId)
                .addAllItems(items)
                .build();

        ValidateProductShippingResponse response = shippingStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .validateProductShipping(request);
        log.info("ShippingGrpcClient.validateProductShipping.done",
                "驗證完成 code=" + response.getCode() + " allValid=" + response.getAllValid());
        if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new RuntimeException(
                    "shipping-service validateProductShipping 回應非 SUCCESS：code="
                            + response.getCode() + ", message=" + response.getMessage());
        }
        return response;
    }
}
