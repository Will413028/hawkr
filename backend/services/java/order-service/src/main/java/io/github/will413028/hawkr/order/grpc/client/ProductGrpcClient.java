package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutRequest;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutResponse;
import io.github.will413028.hawkr.product.grpc.DeductStockRequest;
import io.github.will413028.hawkr.product.grpc.GetProductResponse;
import io.github.will413028.hawkr.product.grpc.GetPublicProductRequest;
import io.github.will413028.hawkr.product.grpc.GetStockRequest;
import io.github.will413028.hawkr.product.grpc.GetStockResponse;
import io.github.will413028.hawkr.product.grpc.ProductServiceGrpc;
import io.github.will413028.hawkr.product.grpc.RestoreStockRequest;
import io.github.will413028.hawkr.product.grpc.StockItem;
import io.github.will413028.hawkr.product.grpc.StockOperationResponse;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

/**
 * Product Service gRPC Client
 *
 * 提供庫存操作：DeductStock / RestoreStock / GetStock
 */
@Component
public class ProductGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(ProductGrpcClient.class);

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
     * <p><b>10 秒怎麼來的</b>：這幾支都是下游服務的<b>本地 DB 讀寫</b>（無第三方、無跨服務串接），
     * p99 為毫秒級；10 秒是兩個數量級以上的餘裕，只用來把「永遠不回」轉成「10 秒後失敗」。
     *
     * <p><b>適用範圍＝本 client 全部五支 RPC</b>（issue #2294 R4 追加，2026-08-08）。
     * 第一版只加在 {@link #restoreStock}（排程路徑）上，其餘四支照舊無 deadline——同一支 client、
     * 逐字同形的缺陷只修一半。其中 {@link #deductStock} 在<b>結帳 hot path</b> 上，
     * 下游掛住即 gateway 執行緒無限期佔用，比排程掛住更早被使用者看見。
     *
     * <p><b>10 秒對 hot path 的兩支夠不夠</b>：{@code deductStock} 是 product 端一次
     * {@code UPDATE ... WHERE stock >= qty} 的原子扣減、{@code batchGetProductForCheckout}
     * 是一次批次 SELECT，皆為本地 DB、無第三方；即使遇到列鎖等待也是秒以下量級。
     * <b>失敗處理已接得住 {@code DEADLINE_EXCEEDED}</b>（它是 {@code StatusRuntimeException}）：
     * {@code OrderService.createOrder} 兩處扣庫存都以 {@code catch (Exception)} 收斂成
     * {@code stock_tx_records} FAILED ＋ {@code SERVICE_UNAVAILABLE} 業務例外，後續由
     * {@code StockCompensationWorker} 冪等歸還（{@code RestoreStock} 以 stockTxId 去重，
     * 未扣成功回 {@code NOOP_NOT_FOUND}）；{@code CheckoutService.resolveItems} 則由上層
     * RPC catch-all 收斂。⚠️ 這條路徑<b>本來就會</b>在下游拋錯時走到，deadline 只是讓
     * 「掛住」也走同一條路，不是新增分支。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}：後者是 observability-common
     * 的共用元件，動它會一次改掉全 repo 所有 Java gRPC 呼叫的逾時語意。
     * ⚠️ {@code withDeadlineAfter} 換算的是<b>絕對</b>時間，因此必須<b>每次呼叫時</b>套用；
     * 挪進建構子 ＝ 啟動 10 秒後全部逾時。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final ProductServiceGrpc.ProductServiceBlockingStub productStub;

    public ProductGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.productStub = grpcStubFactory.blocking("product", ProductServiceGrpc::newBlockingStub);
    }

    /**
     * 原子扣減庫存
     */
    public StockOperationResponse deductStock(String stockTxId, List<StockItem> items) {
        log.info("ProductGrpcClient.deductStock.start", "扣減庫存", "stockTxId", stockTxId);
        DeductStockRequest request = DeductStockRequest.newBuilder()
                .setStockTxId(stockTxId)
                .addAllItems(items)
                .build();
        StockOperationResponse response = productStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .deductStock(request);
        log.info("ProductGrpcClient.deductStock.done", "扣減庫存完成", "status", response.getStatus());
        return response;
    }

    /**
     * 原子歸還庫存（補償/取消）
     */
    public StockOperationResponse restoreStock(String stockTxId, List<StockItem> items) {
        log.info("ProductGrpcClient.restoreStock.start", "歸還庫存", "stockTxId", stockTxId);
        RestoreStockRequest request = RestoreStockRequest.newBuilder()
                .setStockTxId(stockTxId)
                .addAllItems(items)
                .build();
        StockOperationResponse response = productStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .restoreStock(request);
        log.info("ProductGrpcClient.restoreStock.done", "歸還庫存完成", "status", response.getStatus());
        return response;
    }

    /**
     * 查詢庫存
     */
    public GetStockResponse getStock(List<String> skuIds) {
        GetStockRequest request = GetStockRequest.newBuilder()
                .addAllSkuIds(skuIds)
                .build();
        return productStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getStock(request);
    }

    /**
     * 取得商品公開資訊（結帳用：含 SKU 的重量/材積/溫層/配送方式）
     *
     * @param productId 商品 ID（TSID 數字字串）
     * @return GetProductResponse（含 ProductDetail + SKU 列表）
     * @throws RuntimeException 商品不存在或服務不可用
     */
    public GetProductResponse getPublicProduct(String productId) {
        log.info("ProductGrpcClient.getPublicProduct.start", "取得商品資訊",
                "productId", productId);
        GetPublicProductRequest request = GetPublicProductRequest.newBuilder()
                .setProductId(productId)
                .build();
        // product-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        GetProductResponse response = productStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getPublicProduct(request);
        log.info("ProductGrpcClient.getPublicProduct.done", "取得商品資訊完成",
                "productId", productId);
        return response;
    }

    /**
     * 結帳鏈路批次商品查詢（02 設計書 §5.5.2）。
     * <p>
     * 取代 {@link #getPublicProduct} 在 Checkout flow 內的呼叫：
     * <ul>
     *   <li>接受 inactive product 不拋例外，回完整 weight / dimension / shipping_methods</li>
     *   <li>batch 一次取多 SKU，避免 N+1</li>
     *   <li>查無此 sku → response data map key 缺席，caller 透過 key 存在性判定 LOOKUP_FAILED</li>
     * </ul>
     */
    public BatchGetProductForCheckoutResponse batchGetProductForCheckout(java.util.List<String> skuIds) {
        log.info("ProductGrpcClient.batchGetProductForCheckout.start",
                "結帳鏈路批次商品查詢", "skuIdsCount", String.valueOf(skuIds.size()));
        BatchGetProductForCheckoutRequest request = BatchGetProductForCheckoutRequest.newBuilder()
                .addAllSkuIds(skuIds)
                .build();
        BatchGetProductForCheckoutResponse response = productStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetProductForCheckout(request);
        log.info("ProductGrpcClient.batchGetProductForCheckout.done",
                "結帳鏈路批次商品查詢完成", "returnedCount", String.valueOf(response.getDataCount()));
        return response;
    }
}
