package io.github.will413028.hawkr.cart.grpc.client;

import io.github.will413028.hawkr.common.obs.config.GrpcTarget;
import io.github.will413028.hawkr.common.obs.config.ObsProperties;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcChannelRegistry;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.grpc.ObsGrpcClientInterceptor;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaRequest;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import io.opentelemetry.context.propagation.TextMapPropagator;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.BiConsumer;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;

/**
 * {@code batchGetSellerMeta} 的失敗處理（backend#2098）。
 *
 * <p>賣家 meta 裡的 store_code 是店面連結的 canonical key，缺了它前端 {@code sellerPath()}
 * 會渲染出指回目前頁面的死連結。修正前這裡有兩個「安靜地失敗」：
 * <ul>
 *   <li>非 0 業務碼被無視 —— {@code getSellersMap()} 對失敗回應就是空 Map，
 *       和「成功但這些賣家都不存在」完全無法區分；</li>
 *   <li>單次 transport 失敗即降級為空，連一次重試都沒有。</li>
 * </ul>
 * 兩者都用真的 gRPC server 驗（比照 order-service CartGrpcClientCheckoutLockAccountIdTest），
 * 因為要驗的正是「client 對回應/錯誤的判讀」，mock 掉 stub 就等於把待驗行為一起 mock 掉了。
 */
class SellerGrpcClientBatchMetaTest {

    private static final long STORE_ID = 2685914682L;
    private static final String STORE_CODE = "2612345678";

    private final AtomicInteger callCount = new AtomicInteger();

    private Server server;
    private GrpcChannelRegistry registry;

    @AfterEach
    void tearDown() {
        if (registry != null) {
            registry.shutdownAll();
        }
        if (server != null) {
            server.shutdownNow();
        }
    }

    /** 起一個假的 seller-service，行為由 handler 決定；handler 收到「這是第幾次呼叫」。 */
    private SellerGrpcClient clientBackedBy(
            BiConsumer<Integer, StreamObserver<BatchGetSellerMetaResponse>> handler) throws IOException {
        SellerServiceGrpc.SellerServiceImplBase fakeSeller = new SellerServiceGrpc.SellerServiceImplBase() {
            @Override
            public void batchGetSellerMeta(BatchGetSellerMetaRequest request,
                                           StreamObserver<BatchGetSellerMetaResponse> responseObserver) {
                handler.accept(callCount.incrementAndGet(), responseObserver);
            }
        };

        server = ServerBuilder.forPort(0).addService(fakeSeller).build().start();

        ObsProperties obsProperties = new ObsProperties();
        obsProperties.setService("cart-service-test");
        GrpcTarget sellerTarget = new GrpcTarget();
        sellerTarget.setHost("localhost");
        sellerTarget.setPort(server.getPort());
        sellerTarget.setPlaintext(true);
        obsProperties.getGrpc().getTargets().put("seller", sellerTarget);

        registry = new GrpcChannelRegistry(obsProperties,
                new ObsGrpcClientInterceptor(new ObsContext(), TextMapPropagator.noop()));
        return new SellerGrpcClient(new GrpcStubFactory(registry));
    }

    private static BatchGetSellerMetaResponse successResponse() {
        return BatchGetSellerMetaResponse.newBuilder()
                .setCode(0)
                .putSellers(STORE_ID, SellerMeta.newBuilder()
                        .setSellerName("陳記蔬果")
                        .setStoreId(String.valueOf(STORE_ID))
                        .setStoreCode(STORE_CODE)
                        .build())
                .build();
    }

    @Test
    @DisplayName("單次 transport 失敗會重試，第二次成功就拿得到 store_code（不再直接降級為空）")
    void transientFailure_isRetried_andStoreCodeSurvives() throws IOException {
        SellerGrpcClient client = clientBackedBy((attempt, observer) -> {
            if (attempt == 1) {
                observer.onError(Status.UNAVAILABLE.withDescription("seller pod rolling").asRuntimeException());
                return;
            }
            observer.onNext(successResponse());
            observer.onCompleted();
        });

        Map<Long, SellerMeta> result = client.batchGetSellerMeta(List.of(STORE_ID));

        assertThat(callCount.get()).as("第一次失敗後必須重試").isEqualTo(2);
        assertThat(result).containsOnlyKeys(STORE_ID);
        assertThat(result.get(STORE_ID).getStoreCode())
                .as("一次 transport 抖動不該讓整車賣家連結變成死連結")
                .isEqualTo(STORE_CODE);
    }

    @Test
    @DisplayName("非 0 業務碼視為失敗（會重試），不是「成功但沒有任何賣家」")
    void nonZeroBusinessCode_isTreatedAsFailure_notEmptySuccess() throws IOException {
        // gRPC status 是 OK，錯誤只放在 payload 的 code 欄位 —— 這正是舊實作看不見的形狀。
        SellerGrpcClient client = clientBackedBy((attempt, observer) -> {
            observer.onNext(BatchGetSellerMetaResponse.newBuilder()
                    .setCode(9233)
                    .setMessage("賣家不存在")
                    .build());
            observer.onCompleted();
        });

        Map<Long, SellerMeta> result = client.batchGetSellerMeta(List.of(STORE_ID));

        assertThat(callCount.get())
                .as("非 0 code 若被當成有效回應就只會呼叫一次；重試次數是這裡唯一能分辨新舊行為的訊號")
                .isEqualTo(2);
        assertThat(result).isEmpty();
    }

    @Test
    @DisplayName("成功回應只呼叫一次，不做多餘重試")
    void success_doesNotRetry() throws IOException {
        SellerGrpcClient client = clientBackedBy((attempt, observer) -> {
            observer.onNext(successResponse());
            observer.onCompleted();
        });

        Map<Long, SellerMeta> result = client.batchGetSellerMeta(List.of(STORE_ID));

        assertThat(callCount.get()).isEqualTo(1);
        assertThat(result.get(STORE_ID).getStoreCode()).isEqualTo(STORE_CODE);
    }

    @Test
    @DisplayName("重試後仍失敗才降級為空，且不得往上拋（GetCart 不能因為賣家 meta 掛掉而失敗）")
    void allAttemptsFail_degradesToEmptyWithoutThrowing() throws IOException {
        SellerGrpcClient client = clientBackedBy((attempt, observer) ->
                observer.onError(Status.UNAVAILABLE.asRuntimeException()));

        assertThatCode(() -> {
            Map<Long, SellerMeta> result = client.batchGetSellerMeta(List.of(STORE_ID));
            assertThat(result).isEmpty();
        }).doesNotThrowAnyException();

        assertThat(callCount.get()).isEqualTo(2);
    }
}
