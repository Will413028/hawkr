package io.github.will413028.hawkr.product.grpc.client;

import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
import io.github.will413028.hawkr.buyer.grpc.ListFavoritedProductIdsRequest;
import io.github.will413028.hawkr.buyer.grpc.ListFavoritedProductIdsResponse;
import io.github.will413028.hawkr.buyer.grpc.ListSubscribedSellerIdsRequest;
import io.github.will413028.hawkr.buyer.grpc.ListSubscribedSellerIdsResponse;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

/**
 * Buyer Service gRPC Client
 *
 * 用於 /public/products 的 viewer-aware filter（followed_product_only / followed_seller_only；02 §4.2.4 / §5.3.4）。
 * 失敗（grpc 例外或 envelope code != 0）會丟出 BuyerServiceUnavailableException，
 * 由 ProductService 翻成 FAILED_PRECONDITION 回 gateway，gateway 翻成 503。
 */
@Component
public class BuyerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(BuyerGrpcClient.class);
    private static final String BUYER_TARGET = "buyer";

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline 設定，而 Java {@code ManagedChannelBuilder}
     * 預設無 keepalive ⇒ 下游若是「掛住」而非「失敗」，blocking stub <b>永不返回</b>。
     * 本 client 走在 {@code /public/products} 的同步讀取路徑上（viewer-aware filter），
     * 沒有上界等於把 gateway 的 worker 一起吃掉。
     *
     * <p><b>10 秒怎麼來的</b>：這兩支都是 buyer-service 的<b>本地 DB 讀取</b>（無第三方、無再跨服務），
     * p99 毫秒級；10 秒是兩個數量級以上的餘裕，只用來把「永遠不回」轉成「10 秒後失敗」。
     * 取值與 {@code shipping-service} / {@code order-service} 同名常數一致。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}：後者是共用元件，動它會一次
     * 改掉全 repo 所有 Java gRPC 呼叫的逾時語意。
     * ⚠️ {@code withDeadlineAfter} 換算的是<b>絕對</b>時間，因此必須<b>每次呼叫時</b>套用；
     * 挪進建構子 ＝ 啟動 10 秒後全部逾時。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public BuyerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 取得指定買家的所有 favorited product IDs。
     *
     * @param buyerId 買家 ID（必須 > 0）
     * @return product_id 列表（無收藏回空 list）
     * @throws BuyerServiceUnavailableException buyer-service 不可用 / 回非 0 code 時
     */
    public List<Long> listFavoritedProductIds(long buyerId) {
        if (buyerId <= 0) {
            throw new IllegalArgumentException("buyerId must be > 0");
        }
        try {
            var stub = grpcStubFactory.blocking(BUYER_TARGET, BuyerServiceGrpc::newBlockingStub);
            var request = ListFavoritedProductIdsRequest.newBuilder()
                    .setBuyerId(buyerId)
                    .build();
            ListFavoritedProductIdsResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .listFavoritedProductIds(request);
            if (response.getCode() != 0) {
                log.warn("BuyerGrpcClient.listFavoritedProductIds.envelopeError",
                        "buyer 回 code=" + response.getCode() + ", msg=" + response.getMessage(),
                        "buyer_id", String.valueOf(buyerId));
                throw new BuyerServiceUnavailableException(
                        "buyer envelope code " + response.getCode() + ": " + response.getMessage());
            }
            return response.getProductIdsList();
        } catch (BuyerServiceUnavailableException e) {
            throw e;
        } catch (Exception e) {
            log.error("BuyerGrpcClient.listFavoritedProductIds.fail",
                    "Buyer Service 呼叫失敗: " + e.getClass().getName() + ": " + e.getMessage(),
                    e);
            throw new BuyerServiceUnavailableException(
                    "buyer service unavailable: " + e.getMessage(), e);
        }
    }

    /**
     * 取得指定買家所有追蹤的 seller IDs。
     *
     * @param buyerId 買家 ID（必須 > 0）
     * @return seller_id 列表（無追蹤回空 list）
     * @throws BuyerServiceUnavailableException buyer-service 不可用 / 回非 0 code 時
     */
    public List<Long> listSubscribedSellerIds(long buyerId) {
        if (buyerId <= 0) {
            throw new IllegalArgumentException("buyerId must be > 0");
        }
        try {
            var stub = grpcStubFactory.blocking(BUYER_TARGET, BuyerServiceGrpc::newBlockingStub);
            var request = ListSubscribedSellerIdsRequest.newBuilder()
                    .setBuyerId(buyerId)
                    .build();
            ListSubscribedSellerIdsResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .listSubscribedSellerIds(request);
            if (response.getCode() != 0) {
                log.warn("BuyerGrpcClient.listSubscribedSellerIds.envelopeError",
                        "buyer 回 code=" + response.getCode() + ", msg=" + response.getMessage(),
                        "buyer_id", String.valueOf(buyerId));
                throw new BuyerServiceUnavailableException(
                        "buyer envelope code " + response.getCode() + ": " + response.getMessage());
            }
            return response.getStoreIdsList().stream()
                    .map(Long::parseLong)
                    .collect(Collectors.toList());
        } catch (BuyerServiceUnavailableException e) {
            throw e;
        } catch (Exception e) {
            log.error("BuyerGrpcClient.listSubscribedSellerIds.fail",
                    "Buyer Service 呼叫失敗: " + e.getClass().getName() + ": " + e.getMessage(),
                    e);
            throw new BuyerServiceUnavailableException(
                    "buyer service unavailable: " + e.getMessage(), e);
        }
    }

    /**
     * Buyer Service 不可用例外（followed_* filter 場景無 fallback，
     * 必須讓上層感知並回 503 給前端，避免靜默回錯資料）。
     */
    public static class BuyerServiceUnavailableException extends RuntimeException {
        public BuyerServiceUnavailableException(String message) {
            super(message);
        }
        public BuyerServiceUnavailableException(String message, Throwable cause) {
            super(message, cause);
        }
    }
}
