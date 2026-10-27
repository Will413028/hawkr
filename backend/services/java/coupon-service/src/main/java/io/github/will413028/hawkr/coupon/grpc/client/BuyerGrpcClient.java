package io.github.will413028.hawkr.coupon.grpc.client;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsRequest;
import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsResponse;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Buyer Service gRPC Client（buyer_id rekey Phase 5：CouponExpiringWarningJob 通知路由用）
 *
 * 提供 buyer_id -> 登入 principal account_id 反查。硬依賴：Buyer Service 失敗時拋出例外，
 * 由呼叫端決定 fail-soft/fail-closed（鏡像 order-service/cart-service 既有的
 * BuyerGrpcClient.resolvePrincipals）。
 */
@Component
public class BuyerGrpcClient {

    /**
     * issue #3332：身分反解（{@code buyer_id → 登入 account_id}）的 gRPC deadline（秒）。
     *
     * <p>與 {@link SellerGrpcClient#PRINCIPAL_RESOLVE_DEADLINE_SEC} 是<b>同一個推導、同一個值</b>：
     * 自發自用防護（{@code CouponGrpcService}）一次就會依序打 seller 與 buyer 兩支，
     * 兩支合起來才是「一次身分反解」，逾時預算必須一起算。完整推導寫在 seller 側那個常數的
     * javadoc，此處不重複。<b>刻意直接引用而非各寫一份 {@code 5L}</b>：兩個數字若能各自被調整，
     * 那個「合起來算」的推導就會在下一次改動時靜默失效。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC =
            SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC;

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * {@code batchGetBuyerDisplayByBuyerIDs} 雖然 fail-soft（少了名字不阻斷查詢），
     * <b>永不返回連 fail-soft 都到不了</b>。
     *
     * <p><b>10 秒（比身分反解的 5 秒寬）</b>：這一支帶的是收券人清單的批次查詢，
     * 批量大於單筆 PK 反解；buyer-service 側仍是本地 DB 讀取，p99 毫秒級。
     * 取值與 {@code shipping-service} / {@code order-service} 同名常數一致。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public BuyerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 解析 buyer_id -> 登入 principal account_id（notification routing ＋ 自發自用防護用）。
     *
     * <p>回傳 {@link PrincipalResolution}：查無在架買家的 buyer_id 會被 server 原樣回吐，
     * 只拿 map 的呼叫端分不出「真的解析到」與「未驗證的 echo-back」（issue #2789）。
     * 通知類呼叫端直接讀 {@link PrincipalResolution#principals()}，行為與過去完全一致；
     * 閘門類（自發自用防護）必須改用 require 系列，未解析即 fail-closed。</p>
     *
     * @param buyerIds 待解析的 buyer_id 列表
     * @return principals（每個輸入 id 都有 entry）＋ unresolvedIds
     */
    public PrincipalResolution resolvePrincipals(List<Long> buyerIds) {
        if (buyerIds == null || buyerIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        var stub = grpcStubFactory.blocking("buyer", BuyerServiceGrpc::newBlockingStub);
        var request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(buyerIds)
                .build();
        ResolvePrincipalsResponse response = stub
                .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }

    /**
     * 批次取顯示用的買家名字（#2830）。
     *
     * <p>指定券／補償券的收券人清單要顯示「這張券發給了誰」。原本走
     * {@code AccountGrpcClient.batchGetAccountInfo} 取 nickname —— 但 accounts 表
     * <b>沒有 nickname 欄位</b>（{@code domain.Account} 也沒有，{@code toAccountInfo}
     * 從不填），所以那個欄位對<b>每一位</b>收券人都是空字串，不只已註銷者。
     * 買家的名字一律只有 buyer-service 有。
     *
     * <p>直接吃 buyer_id，不需要先解析 principal：本 RPC 是 stint-scoped PK 查詢，
     * 且已註銷的 stint 也解得出來（只回墓碑字串「已刪除帳號」，真名不外流）。
     *
     * <p>Fail-soft：失敗回空 Map，收券人清單只是少了名字，不阻斷模板詳情查詢。
     *
     * @param buyerIds 收券人的 buyer_id 列表
     * @return Map&lt;buyer_id, BuyerDisplay&gt;；查不到的 id 不出現
     */
    public Map<Long, BuyerDisplay> batchGetBuyerDisplayByBuyerIds(List<Long> buyerIds) {
        if (buyerIds == null || buyerIds.isEmpty()) {
            return Map.of();
        }
        var stub = grpcStubFactory.blocking("buyer", BuyerServiceGrpc::newBlockingStub);
        BatchGetBuyerDisplayByBuyerIDsResponse response = stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetBuyerDisplayByBuyerIDs(
                        BatchGetBuyerDisplayByBuyerIDsRequest.newBuilder().addAllBuyerIds(buyerIds).build());
        if (response.getCode() != 0) {
            throw Status.INTERNAL
                    .withDescription("buyer-service batchGetBuyerDisplayByBuyerIDs code=" + response.getCode())
                    .asRuntimeException();
        }
        return response.getBuyersMap();
    }
}
