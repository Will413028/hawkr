package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.cart.grpc.CartServiceGrpc;
import io.github.will413028.hawkr.cart.grpc.CheckoutLockRequest;
import io.github.will413028.hawkr.cart.grpc.CheckoutUnlockRequest;
import io.github.will413028.hawkr.cart.grpc.RemoveItemsByCheckoutIdRequest;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.grpc.Context;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.function.Supplier;
import java.util.concurrent.TimeUnit;

/**
 * Cart Service gRPC Client
 *
 * 提供結帳鎖定/解鎖/清理，供建單、取消、付款事件流程呼叫。
 * 解鎖與清理採降級策略：失敗不阻擋主流程，由 Cart Service 背景排程自動處理。
 */
@Component
public class CartGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(CartGrpcClient.class);

    /**
     * issue #2294 C3：內部 RPC 的 gRPC deadline（秒）。
     *
     * <p>理由與取值同 {@code ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC}：全域零 deadline ⇒
     * 下游掛住即 blocking stub 永不返回 ⇒ {@code CartCleanupRetryWorker}（C3）的租約下界
     * 無從建立，且 Spring 排程執行緒池預設 1，一次掛住全服務排程停擺。
     * cart 側是本地 DB 寫入（p99 毫秒級），10 秒是兩個數量級以上的餘裕。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須每次呼叫時套用
     * （本 client 的 {@link #stub()} 本來就是每次呼叫新建，天然符合）。
     */
    public static final long INTERNAL_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public CartGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 每次呼叫新建 stub 並就地套 deadline。
     *
     * <p>⚠️ <b>這個 method 必須留在所有呼叫點之前</b>（backend#3332）：
     * {@code scripts/check-grpc-client-deadline.py} 以「呼叫點之前最近的同名賦值」判定
     * deadline 覆蓋率，把它擺在檔案末端會讓上面三支呼叫被誤判成缺 deadline
     * （行為其實是對的，只是 gate 看不到）。
     *
     * <p>{@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須每次呼叫時套用；
     * 本 method 每次都新建 stub，天然符合。
     */
    private CartServiceGrpc.CartServiceBlockingStub stub() {
        return grpcStubFactory.blocking("cart", CartServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
    }

    /**
     * 結帳鎖定（建單前呼叫）
     *
     * @return code == 0 表示成功
     */
    public int checkoutLock(long buyerId, String checkoutId, List<String> skuIds) {
        var request = CheckoutLockRequest.newBuilder()
                .setCheckoutId(checkoutId)
                .addAllSkuIds(skuIds)
                .build();

        log.info("CartGrpcClient.checkoutLock.call", "結帳鎖定", "checkoutId", checkoutId);
        var response = callAsAccount(buyerId, () -> stub().checkoutLock(request));
        log.info("CartGrpcClient.checkoutLock.done", "結帳鎖定完成", "code", String.valueOf(response.getCode()));
        return response.getCode();
    }

    /**
     * 解除鎖定（建單失敗/取消/付款失敗時呼叫）
     * 降級：失敗不阻擋主流程，背景排程會自動解鎖過期鎖
     */
    public void checkoutUnlock(long buyerId, String checkoutId) {
        try {
            var request = CheckoutUnlockRequest.newBuilder()
                    .setCheckoutId(checkoutId)
                    .build();

            log.info("CartGrpcClient.checkoutUnlock.call", "解除鎖定", "checkoutId", checkoutId);
            callAsAccount(buyerId, () -> stub().checkoutUnlock(request));
        } catch (Exception e) {
            // 降級：解鎖失敗不阻擋主流程，背景排程 5 分鐘後自動解鎖
            log.error("CartGrpcClient.checkoutUnlock.fail", "解鎖失敗，將由背景排程自動處理", e);
        }
    }

    /**
     * 結帳成功清理（付款成功後呼叫）
     * 降級：清理失敗不影響訂單，鎖定過期後商品會自動恢復 NORMAL
     */
    public void removeItemsByCheckoutId(long buyerId, String checkoutId) {
        try {
            var request = RemoveItemsByCheckoutIdRequest.newBuilder()
                    .setCheckoutId(checkoutId)
                    .build();

            log.info("CartGrpcClient.removeItems.call", "清理購物車", "checkoutId", checkoutId);
            callAsAccount(buyerId, () -> stub().removeItemsByCheckoutId(request));
        } catch (Exception e) {
            // 降級：清理失敗不影響訂單
            log.error("CartGrpcClient.removeItems.fail", "購物車清理失敗", e);
        }
    }

    /**
     * 以指定 buyerId（呼叫端已解析的 buyer_id）覆寫當前 gRPC Context 執行呼叫。
     *
     * Cart Service 需要從 x-account-id metadata 讀取「已解析的 buyer_id」，而非
     * order-service 自己這次請求收到的原始/未解析 account_id（兩者在 buyer_id
     * 已 diverge 時不同）。
     *
     * 過去做法是額外用 MetadataUtils.newAttachHeadersInterceptor 顯式附加一個
     * x-account-id header——但 "cart" channel 早已透過 GrpcChannelRegistry
     * 全域掛載 GrpcAuthClientInterceptor，該攔截器會用「當下 ambient
     * GrpcAuthContext.ACCOUNT_ID」（也就是 order-service 收到的原始 account_id）
     * 再 put 一次同名 header，導致 metadata 出現兩個 x-account-id 值。
     * grpc-java 對重複 key 的 Metadata.get() 回傳最後一次 put 的值，而
     * GrpcAuthClientInterceptor 是後掛載的攔截器、其 start() 較晚執行，
     * 於是「後蓋前」，Cart Service 實際收到的是原始 account_id，不是已解析的
     * buyer_id——對 buyer_id 已 diverge 的帳號，cart_items 的 UPDATE ... WHERE
     * buyer_id = ? 因此永遠命中 0 筆，觸發 CHECKOUT_LOCK_FAILED (6005)。
     *
     * 修法對齊 Go 側 gateway/internal/grpc/cart_client.go 的 WithAuthContext
     * （PR #1646 修的同一類重複 header bug）：不再另外寫一份 Metadata，而是
     * 覆寫 ambient io.grpc.Context 的 GrpcAuthContext.ACCOUNT_ID，讓全域共用的
     * GrpcAuthClientInterceptor 自然讀到正確值、只寫出一份 header——context
     * 是唯一事實來源，不會有「兩個攔截器各寫一份」的競爭。
     */
    private <T> T callAsAccount(long buyerId, Supplier<T> call) {
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, buyerId);
        Context previous = ctx.attach();
        try {
            return call.get();
        } finally {
            ctx.detach(previous);
        }
    }
}
