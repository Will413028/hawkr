package io.github.will413028.hawkr.media.client;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

/**
 * Seller Service gRPC Client（cross-role guard 用途，比照 order-service 同名類別）。
 */
@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);

    /**
     * issue #3332：身分反解（{@code store_id → 登入 account_id}）的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 是 {@code BindStagedAsset} 越權防呆與 {@code Finalize} 擁有者比對的
     * <b>fail-closed 同步閘門</b>，永不返回等於把上傳請求整條掛住，而 fail-closed 的
     * 「拒絕」判斷連執行的機會都沒有。
     *
     * <p><b>5 秒怎麼來的</b>：這是 stint-scoped 的<b>單表 PK 查詢</b>，p99 毫秒級；
     * 取值鏡像 {@code aftersales-service} 的 {@code SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC}
     * （同一支 RPC、同一種工作量），比一般內部讀取的 10 秒短是刻意的——閘門要快失敗。
     *
     * <p>⚠️ 本類的 stub 是<b>建構子時建立的欄位</b>，而 {@code withDeadlineAfter} 換算的是
     * <b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；挪進建構子 ＝ 啟動 5 秒後全部
     * {@code DEADLINE_EXCEEDED}。也<b>不得</b>改共用的 {@code GrpcStubFactory}。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC = 5L;

    private final SellerServiceGrpc.SellerServiceBlockingStub sellerStub;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.sellerStub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
    }

    /**
     * 解析賣家 store_id → 登入 principal account_id。
     * 硬依賴，fail-closed：Seller Service 失敗時拋出例外，由呼叫端（BindStagedAsset 越權
     * 防呆、Finalize 擁有者比對）視為驗證失敗，不可悄悄放行。
     *
     * <p>回傳 {@link PrincipalResolution} 而非裸 map 是刻意的：查無在架賣家的 store_id
     * 會被 server 原樣回吐成合法非零值，只看 map 的呼叫端分不出它是不是真的解析結果，
     * 而擁有權比對的兩側同時回吐就會判成「同一個賣家」⇒ 越權放行（issue #2789）。</p>
     *
     * @param storeIds 待解析的 store_id 列表
     * @return principals（每個輸入 id 都有 entry）＋ unresolvedIds（其值是未驗證的 echo-back）
     */
    public PrincipalResolution resolvePrincipals(List<Long> storeIds) {
        if (storeIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        ResolvePrincipalsRequest request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(storeIds)
                .build();
        ResolvePrincipalsResponse response = sellerStub
                .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            log.warn("SellerGrpcClient.resolvePrincipals.businessError",
                    "resolvePrincipals 回傳非 0 code，fail-closed", "code", String.valueOf(response.getCode()));
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }
}
