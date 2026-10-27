package io.github.will413028.hawkr.cart.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.grpc.GetProductSimpleRequest;
import io.github.will413028.hawkr.product.grpc.GetProductSimpleResponse;
import io.github.will413028.hawkr.product.grpc.BatchGetSkuCartInfoRequest;
import io.github.will413028.hawkr.product.grpc.BatchGetSkuCartInfoResponse;
import io.github.will413028.hawkr.product.grpc.SkuCartInfo;
import io.github.will413028.hawkr.product.grpc.ProductServiceGrpc;
import org.springframework.stereotype.Component;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Product Service gRPC Client（設計書 §4.1）
 *
 * AddItem 時呼叫 GetProductSimple，GetCart 時呼叫 BatchGetSkuCartInfo
 */
@Component
public class ProductGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ProductGrpcClient.class);
    /**
     * backend#3332：product-service 內部讀取 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline，Java {@code ManagedChannelBuilder} 也沒有
     * 預設 keepalive ⇒ 下游掛住時 blocking stub <b>永不返回</b>。這兩支都在<b>使用者同步路徑</b>
     * 上（{@code AddItem} → {@link #getProductSimple}、{@code GetCart} →
     * {@link #batchGetSkuCartInfo}），沒有 deadline ＝ 使用者那一側無限等待；
     * {@code batchGetSkuCartInfo} 的降級（回空 Map）也永遠不會被觸發，因為它等的例外不會來。
     *
     * <p><b>10 秒怎麼來的</b>：product 端兩支都是<b>本地 DB 讀取</b>——
     * {@code getProductSimple} 走 {@code findSkuById}、{@code batchGetSkuCartInfo} 走
     * {@code findSkuCartInfoByIds} ＋ {@code shipping_method} JSONB 解析，
     * 無第三方、無跨服務扇出，p99 毫秒級。10 秒是兩個數量級以上的餘裕，
     * 只用來把「永遠不回」轉成「10 秒後失敗」。與同 package 其他服務的同名常數同值。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}（共用元件，動它會一次改掉全 repo
     * 所有 Java gRPC 呼叫的逾時語意）。⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，
     * 必須每次呼叫時套用；本類別每次呼叫都新建 stub，就地串上即可。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public ProductGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 取得商品簡化資訊（驗證 SKU + 取得快照）
     *
     * @param skuId product_spec.id（數字字串）
     * @return GetProductSimpleResponse，呼叫端需檢查 code 和 is_active
     */
    public GetProductSimpleResponse getProductSimple(String skuId) {
        var stub = grpcStubFactory.blocking("product", ProductServiceGrpc::newBlockingStub);
        var request = GetProductSimpleRequest.newBuilder()
                .setSkuId(skuId)
                .build();

        log.info("ProductGrpcClient.getProductSimple.call", "呼叫 Product Service", "skuId", skuId);
        return stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getProductSimple(request);
    }

    /**
     * 批次取得 SKU 即時資訊（庫存+配送溫層+價格+上架狀態）
     * GetCart 時呼叫，降級回傳空 Map
     *
     * @param skuIds SKU ID 列表（數字字串）
     * @return Map<skuId, SkuCartInfo>，失敗時回傳空 Map
     */
    public Map<String, SkuCartInfo> batchGetSkuCartInfo(List<String> skuIds) {
        if (skuIds == null || skuIds.isEmpty()) {
            return Collections.emptyMap();
        }

        try {
            var stub = grpcStubFactory.blocking("product", ProductServiceGrpc::newBlockingStub);
            var request = BatchGetSkuCartInfoRequest.newBuilder()
                    .addAllSkuIds(skuIds)
                    .build();

            log.info("ProductGrpcClient.batchGetSkuCartInfo.call",
                    "批次查詢 SKU 購物車資訊，數量=" + skuIds.size());

            BatchGetSkuCartInfoResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .batchGetSkuCartInfo(request);
            return response.getSkusMap();
        } catch (Exception e) {
            log.error("ProductGrpcClient.batchGetSkuCartInfo.fail",
                    "Product Service 批次查詢失敗，降級為空", e);
            return Collections.emptyMap();
        }
    }
}
