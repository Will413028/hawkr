package io.github.will413028.hawkr.coupon.grpc.client;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaRequest;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Seller Service gRPC Client
 *
 * 呼叫 BatchGetSellerMeta 取得商家名稱，失敗時降級為空 Map
 */
@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 的 {@code batchGetSellerMeta} 雖然失敗時降級為空 Map，<b>永不返回連降級都到不了</b>。
     *
     * <p><b>10 秒怎麼來的</b>：seller-service 的<b>本地 DB 讀取</b>，p99 毫秒級；
     * 取值與 {@code shipping-service} / {@code order-service} 同名常數一致。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    /**
     * issue #3332：身分反解（{@code store_id → 登入 account_id}）的 gRPC deadline（秒）。
     *
     * <p>比 {@link #INTERNAL_READ_DEADLINE_SEC} 短，理由與取值鏡像
     * {@code aftersales-service} 的 {@code SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC}：
     * 這是 stint-scoped 的<b>單表 PK 查詢</b>，且它的兩個呼叫情境都禁不起久等——
     * <ol>
     *   <li>自發自用防護（{@code CouponGrpcService}）：<b>fail-closed</b> 的同步閘門，
     *       卡住等於把領券請求整條掛住；</li>
     *   <li>{@code CouponExpiringWarningJob}：{@code @Scheduled} 排程，而 Spring Boot 的
     *       {@code spring.task.scheduling.pool.size} 預設是 <b>1</b>
     *       ⇒ 一次掛住就讓本服務的<b>所有</b>排程停擺且永不恢復。</li>
     * </ol>
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC = 5L;
    private final GrpcStubFactory grpcStubFactory;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 批次取得賣家資訊
     *
     * @param storeIds 賣家 store_id 列表
     * @return Map<storeId, SellerMeta>，失敗時回傳空 Map（降級）
     */
    public Map<Long, SellerMeta> batchGetSellerMeta(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return Collections.emptyMap();
        }

        try {
            var stub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
            var request = BatchGetSellerMetaRequest.newBuilder()
                    .addAllStoreIds(storeIds)
                    .build();

            log.info("SellerGrpcClient.batchGetSellerMeta.call",
                    "呼叫 Seller Service", "count", String.valueOf(storeIds.size()));

            BatchGetSellerMetaResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .batchGetSellerMeta(request);
            return response.getSellersMap();
        } catch (Exception e) {
            // 降級：Seller Service 失敗不影響優惠券查詢
            log.error("SellerGrpcClient.batchGetSellerMeta.fail", "Seller Service 呼叫失敗，降級為空", e);
            return Collections.emptyMap();
        }
    }

    /**
     * 解析 store_id → 登入 principal account_id（Plan 2b 自發自用防護用）。
     * 硬依賴（與本類別 batchGetSellerMeta 的降級策略不同）：自發自用防護是防詐骨科，
     * Seller Service 故障時必須 fail-closed，不能悄悄退化成錯誤比對。
     *
     * <p>回傳 {@link PrincipalResolution}：查無在架賣家的 store_id 會被 server 原樣回吐，
     * 只拿 map 的呼叫端分不出「真的解析到」與「未驗證的 echo-back」（issue #2789）。</p>
     */
    public PrincipalResolution resolvePrincipals(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        var stub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
        var request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(storeIds)
                .build();
        ResolvePrincipalsResponse response = stub
                .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            // 非 0 code（業務碼錯誤，非 transport 失敗）同樣視為失敗、fail-closed，
            // 避免呼叫端拿到空 map 後 getOrDefault 悄悄退化成錯誤比對（final review follow-up）
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }
}
