package io.github.will413028.hawkr.cart.grpc.client;

import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

/**
 * Buyer Service gRPC Client（自買防護用，見 SellerGrpcClient.resolvePrincipals 鏡像）
 */
@Component
public class BuyerGrpcClient {

    private final GrpcStubFactory grpcStubFactory;

    public BuyerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 解析呼叫者身份（可能是 buyer_id）→ 登入 principal account_id（自買防護用）。
     * 硬依賴（與 batchGetBuyerMeta 類客戶端的降級策略不同）：自買防護是防詐骨科，
     * Buyer Service 故障時必須 fail-closed，不能悄悄退化成錯誤比對。
     *
     * <p>回傳 {@link PrincipalResolution}：查無在架買家的 buyer_id 同樣會被原樣回吐
     * （buyer.proto 的 {@code unresolved_ids} 逐字寫明那是 UNVERIFIED echo-back），
     * 自買防護的兩側都必須是真正解析出來的 account_id 才算數（issue #2789）。</p>
     */
    /**
     * backend#3332：{@code ResolvePrincipals} 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline，Java {@code ManagedChannelBuilder} 也沒有
     * 預設 keepalive ⇒ 下游掛住時 blocking stub <b>永不返回</b>。本呼叫是自買防護的
     * <b>硬依賴（fail-closed）</b>且位在<b>加入購物車／結帳的同步路徑</b>上，
     * 沒有 deadline ＝ 使用者那一側是無限等待。
     *
     * <p><b>為什麼是 5 秒</b>：{@code buyer-service} 端只做一次
     * {@code buyerRepo.GetPrincipalsByBuyerIDs} 的索引查詢（Go，無第三方、無交易、無扇出），p99 毫秒級。
     * 5 秒有三個數量級的餘裕，只用來把「永遠不回」轉成「5 秒後失敗」。
     * 取值與 aftersales 側同名 RPC 的 {@code PRINCIPAL_RESOLVE_DEADLINE_SEC} 一致。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}（共用元件，動它會一次改掉全 repo
     * 所有 Java gRPC 呼叫的逾時語意）。⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，
     * 必須每次呼叫時套用。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC = 5L;

    public PrincipalResolution resolvePrincipals(List<Long> ids) {
        if (ids == null || ids.isEmpty()) {
            return PrincipalResolution.empty();
        }
        var stub = grpcStubFactory.blocking("buyer", BuyerServiceGrpc::newBlockingStub);
        var request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(ids)
                .build();
        ResolvePrincipalsResponse response = stub
                .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }
}
